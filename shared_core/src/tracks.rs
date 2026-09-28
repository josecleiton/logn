//! Trilha paga no aparelho: a licença, o pacote cifrado e onde cada um fica guardado.
//!
//! O formato do pacote é o do servidor (`BuildTrackPackage` em `backend/internal/domain/
//! tracks.go`): nonce de 12 bytes seguido do AES-256-GCM do JSON, com
//! `logn-track:{id}:{versão}` como dado associado. Mudar um é mudar o outro.

use std::collections::HashMap;

use aes_gcm::aead::{Aead, KeyInit, Payload};
use aes_gcm::{Aes256Gcm, Nonce};
use serde::Deserialize;

use crate::domain::{Challenge, TrackLicense};

/// Folga para o relógio do aparelho contra o do servidor na hora da emissão.
pub const LICENSE_CLOCK_SKEW_SECS: i64 = 300;
/// A partir de quando a tela avisa que a licença offline vai vencer (spec, seção 9).
pub const EXPIRING_SOON_SECS: i64 = 3 * 24 * 60 * 60;

const NONCE_LEN: usize = 12;
const TAG_LEN: usize = 16;

/// Chave da licença no armazenamento. O prefixo `track_key:` manda o shell para o
/// Keychain, só deste aparelho; o id da conta impede que quem entrar depois neste
/// aparelho abra a trilha de outra pessoa.
pub fn license_key(user_id: &str, track_id: &str) -> String {
    format!("track_key:{user_id}:{track_id}")
}

/// Chave do pacote. O prefixo `track_package:` manda o shell para um arquivo no
/// container do app, fora do `UserDefaults`.
pub fn package_key(user_id: &str, track_id: &str) -> String {
    format!("track_package:{user_id}:{track_id}")
}

/// A maior hora já vista pela conta neste aparelho. No Keychain, junto das licenças:
/// no `UserDefaults` bastaria apagá-la para voltar o relógio.
pub fn clock_key(user_id: &str) -> String {
    format!("track_key:{user_id}:clock")
}

/// Quais trilhas a conta tem neste aparelho. Não é segredo: é a lista do que procurar.
pub fn index_key(user_id: &str) -> String {
    format!("track_index:{user_id}")
}

/// A licença abre a trilha agora, sem rede. Relógio antes da emissão não vale: é o
/// jeito barato de esticar o prazo, atrasando o relógio do aparelho.
pub fn license_valid(license: &TrackLicense, now: i64) -> bool {
    now + LICENSE_CLOCK_SKEW_SECS >= license.issued_at && now < license.valid_until
}

/// O conteúdo fechado da trilha, por língua, como o servidor montou.
#[derive(Deserialize, Clone, Debug)]
pub struct PackageContent {
    pub track_id: String,
    pub content_version: i32,
    pub challenges: HashMap<String, Vec<Challenge>>,
}

#[derive(Debug, PartialEq, Eq)]
pub enum PackageError {
    BadKey,
    TooShort,
    /// Chave errada, pacote adulterado, ou de outra trilha ou versão.
    Decrypt,
    Format,
    WrongTrack,
}

/// Abre o pacote com a licença.
pub fn open_package(license: &TrackLicense, blob: &[u8]) -> Result<PackageContent, PackageError> {
    let key = decode_hex(&license.key_hex)
        .filter(|k| k.len() == 32)
        .ok_or(PackageError::BadKey)?;
    if blob.len() < NONCE_LEN + TAG_LEN {
        return Err(PackageError::TooShort);
    }
    let cipher = Aes256Gcm::new_from_slice(&key).map_err(|_| PackageError::BadKey)?;
    let aad = package_aad(&license.track_id, license.content_version);
    let plain = cipher
        .decrypt(
            Nonce::from_slice(&blob[..NONCE_LEN]),
            Payload { msg: &blob[NONCE_LEN..], aad: aad.as_bytes() },
        )
        .map_err(|_| PackageError::Decrypt)?;
    let content: PackageContent = serde_json::from_slice(&plain).map_err(|_| PackageError::Format)?;
    if content.track_id != license.track_id || content.content_version != license.content_version {
        return Err(PackageError::WrongTrack);
    }
    Ok(content)
}

