use facet::Facet;
use facet_generate_attrs as fg;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct GameEvent {
    pub id: String,
    pub event_type: String,
    pub payload_json: String,
    pub timestamp: i64,
    pub previous_hash: String,
    pub current_hash: String,
}

impl GameEvent {
    /// O Mini-Git: Calcula Hash(N) = SHA-256(Hash(N-1) + Payload + Timestamp)
    pub fn new(id: String, event_type: String, payload_json: String, timestamp: i64, previous_hash: String) -> Self {
        let mut hasher = Sha256::new();
        let payload_for_hash = format!("{}{}{}{}", previous_hash, id, event_type, payload_json);
        hasher.update(payload_for_hash.as_bytes());
        hasher.update(&timestamp.to_be_bytes());
        let current_hash = hasher.finalize().iter().map(|b| format!("{:02x}", b)).collect::<String>();

        Self {
            id,
            event_type,
            payload_json,
            timestamp,
            previous_hash,
            current_hash,
        }
    }

    /// Hash do gênesis: onde a cadeia de um usuário começa.
    pub const GENESIS: &'static str =
        "0000000000000000000000000000000000000000000000000000000000000000";

    /// Reencadeia a fila a partir de um topo novo, preservando o conteúdo.
    ///
    /// Quando o servidor responde `rebase_required` é porque o `previous_hash` do
    /// primeiro evento não bate com o que ele tem — outro aparelho escreveu antes, ou
    /// o app reabriu sem lembrar o topo. O conteúdo continua válido; só o encadeamento
    /// precisa ser refeito, e é exatamente isso que o Mini-Git promete.
    pub fn rebase(events: &[GameEvent], onto: &str) -> Vec<GameEvent> {
        let mut previous = onto.to_string();
        events
            .iter()
            .map(|e| {
                let rebased = GameEvent::new(
                    e.id.clone(),
                    e.event_type.clone(),
                    e.payload_json.clone(),
                    e.timestamp,
                    previous.clone(),
                );
                previous = rebased.current_hash.clone();
                rebased
            })
            .collect()
    }
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct SyncPayload {
    pub user_id: String,
    pub events: Vec<GameEvent>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mini_git_hash() {
        let event = GameEvent::new(
            "ch1".to_string(),
            "ANSWER".to_string(),
            "{}".to_string(),
            1690000000,
            "0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        );

        // O hash gerado deve ser consistente e não vazio
        assert!(!event.current_hash.is_empty());
        assert_eq!(event.current_hash.len(), 64); // SHA-256 len
    }

    /// O contrato do `GET /api/v1/nodes` como o Go realmente responde.
    ///
    /// Sem `status` no JSON a lista inteira falhava no serde e o `if let Ok(..)` engolia
    /// o erro: quem entrava autenticado via a árvore vazia e nenhuma mensagem.
    #[test]
    fn test_parses_nodes_without_status_from_the_backend() {
        let body = r#"[
            {"id":"10000000-0000-0000-0000-000000000001","name":"1. Fundamentos & Notação",
             "description":"Simulação de algoritmos.","row":0,"column":0,
             "required_xp":0,"prerequisites":[]},
            {"id":"20000000-0000-0000-0000-000000000002","name":"2. Estruturas Básicas",
             "description":"Estruturas de dados básicas.","row":1,"column":-1,
             "required_xp":100,"prerequisites":["10000000-0000-0000-0000-000000000001"]}
        ]"#;

        let nodes: Vec<SkillNode> =
            serde_json::from_slice(body.as_bytes()).expect("o payload do Go tem de desserializar");

        assert_eq!(nodes.len(), 2);
        assert_eq!(nodes[1].prerequisites, vec![nodes[0].id.clone()]);
        // `status` é derivado em view(); o default só precisa existir para o parse passar.
        assert_eq!(nodes[0].status, NodeStatus::Locked);
    }
}


#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengeValidation {
    #[serde(rename = "type")]
    pub validation_type: String, // "LINE_MATCH", "EXACT_MATCH" or "OUTPUT_MATCH" (DRY_RUN)
    /// Linha do bug **contada a partir de 1**, como a numeração que o jogador vê.
    ///
    /// É o número que quem escreve o desafio lê na tela; o motor trabalha em índice,
    /// e a conversão acontece uma vez só, ao montar a partida.
    pub correct_line: Option<i32>,
    pub expected_string: Option<String>,
    /// O que dizer a quem errou **este** desafio.
    ///
    /// Sem isto os três erros de uma sessão saíam com o mesmo texto genérico, porque
    /// o texto morava no motor. Quando falta, o motor cai no genérico.
    #[serde(default)]
    pub explanation: Option<String>,
}

