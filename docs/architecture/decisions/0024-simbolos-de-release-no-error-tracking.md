# ADR 0024: símbolos de release no Error Tracking

## 1. Contexto

Os dois apps ligam o `autoCapture` do Error Tracking de PostHog (ADR 0001), mas nenhuma
release subia símbolo. O crash chegava, e a pilha não se lia:

- **Android, JVM.** O release sai com R8. Sem o mapeamento, o frame chega como `a.b.c`.
- **Android, Core.** O SDK 3.58 só embrulhava o `UncaughtExceptionHandler`: panic ou
  segfault em `libshared_core.so` não chegava a PostHog de jeito nenhum.
- **iOS.** O binário de loja sai sem símbolos, e o Core é estático dentro dele: o
  frame do Rust e o do Swift chegam como endereço sem o dSYM.

Para simbolizar um `.so`, `posthog-cli symbol-sets upload` exige debug info **e** build
ID GNU, e ignora sem erro a biblioteca que não tem os dois. O `.so` do Core não tinha
nenhum dos dois: o `boltffi` linka com o clang do NDK chamado direto, que não põe
`--build-id`, e o perfil `release` do Cargo não gerava debug info do nosso código (a
que havia era da std).

## 2. Decisão

**Core.** `[profile.release] debug = "line-tables-only"` no `shared_core/Cargo.toml`:
nome de função e linha, sem tipos nem variáveis. O `boltffi` sobe de 0.30.1 para
0.31.0, a primeira com `[targets.android.link] extra_args`, por onde entra
`-Wl,--build-id=sha1`. `[targets.android.debug_symbols] enabled = true` faz o `boltffi`
recusar o pack se o perfil perder a debug info.

**Android.** O SDK sobe para 3.71.4 e liga `errorTrackingConfig.captureNativeCrashes`.
Entra o plugin Gradle `com.posthog.android` 1.7.0, que injeta um mapping ID nos assets e
sobe o mapeamento do R8 e o `.so` sem strip de `merged_native_libs`, com
`applicationId`, `versionName` e `versionCode` como release. O AGP continua fazendo strip do que vai no
`.aab`, e o build ID sobrevive ao strip: é ele que casa o frame com o símbolo.

**Só a release sobe.** O plugin subiria em todo build com R8, inclusive no
`install-device`. As tasks `uploadPostHog*` ficam desligadas, a não ser com
`-Plogn.posthogUpload`, que só o `just android/release` passa. Sem `posthog-cli` ou sem
`POSTHOG_CLI_HOST`, `POSTHOG_CLI_PROJECT_ID` e `POSTHOG_CLI_API_KEY` no `.env`, a
release para: crash de versão sem símbolo fica ilegível para sempre.

**A chave é segredo de build.** É pessoal (`phx_`), com `error tracking write` e
`organization read`, e não é a `TELEMETRY_KEY` do app. Mora no `.env`; a receita a
passa só ao ambiente de um Gradle `--no-daemon --no-configuration-cache`, sem `export`,
e o plugin a entrega ao `posthog-cli` pelo ambiente. Sem daemon, porque ele guardaria o
ambiente da chamada. Sem configuration cache, porque ele grava em disco o ambiente das
tasks `Exec` e o reusaria depois de a chave ser trocada.

**A release confere o que subiu.** Antes do Gradle, o `.so` tem de ter build ID e
`.debug_line`. Depois, as duas tasks de upload têm de ter rodado (ou estar
`UP-TO-DATE`), nunca `SKIPPED`. As duas checagens existem porque o `posthog-cli` pula
sem erro o que não consegue simbolizar.
O Gradle do app só lê do `.env` a lista fechada de `appKeys`, e o `just sync-env` deixa
`POSTHOG_CLI_*` fora do `Local.xcconfig`, como já fazia com `*_SECRET`.

**O código vai junto.** `includeNativeSymbolSources` sobe os fontes que a debug info
cita, para o crash mostrar o trecho. O repositório é público e a trilha não mora nele
(regra 8), então o que sobe já é público.

## 3. Consequências

- Crash nativo do Core só aparece no **Android 12 ou mais**: o SDK lê o tombstone do
  sistema (`ApplicationExitInfo`) na abertura seguinte. Entre o `minSdk` 26 e o Android
  11, ele continua invisível.
- O `.so` em `android/generated/` passa de uns 8 MB para uns 30 MB por ABI. O app não
  cresce, porque o AGP faz strip, e o build de debug não muda.
- O `posthog-cli` vira ferramenta de build da release, como o `boltffi`.
- O iOS já ganha a linha do Rust no dSYM, mas o upload dele fica para quando houver
  conta Apple Developer (ADR 0022). O caminho é a Run Script do `upload-symbols.sh` do
  posthog-ios, que pede `ENABLE_USER_SCRIPT_SANDBOXING = NO`, ou o `posthog-cli` sobre o
  `dSYMs/` do `.xcarchive` no `release-ios`.
