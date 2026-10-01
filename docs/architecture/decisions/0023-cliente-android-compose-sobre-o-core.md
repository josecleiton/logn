# ADR 0023: cliente Android, Compose sobre o mesmo Core

## 1. Contexto

A ADR 0022 pôs o lançamento público no Google Play e deixou o cliente fora de escopo. Até
aqui só existia o esqueleto que destrava o Play Console (uma Activity vazia com a Play
Billing Library). A regra 9 do AGENTS.md pede ADR para toda dependência nova, e o cliente
traz várias de uma vez.

O iOS é a referência: o que ele faz hoje é o que o Android tem de fazer, tela por tela. O
Core Rust já é o dono de toda regra de jogo, sessão, fila offline e compra (ADR 0001), e
o shell de iOS é uma camada que despacha `Event`, executa efeitos e desenha o `ViewModel`.
O Android repete esse papel, e não reescreve regra nenhuma.

## 2. Decisão

**Kotlin e Jetpack Compose, sobre o mesmo Core, pela mesma ponte.** A ponte é `boltffi`
(JNI) com `bincode`, como no iOS; nada de UniFFI. `shared_core/boltffi.toml` ganhou o alvo
`android`, e `codegen` ganhou `--language kotlin` (tipos do facet typegen no pacote
`sh.logn.core`, com o namespace `LogN` em `sh.logn.core.LogN`).

**Dois módulos.**

- `:core-shell` é a biblioteca que embrulha o que é gerado: a classe `Core` (o
  `CoreWrapper.swift` de Android), as portas de efeito (`HttpPort`, `KeyValuePort`,
  `TelemetryPort`, `WallClock`) e o `TimerRegistry` de `crux_time`. Não conhece Android
  além de `Looper`.
- `:app` implementa as portas (OkHttp, armazenamento, PostHog), é dono do `Core` no
  `Application` e desenha as telas.

**O que é gerado não vai para o git.** Tipos do Core, ponte JNI, `.so` de cada ABI e as
strings do catálogo moram em `android/generated/`, ignorado. `just android/generate` refaz
tudo, e a tarefa `verifyGenerated` do Gradle recusa o build se `shared_core/src`,
`Cargo.lock`, `boltffi.toml` ou `i18n/` mudaram depois do último `generate`. O motivo é o
mesmo erro que o iOS já sofreu: tipo gerado fora de sincronia com o `.so` não falha na
compilação, falha no `bincode`, em tempo de execução. O teste instrumentado
`CoreBridgeTest` roda a ponte com o `.so` de verdade no aparelho e pega esse caso.

**Plataforma.**

| Item | Valor | Por quê |
|---|---|---|
| ABIs | `arm64-v8a`, `x86_64` | aparelho real e emulador; 32 bits fica de fora |
| NDK | 29.0.14206865 | alinha os `.so` em 16 KB, exigido pelo Play para Android 15+ (`just android/check-16kb`) |
| `minSdk` | 26 | o `min_sdk` do boltffi; cobre a base ativa sem ramo de compatibilidade |
| `compileSdk`/`targetSdk` | 37 | o que o Play exige para app novo |
| AGP | 9.3.1, com Kotlin embutido | sem o plugin `kotlin-android`; o plugin de Compose acompanha o Kotlin do AGP (2.2.10) |
| Gradle | 9.6.1 | o que o AGP 9.3 pede |

**Release com R8.** `minify` e `shrinkResources` ligados. A regra de consumidor de
`:core-shell` mantém `sh.logn.ffi.**` (os `native` do JNI) e `sh.logn.core.**` (o `bincode`
chama os construtores por nome). A tarefa `verifyReleaseConfig` recusa release com
`API_BASE_URL` sem `https://` ou com o `.so` de debug.

**Armazenamento (`Effect.SecureStore`).** O mesmo corte por chave do iOS:

| Chave | Onde | Equivalente iOS |
|---|---|---|
| `refresh_token`, `track_key:*` | `EncryptedSharedPreferences`, chave mestra no Android Keystore | Keychain |
| `track_package:*` | arquivo em `filesDir/TrackPackages`, nome = SHA-256 da chave | arquivo em Application Support |
| o resto | `SharedPreferences` comuns, valor em Base64 | `UserDefaults` |