/// Uma variável do painel de watch do DRY_RUN — nome e valor no estado inicial.
/// Não entrega a resposta: diz *o que* acompanhar durante o trace.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
pub struct WatchVariable {
    pub name: String,
    pub value: String,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengeContent {
    pub title: String,
    /// O enunciado, em texto, na língua em que o servidor mandou. Chegou a ser uma
    /// chave do catálogo (`TrapKey`), e aí nenhum desafio de verdade desserializava:
    /// o conteúdo vem do servidor, não do catálogo da interface.
    pub description: String,
    pub code_lines: Vec<String>,
    pub options: Option<Vec<String>>,
    pub correct_options: Option<Vec<String>>,
    /// Só em DRY_RUN: estado inicial mostrado no painel de watch.
    #[serde(default)]
    pub watch_variables: Option<Vec<WatchVariable>>,
    /// Só em DRY_RUN: em que ponto o watch foi capturado, ex. "antes da linha 4".
    #[serde(default)]
    pub watch_note: Option<String>,
    /// Segundos para esta questão, quando ela foge da régua do template.
    ///
    /// Existe para o desafio atípico — um trace longo demais, um enunciado curto demais
    /// — e não para ser preenchido em todo desafio: o custo é quase todo do template, e
    /// obrigar um número por desafio faz todo mundo copiar o do vizinho.
    #[serde(default)]
    pub seconds: Option<i32>,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengePayload {
    pub content: ChallengeContent,
    pub validation: ChallengeValidation,
}

/// Versão do formato da trilha empacotada. Sobe quando o formato muda de um jeito que
/// um app antigo não consegue ler — aí ele ignora a semente em vez de quebrar.
///
/// 2: a semente leva as três línguas, uma trilha por língua em `locales`.
/// 3: cada trilha ganha `origins`, os cartões de origem (ADR 0011).
pub const TRAIL_SEED_VERSION: i32 = 3;

/// A trilha que viaja dentro do app, gerada por `just seed-bundle`.
///
/// Existe para a primeira abertura sem rede: sem ela, instalação nova e offline não tem
/// conteúdo nenhum, nem para quem tem conta. É semente, não verdade — qualquer coisa
/// vinda do servidor ou do retrato guardado é mais nova e manda.
///
/// Não tem `Facet` de propósito: nunca atravessa a FFI, é lida e descartada aqui dentro.
#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct TrailSeed {
    pub version: i32,
    pub generated_at: String,
    /// A trilha em cada língua (`pt-BR`, `en`, `es`), como a API a serve nela. A
    /// estrutura é a mesma nas três; muda o texto.
    pub locales: std::collections::BTreeMap<String, SeedTrail>,
}

/// A trilha de uma língua dentro da semente.
#[derive(Serialize, Deserialize, Clone, Debug, Default)]
pub struct SeedTrail {
    pub nodes: Vec<SkillNode>,
    pub challenges: Vec<Challenge>,
    #[serde(default)]
    pub origins: Vec<OriginCard>,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct Challenge {
    pub id: String,
    pub node_id: String,
    pub template_type: String,
    pub payload: ChallengePayload, // Typed for Facet
    /// De onde o desafio veio, quando não foi escrito para o LogN; vazio é o caso
    /// comum. O `default` mantém compatível o JSON gravado antes de a coluna existir.
    /// O texto por trás deste id vem em `Model::origins`.
    #[serde(default)]
    pub origin: String,
}

/// O cartão de origem: quem escreveu o desafio, quando não foi escrito para o LogN. O
/// texto vem do servidor pelas tabelas de tradução (ADR 0011), como o resto da trilha —
/// não é mais cópia fixa do catálogo de interface.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default)]
#[facet(fg::namespace = "LogN")]
pub struct OriginCard {
    pub id: String,
    pub name: String,
    pub role: String,
    pub body: String,
}

/// O que o Core tem a dizer ao jogador, como **chave**, não como frase.
///
/// O Core escrevia a frase pronta em `status`, e ela vazava para a tela: a de login
/// abria com "Logged out successfully", em inglês, no vermelho de erro, num app em
/// português. A cópia vive em `i18n/locales/`; o cliente resolve a chave.
///
/// `Silent` é o estado normal — a maior parte do que acontece não precisa ser narrada.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq, Default)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum StatusKey {
    #[default]
    Silent,
    SigningIn,
    WrongCredentials,
    SignInFailed,
    NoConnection,
    ServerUnreadable,
    SessionExpired,
    SigningOut,
    ResumingSession,
    SendingCode,
    CodeSentFailed,
    CheckingCode,
    CodeInvalid,
    CreatingAccount,
    AccountFailed,
    ResettingPassword,
    ResetFailed,
    Syncing,
    SyncDiverged,
    SyncFailed,
    SyncOffline,
    SignInToSync,
    TreeUnavailable,
    /// O servidor respondeu 429. Os botões que batem nele ficam travados pela
    /// contagem do `Retry-After`, e a própria contagem aparece neles.
    RateLimited,
    /// Os quatro abaixo vêm do código de erro da API (`invalid_email`,
    /// `password_too_short`, `password_too_long`, `email_taken`). Antes todos viravam
    /// "não deu para criar a conta", sem dizer o que corrigir.
    InvalidEmail,
    PasswordTooShort,
    PasswordTooLong,
    EmailTaken,
    /// A compra foi feita na loja e o servidor está confirmando.
    PurchaseConfirming,
    PurchaseConfirmed,
    /// O servidor não confirmou agora (rede, erro dele). A loja entrega de novo na
    /// próxima abertura, e o app tenta outra vez sozinho.
    PurchaseFailed,
    /// `purchase_owned_by_other_account`: a compra é de outra conta ativa.
    PurchaseOwnedByOtherAccount,
    /// `purchase_account_mismatch`: a compra foi feita logado em outra conta.
    PurchaseAccountMismatch,
    /// `purchase_revoked`: a compra foi reembolsada ou revogada.
    PurchaseRevoked,
    /// Comprar pede conta; o visitante não compra.
    PurchaseNeedsAccount,
    /// A licença de uma trilha baixada foi revogada: chave e pacote saíram do aparelho.
    TrackRevoked,
    /// A compra está na conta, mas a licença ou o pacote não chegaram agora. Baixa
    /// sozinha na próxima abertura com rede.
    TrackDownloadFailed,
    /// O servidor recusou o token do provedor (`social_token_invalid`), ou o login
    /// nele não chegou ao fim.
    SocialSignInFailed,
    /// `social_email_unverified`: o provedor não confirmou o e-mail da conta.
    SocialEmailUnverified,
    /// `provider_disabled`: o servidor está com o login por esse provedor desligado.
    SocialProviderDisabled,
    /// `provider_reauth_required`: excluir a conta pede a confirmação pela Apple, não a
    /// senha (ADR 0017).
    ProviderReauthRequired,
    /// `purchase_pending`: o Google Play aceitou a compra, mas o pagamento ainda não caiu
    /// (boleto, dinheiro). A trilha abre quando cair.
    PurchasePending,
    /// `store_unavailable`: o servidor não fala com a loja agora. Tentar de novo depois.
    StoreUnavailable,
    /// `login_locked`: senha errada demais para este e-mail. Trava só o login por senha;
    /// o login social e a troca de senha pelo código continuam abertos, e o app não
    /// trava botão.
    LoginLocked,
    /// `otp_locked`: código errado demais para este e-mail. Nenhum código novo sai até a
    /// janela do servidor vencer (um dia), e esperar um minuto não resolve.
    CodeLocked,
}

/// Uma das verificações que a abertura roda antes de soltar o jogador no app.
///
/// A splash é o log de um juiz: cada verificação imprime o seu veredito numa linha, na
/// ordem, e a splash some quando a última fecha. A ordem é sessão, sync, termos: o sync
/// vem antes para nenhum XP depender do aceite (ADR 0020).
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum BootCheck {
    Session,
    Sync,
    Terms,
}

/// Como uma linha da abertura está. `Warn` não segura o jogador; `Fail` na sessão
/// manda para o login. `Skipped` é a verificação que não rodou e roda na próxima
/// abertura (termos sem rede).
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum BootVerdict {
    Running,
    Ok,
    Warn,
    Fail,
    Skipped,
}

/// O que a linha tem a dizer, como chave. A frase mora no catálogo.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum BootDetail {
    /// Sessão: falando com o servidor.
    Checking,
    /// Sessão: o servidor trocou o token.
    TokenRenewed,
    /// Sessão: sem rede, mas dentro do prazo guardado no aparelho.
    LocalTokenValid,
    /// Sessão: o servidor recusou, ou o prazo local venceu.
    SessionEnded,
    /// Sync: mandando a fila.
    Sending,
    NothingToSend,
    /// Sync: `count` eventos subiram.
    Sent,
    /// Sync: sem rede; `count` eventos continuam na fila.
    NoNetwork,
    /// Sync: o servidor recusou a fila, ou ela divergiu depois do rebase.
    Rejected,
    /// Sync: passou do tempo da abertura. O envio segue por trás.
    StillSending,
    /// Termos: perguntando ao servidor o que a conta tem para aceitar.
    TermsChecking,
    /// Termos: a conta aceitou a vigente.
    TermsCurrent,
    /// Termos: há versão relevante para aceitar; o app bloqueia.
    TermsChanged,
    /// Termos: só mudanças não relevantes; o app avisa uma vez.
    TermsNotice,
    /// Termos: sem rede ou sem resposta a tempo; confere na próxima abertura.
    TermsDeferred,
}

