# Cliente Android

Kotlin e Compose sobre o mesmo Core Rust do iOS, pela mesma ponte (`boltffi` + `bincode`).
As decisões e as dependências estão na ADR 0023; o iOS é a referência de cada tela.

- `core-shell/`: a ponte com o Core (`Core.kt`), as portas de efeito e os timers.
- `app/`: as portas de verdade (OkHttp, armazenamento, PostHog), o `Application` dono do
  Core e as telas em `ui/`.
- `generated/`: tipos do Core, ponte JNI, `.so` e strings do catálogo. Fora do git.

A permissão `com.android.vending.BILLING` vem da Play Billing Library. Escrita à mão no
manifesto, sem a biblioteca, ela conta como a API AIDL antiga, e o Console recusa (o piso
é a 8.0).

`versionCode` sobe a cada upload, inclusive depois de um upload recusado.

## Build

Gradle 9.6.1 pelo wrapper, AGP 9.3.1, JDK 17, SDK 37 e NDK 29.0.14206865. Da raiz:

```sh
just android/generate          # tipos, strings e .so (debug); `generate release` para loja
just android/seed              # semente da trilha e documentos legais, com o backend de pé
just android/install           # adb reverse, instala no aparelho e abre
just android/test              # testes na JVM
just android/itest             # a ponte com o .so de verdade, no aparelho
just android/lint              # ktlint e Android lint
just android/tokens-gate       # medida solta fora de ui/theme
```

O Gradle recusa o build se o Core ou o catálogo mudaram depois do último `generate`.

O debug fala com `http://localhost:8080`, e o backend local chega ao aparelho por
`adb reverse tcp:8080 tcp:8080` (`just android/reverse`). O resto vem do `.env` da raiz:
`TELEMETRY_KEY`, `POSTHOG_HOST`, `GOOGLE_WEB_CLIENT_ID`, `GITHUB_CLIENT_ID` e, no release,
`API_BASE_URL`. Sem chave de telemetria, o PostHog não sobe.

Release, para o Play Console:

```sh
just android/generate release
cd android && ./gradlew :app:bundleRelease   # → app/build/outputs/bundle/release/app-release.aab
```

O roteiro de teste no aparelho, tela a tela, está em `docs/testing/roteiro-android.md`.

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