Na primeira abertura depois de instalar, o arquivo cifrado é apagado (o `prepareKeychain`
de iOS). Se o Keystore falhar, o arquivo cifrado é recriado; se ainda falhar, a credencial
fica só na memória, a sessão cai ao reiniciar, e o app não quebra.

**Sem backup.** `allowBackup="false"` e `dataExtractionRules` excluindo tudo, na nuvem e
na transferência entre aparelhos: a sessão e as chaves das trilhas pagas são do Keystore
deste aparelho, e restaurar o resto sem elas deixa o app num estado que o Core não espera.

**Identificador do aparelho.** UUID aleatório por instalação, num arquivo em `filesDir`.
Nunca o `ANDROID_ID` nem o id de publicidade. É o `X-Device-ID` da licença da trilha paga,
como a política de privacidade descreve.

**Rede.** OkHttp, com a URL relativa do Core resolvida contra `API_BASE_URL` do
`BuildConfig` (lido do `.env`). Os cabeçalhos da resposta voltam inteiros, porque o Core lê
`Retry-After` e `Content-Language`. Sem texto claro em release (`network_security_config`);
o debug libera só `localhost` e `127.0.0.1`, e o backend local chega ao aparelho por
`adb reverse tcp:8080 tcp:8080`. Nada de pedido vai para o log.

**Telemetria.** Só PostHog, com o interruptor "Análise de uso" do iOS: desligado, o SDK
sobe sem a captura de abertura e fechamento, e os erros continuam saindo com identificador
anônimo. O erro do Core vira `$exception`, para cair no Error Tracking. O SDK de Android
não tem o log do PostHog que o de iOS usa; o `Effect.Log` vai para o logcat, só em debug.
Sem Sentry, sem Firebase e sem `google-services.json`.

**Navegação derivada do `ViewModel`, sem NavHost.** A raiz escolhe a tela na mesma ordem de
`LogNiOSApp.swift` (abertura, termos, app, aviso de exclusão, despedida, login). Voltar é
`BackHandler` por tela. É o que o iOS faz, e evita duas fontes de verdade sobre onde o
jogador está.

**Identidade visual.** Tema escuro e retrato, como no iOS. Os tokens de
`LognDesignSystem.swift` viram `LognDark`, `LognFont`, `Space`, `Radius` em `ui/theme/`, e
o `just android/tokens-gate` barra medida com `.dp`/`.sp` fora dali. IBM Plex Sans e Mono
em `res/font`. Balão, mapa e placar são desenhados em `Canvas` a partir dos paths do design
system, sem imagem e sem `material-icons-extended`.

**Entrada.** Links `logn://verify`, `logn://reset-password` e `logn://oauth/github`, um
`intent-filter` para cada um (os `<data>` de um filtro se combinam). Activity
`singleTask`, e o retorno chega por `onNewIntent`.

**Login social.** Sem Apple no Android.

- Google pelo Credential Manager, com o client web como `serverClientId`: o ID token sai
  com a audiência que o servidor aceita, e com o SHA-256 do nonce, como no iOS. O
  Credential Manager não entrega access token, então a exclusão da conta não revoga o
  consentimento no Google; fica com a pessoa, na conta Google dela.
- GitHub por Custom Tab, com PKCE S256 e `state`, voltando por `logn://oauth/github`. Link
  sem o `state` do pedido é ignorado sem desfazer o login em curso, porque qualquer app do
  aparelho pode mandá-lo. O pedido tem teto de dez minutos, e a aba fechada à mão vira
  desistência no `onResume`.
- Os dois correm num escopo do processo, e não da tela: a janela do provedor pode
  recriar a Activity.

**Documentos legais.** WebView sem JavaScript, sem acesso a arquivo, e a página do
servidor buscada por OkHttp em `shouldInterceptRequest`, que é o único jeito de ler os
cabeçalhos `X-LogN-Legal-*`; ela vai ao WebView com os cabeçalhos dela, CSP inclusive.
Resposta diferente de 200, ou falha de rede, cai na cópia dos assets.

**Compra no Google Play.** O Core ganhou, no `SubmitPurchase` e no `PurchaseSubmitted`,
o `provider` e o `purchase_token`. Com `google_play`, o corpo é `{provider, product_id,
purchase_token}`, que o servidor já aceita (ADR 0022); vazio é a App Store, como sempre.
`purchase_pending` (boleto, dinheiro) e `store_unavailable` ganharam `StatusKey` próprio e
não fecham a compra: o Play entrega de novo quando o pagamento cair.