/// O sinal de uma mudança nos termos, como o diff do design: `+`, `~`, `−`.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum TermsChangeKind {
    Added,
    Changed,
    Removed,
}

/// Uma linha do "o que mudou" da tela de novo aceite. `summary` é conteúdo: vem do
/// servidor já na língua do app, e o cliente mostra como chegou.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
pub struct TermsChange {
    /// `terms` ou `privacy`.
    pub kind: String,
    /// A versão em que a mudança entrou.
    pub version: u32,
    pub change: TermsChangeKind,
    /// O id da seção no documento, para abrir o documento com ela destacada.
    pub section: String,
    pub summary: String,
}

/// A tela que cobre o app quando há versão relevante para aceitar (ADR 0020).
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
pub struct TermsUpdateViewModel {
    /// A última versão aceita pela conta, e a data de vigência dela (AAAA-MM-DD). Zero
    /// e vazia quando a conta nunca aceitou.
    pub from_version: u32,
    pub from_date: String,
    /// A vigente, e a data de vigência dela.
    pub to_version: u32,
    pub to_date: String,
    /// Quantas versões ficaram entre a aceita e a vigente, contando a vigente.
    pub versions_skipped: u32,
    /// Todas as mudanças das versões puladas, na ordem.
    pub changes: Vec<TermsChange>,
    /// As seções a destacar ao abrir cada documento.
    pub terms_sections: Vec<String>,
    pub privacy_sections: Vec<String>,
    /// O aceite foi enviado e espera o servidor.
    pub accepting: bool,
}

