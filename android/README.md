# Cliente Android

Por enquanto é um esqueleto: uma tela com o nome do app, sem dependência nenhuma. Existe
para o Play Console ter um AAB do pacote `sh.logn.app` (ADR 0022). Sem ele, a Google Play
Developer API responde `applicationNotFound`, e os produtos das trilhas não podem ser
criados, porque o Console só os aceita depois de receber um AAB com a permissão
`com.android.vending.BILLING`. O cliente de verdade (Kotlin, Compose, o Core pela FFI)
entra por cima disto.

## Build

Gradle 9.6.1 pelo wrapper, AGP 9.3.1, JDK 17 e SDK 36.

```sh
cd android
./gradlew :app:bundleRelease   # → app/build/outputs/bundle/release/app-release.aab
```

## Assinatura

O release é assinado com a **upload key**, e o Play App Signing reassina com a chave do
Google o que chega a quem instala. A upload key mora fora do repositório, em
`~/.android/keystores/logn-upload.jks`, com cópia fora da máquina: sem ela, só o suporte
do Google troca a chave, e leva dias.

O build lê o caminho e as senhas de `android/keystore.properties`, que o `.gitignore`
barra. Copie `keystore.properties.example` e preencha. Sem o arquivo, o release sai sem
assinatura e o Play Console recusa o upload; não existe chave de exemplo no lugar.

`java.util.Properties` trata `\` como escape e corta o valor em `:` ou `=` sem escape.
Senha com esses caracteres tem de ir escapada no arquivo.

## Chaves e login com Google

Cada chave que assina o app tem o seu client Android no Google Auth Platform (pacote +
SHA-1), e todos entram em `google_android_client_ids` no Terraform:

| Chave | Onde |
|---|---|
| debug | `~/.android/debug.keystore` |
| upload | `~/.android/keystores/logn-upload.jks` |
| Play App Signing | Play Console → Integridade do app, depois do primeiro upload |

O app pede o ID token com o client web como `serverClientId` (`google_web_client_id`).