fn package_aad(track_id: &str, version: i32) -> String {
    format!("logn-track:{track_id}:{version}")
}

fn decode_hex(s: &str) -> Option<Vec<u8>> {
    if s.len() % 2 != 0 {
        return None;
    }
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(s.get(i..i + 2)?, 16).ok())
        .collect()
}

#[cfg(test)]
pub(crate) mod testing {
    use super::*;

    /// Cifra como o servidor cifra, para o teste montar pacote. Nonce fixo: só teste.
    pub fn seal_package(license: &TrackLicense, json: &str) -> Vec<u8> {
        let key = decode_hex(&license.key_hex).unwrap();
        let cipher = Aes256Gcm::new_from_slice(&key).unwrap();
        let nonce = [7u8; NONCE_LEN];
        let aad = package_aad(&license.track_id, license.content_version);
        let mut out = nonce.to_vec();
        out.extend(
            cipher
                .encrypt(Nonce::from_slice(&nonce), Payload { msg: json.as_bytes(), aad: aad.as_bytes() })
                .unwrap(),
        );
        out
    }
}

#[cfg(test)]
mod tests {
    use super::testing::seal_package;
    use super::*;

    fn license(version: i32) -> TrackLicense {
        TrackLicense {
            track_id: "t1".into(),
            key_hex: "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff".into(),
            content_version: version,
            issued_at: 1_000,
            valid_until: 2_000,
        }
    }

    const CONTENT: &str = r#"{"track_id":"t1","content_version":1,"challenges":{"pt-BR":[]}}"#;

    #[test]
    fn test_the_package_opens_with_its_license() {
        let content = open_package(&license(1), &seal_package(&license(1), CONTENT)).unwrap();
        assert_eq!(content.track_id, "t1");
        assert!(content.challenges.contains_key("pt-BR"));
    }

    #[test]
    fn test_the_package_of_another_version_does_not_open() {
        let blob = seal_package(&license(1), CONTENT);
        assert_eq!(open_package(&license(2), &blob).unwrap_err(), PackageError::Decrypt);
    }

    #[test]
    fn test_a_tampered_package_does_not_open() {
        let mut blob = seal_package(&license(1), CONTENT);
        let last = blob.len() - 1;
        blob[last] ^= 1;
        assert_eq!(open_package(&license(1), &blob).unwrap_err(), PackageError::Decrypt);
        assert_eq!(open_package(&license(1), &blob[..10]).unwrap_err(), PackageError::TooShort);
        let mut bad = license(1);
        bad.key_hex = "zz".into();
        assert_eq!(open_package(&bad, &blob).unwrap_err(), PackageError::BadKey);
    }

    /// Pacote cifrado em Go com `crypto/cipher`, como o `seal` de `tracks.go`, com a
    /// mesma chave. Prova que os dois lados concordam no formato, não só consigo mesmos.
    #[test]
    fn test_a_package_sealed_by_the_server_opens() {
        let blob = decode_hex(GO_SEALED_PACKAGE).unwrap();
        let content = open_package(&license(1), &blob).unwrap();
        assert_eq!(content.content_version, 1);
    }

    #[test]
    fn test_the_license_is_valid_only_inside_its_window() {
        let l = license(1);
        assert!(license_valid(&l, 1_000));
        assert!(license_valid(&l, 1_999));
        assert!(!license_valid(&l, 2_000), "venceu");
        assert!(license_valid(&l, 1_000 - LICENSE_CLOCK_SKEW_SECS), "folga do relógio");
        assert!(!license_valid(&l, 1_000 - LICENSE_CLOCK_SKEW_SECS - 1), "relógio atrasado de propósito");
    }

    const GO_SEALED_PACKAGE: &str = "0c7e8e708ac2bf57d3fdab7fa72a8b6e2bd56dca1d727d884133ad2b1ba23f304ed7fda8cbab1075b087f411daf24b69fea8b1d794c602e4d52539a1c3a7e3cc1d9e11ce6a584c072efd27ad833aa17b1a990701ee32fc8d3ed34b3ca3725f15ba5c9789e6794df69dd781";
}