/// Uma linha do log da abertura.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
pub struct BootLine {
    pub check: BootCheck,
    pub verdict: BootVerdict,
    pub detail: BootDetail,
    /// Quantos eventos, para `Sent` e `NoNetwork`. Zero no resto.
    pub count: u32,
}

/// A splash, como a tela precisa dela.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default)]
#[facet(fg::namespace = "LogN")]
pub struct BootViewModel {
    /// A abertura ainda não terminou: a splash fica na frente de tudo.
    pub in_progress: bool,
    pub lines: Vec<BootLine>,
    /// De 0 a 100, para a barra.
    pub progress: u8,
    /// Sem rede, com a sessão dentro do prazo: a splash para e pergunta se tenta de
    /// novo ou entra com o que está no aparelho.
    pub awaiting_offline_choice: bool,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq, Default)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum NodeStatus {
    #[default]
    Locked,
    Active,
    Completed,
    PaywallLocked,
}

/// Estado de uma célula do telão. Os quatro estados do DS, e só eles.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum ScoreCellState {
    /// Aceito — `+` ou `+N` em cima, minuto do AC embaixo.
    Accepted,
    /// Tentado sem AC — `−N` em cima.
    Failed,
    /// Submetido após o congelamento — `?` em cima, `frz` embaixo.
    Frozen,
    /// Não tentado — célula vazia.
    Untried,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ScoreCell {
    pub state: ScoreCellState,
    pub top: String,
    pub bottom: String,
}