O shell (`PlayStore`) compra com o id da conta no `obfuscatedAccountId`, manda ao Core o
que o Play aprovou, e não reconhece nada no aparelho: quem reconhece é o servidor, depois
de gravar a licença. Na abertura e a cada login, o que foi pago e não reconhecido volta ao
servidor, como o `Transaction.unfinished` do iOS, com duas travas que o iOS não precisa,
porque lá fechar a transação a tira da fila da loja:

- compra marcada com outra conta LogN não sai do aparelho;
- o que o Core já fechou para uma conta fica guardado como SHA-256 de conta e token, e
  não volta.

**Ainda por vir, e já decidido.** Play In-App Review para o `StoreReview`. Entra nesta
ADR, na tabela abaixo, quando entrar no código.

## 3. Dependências

| Dependência | Versão | Para quê | Alternativa descartada |
|---|---|---|---|
| Compose BOM (ui, foundation, material3) | 2026.06.01 | as telas | Views em XML: o iOS é declarativo, e a paridade tela a tela sai mais direta |
| activity-compose | 1.13.0 | `setContent`, edge-to-edge | — |
| lifecycle-runtime-compose | 2.11.0 | `collectAsStateWithLifecycle` do `ViewModel` | coletar à mão, sem parar em segundo plano |
| core-ktx | 1.19.0 | `SharedPreferences.edit {}` e afins | — |
| core-splashscreen | 1.0.1 | a splash de sistema com o balão, até o primeiro quadro | tema com `windowBackground`, que no Android 12+ vira ícone genérico |
| kotlinx-coroutines-android | 1.10.2 | portas assíncronas e timers | callbacks |
| okhttp | 5.4.0 | `Effect.Http` | `HttpURLConnection`, sem timeout por fase e sem `MockWebServer`; Retrofit, que não serve quando o Core já monta o pedido |
| security-crypto | 1.1.0 | `EncryptedSharedPreferences` | Keystore à mão com AES-GCM; a biblioteca foi descontinuada sem substituta em Jetpack, e sair dela é a próxima ADR que mexer em armazenamento |
| posthog-android | 3.58.3 | telemetria e erros | Firebase Crashlytics e Sentry: um segundo destino de dado pessoal para declarar |
| billing-ktx | 9.1.0 | compra de trilha (já no esqueleto, ADR 0022) | — |
| credentials, credentials-play-services-auth | 1.5.0 | login com Google pelo Credential Manager, o caminho que o Google mantém | Google Sign-In (`play-services-auth`), descontinuado; OAuth com PKCE numa Custom Tab como no iOS, que pediria um client Android com esquema próprio e a troca do código no aparelho |
| googleid | 1.1.1 | a opção "Continuar com o Google" e o ID token do Credential Manager | — |
| browser | 1.8.0 | Custom Tab do login com GitHub | WebView embutido, que o GitHub recusa para OAuth e que exporia a senha do GitHub ao app |
| junit, robolectric, mockwebserver3, androidx.test | 4.13.2, 4.15.1, 5.4.0, 1.7.0 | teste | — |

## 4. Descartados

- **Hilt e Koin.** A injeção é manual, no `Application`: um `Core`, três portas.
- **Navigation Compose.** A raiz é derivada do `ViewModel`.
- **Room e DataStore.** O Core já decide o que persiste; o shell só guarda bytes.
- **UniFFI.** A ponte é a mesma do iOS, e duas pontes seriam dois formatos para manter.
- **Tema claro e paisagem.** Nenhum dos dois existe no iOS.

## 5. Consequências

- Quem mexe no Core roda `just android/generate` antes de compilar o Android, e o Gradle
  lembra se esquecer.
- Efeito novo no Core quebra a compilação de `:core-shell` (o `when` sobre `Effect` é
  exaustivo), em vez de travar o Core esperando um resolve que nunca vem.
- Texto de tela continua saindo do catálogo (regra 6): o `tools_i18n` gera
  `res/values*/strings.xml` e os acessores `Str.<Grupo>.<chave>(context)`, e cada língua
  declara a própria tag (`i18n_locale`), que é o `SetLocale` do Core.