/// Uma linha do telão. 13 células, uma por letra A—M.
#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ScoreboardRow {
    pub rank: i32,
    pub team: String,
    pub university: String,
    pub solved: i32,
    pub penalty: i32,
    pub is_user: bool,
    pub cells: Vec<ScoreCell>,
}

/// Uma linha do ranking de celular.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default)]
#[facet(fg::namespace = "LogN")]
pub struct StandingRow {
    pub rank: i32,
    pub handle: String,
    pub university: String,
    pub solved: i32,
    pub penalty: i32,
    pub is_user: bool,
    /// Só na linha do usuário: `subiu 6 nesta rodada`. Vazio nas demais.
    pub note: String,
}

/// A licença de uma trilha paga, como `GET /api/v1/tracks/{id}/license` devolve.
///
/// Vive no Keychain do aparelho (chave `track_key:`), nunca no retrato: ali ela ficava
/// num `UserDefaults` que qualquer cópia de backup abre, e o prazo era editável.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct TrackLicense {
    pub track_id: String,
    /// Chave AES-256 de conteúdo da versão, em hexadecimal.
    pub key_hex: String,
    pub content_version: i32,
    /// Quando o servidor emitiu. Relógio do aparelho antes disto é relógio atrasado de
    /// propósito, e a trilha pede para conectar.
    pub issued_at: i64,
    pub valid_until: i64,
}

/// Uma trilha do catálogo, como `GET /api/v1/tracks` devolve: a principal e as pagas.
#[derive(Serialize, Deserialize, Clone, Debug, Default)]
pub struct Track {
    pub id: String,
    pub slug: String,
    /// `free` ou `paid`.
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub author: String,
    #[serde(default)]
    pub product_id: String,
    #[serde(default)]
    pub content_version: i32,
    #[serde(default)]
    pub color: String,
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub node_count: u32,
    #[serde(default)]
    pub problem_count: u32,
    #[serde(default)]
    pub languages: Vec<String>,
    #[serde(default)]
    pub owned: bool,
    /// Motivo, quando o direito da conta foi revogado. Vazio no resto.
    #[serde(default)]
    pub revoked_reason: String,
}

/// Onde a licença offline de uma trilha comprada está na escada de dias sem contato
/// (LogN Validade Offline). Com rede, cada abertura renova, e só `Silent` aparece.
#[derive(Facet, Serialize, Deserialize, Clone, Copy, Debug, Default, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum OfflineState {
    /// Sem licença no aparelho: não comprada, ou não baixada.
    #[default]
    NoLicense,
    /// Mais de 3 dias pela frente. Nada aparece além do "vale até" no detalhe.
    Silent,
    /// De 3 a 1 dia: selo no botão Trilhas, card âmbar, bloco no detalhe.
    Soon,
    /// Último dia.
    Today,
    /// Venceu, ou o relógio foi para antes da emissão. Só esta trilha fecha.
    Expired,
}

/// A trilha como a tela a vê: catálogo, detalhe, compra e o que está no aparelho.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
#[facet(fg::namespace = "LogN")]
pub struct TrackView {
    pub id: String,
    /// A trilha gratuita, a principal.
    pub is_free: bool,
    pub name: String,
    pub description: String,
    pub author: String,
    /// A cor do balão da trilha, `#RRGGBB`.
    pub color: String,
    /// O produto da App Store. É daqui que o app compra, nunca de um id montado.
    pub product_id: String,
    pub node_count: u32,
    pub problem_count: u32,
    /// Línguas publicadas, na forma do banco (`pt-BR`, `en`, `es`).
    pub languages: Vec<String>,
    /// É a trilha que a árvore mostra agora.
    pub selected: bool,
    pub owned: bool,
    /// A compra foi revogada; `revoked_reason` diz por quê.
    pub revoked: bool,
    pub revoked_reason: String,
    pub discontinued: bool,
    /// Nós conquistados nesta trilha.
    pub nodes_done: u32,
    /// Nós que só abrem com compra.
    pub closed_node_count: u32,
    /// Balões subidos na amostra, para a oferta do fim dela.
    pub sample_balloons: u32,
    /// Todos os problemas da amostra já renderam XP.
    pub sample_done: bool,
    /// XP ganho nesta trilha.
    pub track_xp: i32,
    /// O conteúdo fechado está no aparelho e abre.
    pub downloaded: bool,
    /// Tamanho do pacote no aparelho, em bytes. Zero quando não está baixado.
    pub download_bytes: u64,
    pub offline: OfflineState,
    /// Dias inteiros até a licença offline vencer. Zero sem licença ou vencida.
    pub offline_days_left: u32,
    /// Dias desde o último contato com o servidor, pela emissão da licença.
    pub days_since_contact: u32,
    /// Até quando a licença vale, em segundos desde a época. Zero sem licença.
    pub valid_until: i64,
    /// Os nós da trilha, na ordem da árvore, para o detalhe (1d).
    pub nodes: Vec<TrackNodeRow>,
}

/// Um nó na lista do detalhe da trilha.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
#[facet(fg::namespace = "LogN")]
pub struct TrackNodeRow {
    pub id: String,
    pub name: String,
    /// Abre sem compra: a amostra.
    pub free: bool,
    pub done: bool,
    /// O próximo a jogar.
    pub active: bool,
}

/// O selo do botão Trilhas: a trilha comprada que mais pede atenção.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
#[facet(fg::namespace = "LogN")]
pub struct CatalogBadge {
    /// `Soon`, `Today` ou `Expired`; qualquer outro é sem selo.
    pub state: OfflineState,
    pub days_left: u32,
}

/// Em que passo está a compra que o jogador acabou de fazer (LogN Trilhas, F3).
#[derive(Facet, Serialize, Deserialize, Clone, Copy, Debug, Default, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum PurchaseStage {
    /// Nenhuma compra na tela.
    #[default]
    Idle,
    /// A loja aprovou; o servidor está validando a transação.
    Validating,
    /// Transação válida; pedindo a licença.
    Licensing,
    /// Licença na mão; baixando o pacote.
    Downloading,
    /// Tudo no aparelho: a trilha abre sem rede.
    Ready,
    /// Parou; `failure` diz por quê.
    Failed,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
#[facet(fg::namespace = "LogN")]
pub struct PurchaseFlowView {
    pub stage: PurchaseStage,
    pub track_id: String,
    pub failure: StatusKey,
}

/// O resultado de "Restaurar compras".
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
#[facet(fg::namespace = "LogN")]
pub struct RestoreResultView {
    /// Há uma restauração na tela, em andamento ou acabada.
    pub active: bool,
    pub finished: bool,
    pub total: u32,
    pub restored: u32,
    /// Transações que são de outra conta ativa.
    pub other_account: u32,
    /// Nomes das trilhas que voltaram, na língua do catálogo.
    pub restored_names: Vec<String>,
}

/// A oferta do fim da amostra, no relatório da partida (LogN Trilhas, 1f).
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
#[facet(fg::namespace = "LogN")]
pub struct SampleOfferView {
    pub active: bool,
    pub track_id: String,
    pub next_node_name: String,
    /// Posição do próximo nó na trilha, a partir de 1.
    pub next_node_index: u32,
    pub remaining_nodes: u32,
    pub remaining_problems: u32,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct SkillNode {
    pub id: String,
    pub name: String,
    pub description: String,
    pub row: i32,
    pub column: i32,
    pub required_xp: i32,
    pub prerequisites: Vec<String>,
    #[serde(default)]
    pub track_id: String,
    /// Nó de trilha paga fora da amostra: só abre com licença. Quem decide é o
    /// servidor; o Core não adivinha pela linha nem por um id de trilha fixo.
    #[serde(default)]
    pub requires_purchase: bool,
    /// Assunto do nó, neutro (`adhoc`, `graphs`): decide cor e ícone no cliente, que
    /// antes adivinhava pelo nome — e o nome agora muda com a língua. Vazio quando o
    /// servidor ou o retrato antigo não mandam.
    #[serde(default)]
    pub topic: String,
    /// Derivado em `view()` a partir do XP e do DAG, nunca persistido: o Go não manda
    /// este campo, e sem o `default` a lista inteira falhava no `serde` — o usuário
    /// autenticado via uma árvore vazia sem erro nenhum aparecer.
    #[serde(default)]
    pub status: NodeStatus,
    /// Um por problema do nó, na ordem das letras: `true` quando ele já rendeu XP.
    /// Derivado em `view()`, como `status`. É o que a trilha conta (`3/5`) e o que o
    /// sheet desenha balão a balão.
    #[serde(default)]
    pub problems_solved: Vec<bool>,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
#[repr(C)]
pub enum TelemetryOperation {
    Identify { user_id: String },
    Track { event: String, properties: std::collections::HashMap<String, String> },
    /// Esquece a identidade: o que for enviado depois sai com um identificador anônimo
    /// novo. Na exclusão da conta, no logout e ao desligar a análise de uso.
    Reset,
    /// O interruptor "Análise de uso". Desligado, o shell para a captura automática do
    /// SDK (abertura e fechamento do app); erros e medições seguem.
    SetAnalyticsEnabled { enabled: bool },
}

impl crux_core::capability::Operation for TelemetryOperation {
    type Output = ();
}


#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(C)]
pub enum MonitoringOperation {
    LogError { message: String, details: String },
    StartSpan { name: String },
    EndSpan { name: String },
}

impl crux_core::capability::Operation for MonitoringOperation {
    type Output = ();
}

/// Severidade de um registro de log.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum LogLevel {
    Debug,
    Info,
    Warn,
    Error,
}

/// Um registro de log estruturado. O shell escolhe o provedor (hoje, os Logs do
/// PostHog). Fire-and-forget, como a telemetria.
///
/// Log é o rastro do que aconteceu; o que não deveria acontecer vai também como
/// `MonitoringOperation::LogError`, que vira issue. Mensagem e atributos nunca levam
/// token, senha, OTP, e-mail nem corpo de requisição (AGENTS.md, regra 9): rota,
/// status e código de erro bastam.
#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct LogOperation {
    pub level: LogLevel,
    pub message: String,
    pub attributes: std::collections::HashMap<String, String>,
}

impl crux_core::capability::Operation for LogOperation {
    type Output = ();
}

/// Pede ao shell que ofereça a avaliação na loja (App Store / Play Store).
/// Não tem retorno: a loja não diz se mostrou, e o Core não espera. Fire-and-forget.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum StoreReviewOperation {
    RequestReview,
}

impl crux_core::capability::Operation for StoreReviewOperation {
    type Output = ();
}
