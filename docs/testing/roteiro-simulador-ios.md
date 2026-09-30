# Roteiro de teste no simulador iOS — tela a tela

Como percorrer o LogN inteiro no simulador, com backend de verdade, sem depender de
alguém tocando na tela. Escrito depois de a sessão de auditoria de design encontrar
cinco bugs que **só apareceram percorrendo o app**: nenhum deles quebrava o build,
nenhum aparecia em teste unitário.

Tudo aqui é comando de terminal. O agente lê o resultado por screenshot.

---

## 0 · Antes de começar

### Acessibilidade (uma vez por máquina)

O toque sintético usa `CGEvent`, que exige permissão de Acessibilidade para o
terminal (ou para o app que roda o agente). Sem isso os toques são engolidos em
silêncio — não dá erro, só não acontece nada.

```bash
cat > /tmp/AskA11y.swift <<'EOF'
import ApplicationServices
let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
print(AXIsProcessTrustedWithOptions(options) ? "JA_AUTORIZADO" : "PEDIDO_ENVIADO — confirme o diálogo")
EOF
swift /tmp/AskA11y.swift
```

### Infra

```bash
cd <raiz do repo>
docker compose up -d db mailpit        # Postgres 5432, Mailpit 8025
set -a; source .env; set +a
(cd backend && go run .) &             # :8080
curl -s localhost:8080/ping
```

O servidor aplica as migrações de `backend/schema/migrations/` no boot e diz quais
rodaram. A `0000` é o schema de partida, e volume novo sai dela; qualquer alteração
posterior vai num arquivo `NNNN_descricao.sql` novo, nunca editando um já aplicado.

> **O `go test` do backend derruba o seed.** `setupTestDB` faz
> `TRUNCATE game_events, user_sync_state, challenges CASCADE`. Depois de rodar os
> testes, os desafios somem e a partida abre vazia. Para repor:
>
> ```bash
> docker exec logn-db-1 psql -U logn_user -d logn_db \
>   -c "DELETE FROM schema_migrations WHERE name >= '0021'"
> # e reinicie o backend: as migrações de conteúdo rodam de novo, em ordem, e repõem a trilha
> ```

`/api/v1/sync` e `/api/v1/progress` exigem `Authorization: Bearer`. Sem token são 401
— o corpo do pedido não decide mais de quem é a cadeia.

### Conta de teste

O cadastro pela UI passa por OTP (o código chega no Mailpit, `localhost:8025`).
Para ir direto ao ponto, crie pela API:

```bash
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"jogador@example.com","password":"logn12345","otp":"<código do Mailpit>"}'
```

### Build e instalação

```bash
SP=/tmp/logn-shots && mkdir -p "$SP"
DEV=$(xcrun simctl list devices booted | grep -o '[0-9A-F-]\{36\}' | head -1)

just build-ios-ffi                      # codegen + xcframework; obrigatório se tocou em Rust
xcodebuild -project ios/LogNiOS/LogNiOS.xcodeproj -scheme LogNiOS \
  -configuration Debug -destination "platform=iOS Simulator,id=$DEV" \
  -derivedDataPath "$SP/dd" build

xcrun simctl terminate booted sh.logn.LogNiOS 2>/dev/null
xcrun simctl install booted "$SP/dd/Build/Products/Debug-iphonesimulator/LogNiOS.app"
xcrun simctl launch booted sh.logn.LogNiOS
```

O bundle id é `sh.logn.LogNiOS` — não `com.josecleiton.*`. É o id de teste, de propósito:
o de loja é `sh.logn.app`, o mesmo pacote do Android.

---

## 1 · Geometria da janela: o passo que ninguém acerta chutando

O toque é em coordenada de tela do macOS. Para converter da imagem do screenshot
para a tela é preciso saber **onde a área do device começa e que tamanho tem**.

Não é a janela: a janela do Simulator inclui moldura e uma toolbar flutuante. Chutar
"a janela inteira" erra ~25px na vertical, o que basta para um botão de 40dp no topo
nunca receber o toque — e o sintoma é indistinguível de um bug no app.

A área real é o `AXGroup` chamado `iOSContentGroup`:

```bash
osascript -e 'tell application "System Events" to tell process "Simulator" \
  to tell window 1 to get {position, size} of every UI element' \
  | tr ',' '\n' | paste -sd' ' -
```

Procure o grupo cuja proporção bate com a do device (iPhone 17: `1206/2622 ≈ 0.46`).
Na sessão em que este roteiro foi escrito:

```
origem = (922, 429)   tamanho = 345 × 750     # aspecto 0.460 ✓
```

> **Em zsh, `$GEO` sem aspas não vira quatro argumentos.** Diferente do bash, o zsh não
> faz word-splitting de parâmetro não citado, e o `tap` responde com a mensagem de uso
> como se você tivesse errado a sintaxe. Use `${=GEO}`, ou passe os quatro números
> direto: `"$SP/tap" 922 429 345 750 0.5 0.697`.

Guarde como `GEO="922 429 345 750"`. **Reconfira sempre que a janela se mover ou o
device mudar.**

---

## 2 · O utilitário de toque

`tools/simctl/tap.swift` recebe a geometria e coordenadas **normalizadas** (0..1) da
tela do device, então o roteiro não depende de onde a janela está.

```bash
swiftc -O tools/simctl/tap.swift -o "$SP/tap"

"$SP/tap" $GEO 0.5 0.697                       # um toque
"$SP/tap" $GEO 0.5 0.55 0.5 0.62                # vários em sequência
"$SP/tap" $GEO drag 0.074 0.574 0.5 0.497       # arrastar (long-press + passos)
```

O arrasto é em passos pequenos com long-press antes: um salto único não gera os
eventos intermediários que o `.onDrag` do SwiftUI precisa.

Para digitar, o teclado de hardware do macOS é encaminhado ao device:

```bash
osascript -e 'tell application "Simulator" to activate'
osascript -e 'tell application "System Events" to keystroke "jogador@example.com"'
```

### Screenshot — use caminho absoluto

```bash
xcrun simctl io booted screenshot "$SP/tela.png"
```

Caminho relativo falha com *"the volume Macintosh HD is read only"*: o `simctl`
resolve contra outro diretório.

### Quando o toque não pega: `AXPress`

Alguns controles não respondem ao clique sintético mesmo com o ponto exatamente no
centro do frame (o avatar do perfil é um caso conhecido — **causa não identificada**;
por dedo funciona). Aí pressione pela árvore de acessibilidade:

```bash
# descobre os botões e seus frames
osascript <<'AS'
tell application "System Events" to tell process "Simulator"
  set out to ""
  repeat with e in (entire contents of window 1)
    try
      if role of e is "AXButton" then ¬
        set out to out & (position of e as text) & " | " & (size of e as text) & linefeed
    end try
  end repeat
  return out
end tell
AS

# pressiona o de uma posição específica (compare como texto: "1212495")
osascript <<'AS'
tell application "System Events" to tell process "Simulator"
  repeat with e in (entire contents of window 1)
    try
      if (role of e is "AXButton") and ((position of e as text) is "1212495") then
        perform action "AXPress" of e
        return "pressed"
      end if
    end try
  end repeat
  return "not found"
end tell
AS
```

A comparação por igualdade de texto funciona; comparar `item 1 of (position of e)`
com número, não.

---

## 3 · Atalhos de DEBUG

Existem para inspecionar tela contra o design system sem percorrer a navegação
inteira. Só em Debug, não mudam nada em Release.

```bash
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartAsGuest 1      # entra como visitante
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartTab placar     # trilhas | arena | placar
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartScreen perfil  # perfil | telao
```

`-LogNStartScreen perfil` é a forma confiável de abrir o hub de perfil sem depender
do toque no avatar.

Do login, com o app deslogado (instalação limpa: `xcrun simctl uninstall` antes):

```bash
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartScreen cadastro     # tela de cadastro
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartScreen termos       # termos de uso
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartScreen privacidade  # política
```

Somando `-AppleLanguages '(es)'` (ou `(en)`), a interface e o documento vêm nessa língua.

---

## 4 · Roteiro tela a tela

Coordenadas normalizadas medidas no iPhone 17 (1206×2622). Confira no screenshot
antes de confiar: a árvore desloca alguns pontos conforme o nó ativo muda de tamanho.

### 4.0 Abertura (splash)

A splash confere a sessão e manda a fila, e some quando a última linha fecha
(ADR 0010). Com rede local, isso leva milissegundos e não dá para fotografar. Os
casos que param numa tela são sem rede e sessão recusada.

A janela do Simulator precisa estar aberta (`open -a Simulator`) para a geometria
da seção 1 existir.

**Sem token (instalação limpa):** o app vai direto ao login, sem splash. A primeira
abertura depois de `simctl install` leva alguns segundos com a tela no canvas vazio.
Isso é o lançamento do app, não a splash.

**Sem rede, sessão no prazo (tela 03):** entre com a conta, derrube o backend
(`kill $(lsof -ti tcp:8080 -sTCP:LISTEN)`), responda um desafio para ter fila e
reabra o app. Com a fila vazia a tela 03 não aparece: o app entra direto, com a
tarja de sem rede.

| elemento | y |
|---|---|
| Tentar de novo | `0.905` (x `0.27`) |
| Continuar | `0.905` (x `0.73`) |

**Conferir:** linha `✓ sessão · token local válido`, linha `! sync · sem rede · N na
fila`, o aviso em `warn` e os dois botões. Suba o backend e toque em "Tentar de novo":
o app entra sem a tarja de sem rede. "Continuar" entra com a tarja.

**Sessão recusada (tela 04):** revogue as sessões no banco local e reabra.

```bash
docker exec logn-db-1 psql -U logn_user -d logn_db \
  -c "DELETE FROM refresh_tokens WHERE user_id = '<id da conta>'"
```

**Conferir:** login com o e-mail preenchido e "Sua sessão expirou. Entre de novo.".

**A fila é da conta:** com o app fechado, as chaves ficam no plist do sandbox:

```bash
D=$(xcrun simctl get_app_container booted sh.logn.LogNiOS data)
plutil -p "$D/Library/Preferences/sh.logn.LogNiOS.plist" | grep -E '"(offline_events|account_user_id)'
```

Tem de haver `account_user_id` e `offline_events:<id>`, e nunca `offline_events`
sozinha depois de uma abertura com rede. Para testar a migração, grave uma fila na
chave antiga com `plutil -replace offline_events -string '<json>'` (app fechado) e
reabra: o log do backend mostra `sync ok ... eventos=N`, e a chave antiga some.

Para rodar contra o backend local sem mexer no `.env`, passe
`API_BASE_URL=http://localhost:8080` ao `xcodebuild`.

### 4.1 Login

| elemento | y |
|---|---|
| campo e-mail | `0.555` |
| campo senha | `0.6235` |
| botão Entrar | `0.697` |
| Criar conta | `0.753` (x `0.13`) |
| Jogar como visitante | `0.868` |

```bash
osascript -e 'tell application "Simulator" to activate'
"$SP/tap" $GEO 0.5 0.555;  osascript -e 'tell application "System Events" to keystroke "jogador@example.com"'
"$SP/tap" $GEO 0.5 0.6235; osascript -e 'tell application "System Events" to keystroke "logn12345"'
"$SP/tap" $GEO 0.5 0.697
sleep 6
"$SP/tap" $GEO 0.315 0.626        # "Not Now" no diálogo de salvar senha do iOS
xcrun simctl io booted screenshot "$SP/01-login.png"
```

**Conferir:** avatar no topo mostra a inicial do e-mail (não `?`), e o mapa de
trilhas carrega sozinho — sem passar pelo estado vazio.

### 4.2 Árvore de trilhas

**Conferir:** sete nós com ícones distintos (dois deles usam o asterisco), ativo com miolo tint e traço accent; bloqueado como balão cinza inteiro, sem traço nem brilho; conquistado sempre com brilho, cordinha
sólida entre nós destravados e tracejada entre os fechados, contador com plural
certo (`1 balão`, não `1 balões`).

### 4.3 Sheet do nó

```bash
"$SP/tap" $GEO 0.5 0.272          # tag do nó 1 (confira no screenshot)
xcrun simctl io booted screenshot "$SP/02-no.png"
"$SP/tap" $GEO 0.5 0.901          # Começar partida
```

**Conferir:** o balão do cabeçalho usa as mesmas regras de estado.

### 4.4 Partida — SPOT_THE_BUG (problema A)

```bash
"$SP/tap" $GEO 0.5 0.5555         # toque na linha que o gabarito local indica
"$SP/tap" $GEO 0.5 0.907          # Confirmar
xcrun simctl io booted screenshot "$SP/03-veredito.png"
```

**Conferir:** a linha correta tem de dar **AC**. O `correct_line` do JSON é contado
a partir de 1 (como a numeração na tela) e o toque manda índice; se essa conversão
voltar a sumir, a resposta certa é recusada e o relatório ainda imprime a linha
certa em "sua resposta".

> Este passo mandava tocar numa linha diferente da que o gabarito indica hoje,
> porque o gabarito gravado no seed estava errado. O bug está consertado na
> migração `0004`, mas o roteiro tinha virado cúmplice dele: seguir o roteiro dava
> AC e confirmava o erro. Quando o gabarito de um desafio mudar, este número muda
> junto.

### 4.5 Partida — FILL_IN_THE_BLANK (problema B)

```bash
"$SP/tap" $GEO 0.5 0.907                       # Próximo problema
"$SP/tap" $GEO drag 0.074 0.574 0.5 0.497       # arrasta o 1º bloco para "solte aqui"
"$SP/tap" $GEO 0.5 0.907                       # Confirmar resposta
```

**Conferir:** os blocos existem. Sem `options` no payload a tela abre com a lacuna e
nada para arrastar — o problema fica sem resposta possível. A CHECK constraint da
`0000_schema_inicial.sql` cobre isso hoje.

### 4.6 Partida — DRY_RUN (problema C)

```bash
"$SP/tap" $GEO 0.5 0.907          # Próximo problema
"$SP/tap" $GEO 0.5 0.7065         # console "SAÍDA PREVISTA"
osascript -e 'tell application "System Events" to keystroke "9"'
"$SP/tap" $GEO 0.5 0.907          # Confirmar saída
```

**Conferir:** painel de watch com as variáveis e a nota ("antes da linha 4"), e
"espaços ignorados" aparecendo quando há texto.

### 4.6b Partida — COMPLEXITY_MATCH e TAG_THE_PATTERN (problemas D e E)

Dois campos de soltar (TEMPO e ESPAÇO) e uma grade de marcar, respectivamente. Os
blocos e as tags vêm de `content.options`; o gabarito, de `content.correct_options` —
no COMPLEXITY_MATCH a ordem importa: `[tempo, espaço]`.

**Conferir:** o `Confirmar` só habilita com os dois campos preenchidos (complexidade)
ou com exatamente `max_selections` tags marcadas.

### 4.7 Vereditos e armadilhas

- **AC** — balão na cor da letra, `+50 XP`, fundo verde.
- **WA** — `+20 min pen`, `−1 vida`, e o cartão de trap embaixo.
- **TLE** — deixe o relógio da questão zerar (60s) sem responder.

Com a partida encerrada o botão vira **Ver o relatório** e a linha de baixo diz se o
nó ficou completo ou em aberto — os dois têm de concordar.

### 4.8 Relatório pós-partida

```bash
"$SP/tap" $GEO 0.5 0.878          # Ver o relatório / Continuar
xcrun simctl io booted screenshot "$SP/04-relatorio.png"
"$SP/tap" $GEO 0.5 0.904          # Entendi → volta para a trilha
```

**Conferir:** títulos com **uma** letra (`A · <nome do problema>`, não `A · A · …`),
e uma explicação **diferente por problema** — se os três erros vierem com o mesmo
texto, o desafio perdeu o `validation.explanation` e o motor caiu no genérico.

### 4.8b Progresso volta do servidor

Feche o app, abra de novo e confira que o XP e os nós destravados continuam lá:

```bash
xcrun simctl terminate booted sh.logn.LogNiOS
xcrun simctl launch booted sh.logn.LogNiOS
sleep 6 && xcrun simctl io booted screenshot "$SP/07-reabre.png"
```

Abrir em `0 XP` com eventos já sincronizados significa que `GET /api/v1/progress`
não foi chamado ou não voltou.

### 4.9 Perfil — sincronizado e com fila

```bash
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartScreen perfil
# ou AXPress no avatar (seção 2)
```

| elemento | y |
|---|---|
| botão Tentar (cartão de fila) | `0.531` (x `0.813`) |
| Sair da conta | `0.8515` |
| Gerenciar conta | `0.908` |

Jogar uma partida e voltar ao perfil produz o **estado de fila**: ponto `warn`,
`N EVENTOS NA FILA`, cartão "XP ainda só existem neste aparelho", badge no avatar.
Tocar em **Tentar** sincroniza:

```bash
"$SP/tap" $GEO 0.813 0.531
sleep 4
tail -3 "$SP/backend.log"    # espera: "sync ok: user=<uuid> eventos=N topo=<hash>"
docker exec logn-db-1 psql -U logn_user -d logn_db \
  -c 'SELECT user_id, count(*) FROM game_events GROUP BY user_id;'
```

O `user_id` tem de ser o **UUID** da conta. Se aparecer `user_1`, o cliente perdeu a
identidade da sessão e o Postgres recusa o insert.

Depois do sync: ponto verde, `TUDO SINCRONIZADO`, cartão some, sheet encolhe.

### 4.10 Gerenciar conta

```bash
"$SP/tap" $GEO 0.5 0.908
xcrun simctl io booted screenshot "$SP/05-conta.png"
"$SP/tap" $GEO 0.062 0.1045       # voltar
```

Vira sheet de altura cheia. **Não toque em "Excluir minha conta".**

### 4.11 Saída com desfazer (fila vazia)

```bash
"$SP/tap" $GEO 0.5 0.8515         # Sair da conta
xcrun simctl io booted screenshot "$SP/06-saida.png"
"$SP/tap" $GEO 0.5 0.904          # "saiu por engano? desfazer"
```

**Conferir:** "Até a próxima, **\<primeiro nome\>**" — nome vazio significa que o
e-mail da sessão não sobreviveu ao login. O desfazer devolve a sessão inteira.

Para seguir para o login em vez de desfazer: `0.8455` (**Entrar de novo**).

### 4.12 Sheet crítico de saída (fila cheia)

Com eventos pendentes, **Sair da conta** abre o sheet crítico em vez da saída direta.

| elemento | y |
|---|---|
| Sincronizar e sair | `0.769` |
| Continuar conectado | `0.832` |
| Sair e descartar N XP | `0.884` |

**Conferir:** "Sincronizar e sair" tem de **subir a fila antes de sair**. Se o sync
falhar, o app fica onde está e a fila continua ali — sair antes da resposta apaga
progresso que nunca chegou ao servidor.

### 4.13 Termos de uso e política de privacidade

A página vem do servidor (`/legal/<kind>?embed=1`). Para cair na cópia offline, o
bundle precisa dela: `LEGAL_BUNDLE_ALLOW_DRAFT=1 just legal-bundle` com o backend de pé,
e `xcodegen generate` antes do build para os arquivos entrarem no projeto. Para o app
falar com o backend local em vez da produção, passe a URL no build:
`xcodebuild ... 'API_BASE_URL=http:/$()/localhost:8080' build`.

```bash
xcrun simctl uninstall booted sh.logn.LogNiOS; xcrun simctl install booted "$APP"
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartScreen termos -AppleLanguages '(pt-BR)'
sleep 10   # o processo do WebKit leva uns segundos na primeira abertura
xcrun simctl io booted screenshot "$SP/13-termos.png"
```

**Conferir:**
- sem clarão branco antes do texto; enquanto carrega, linhas de esqueleto;
- cabeçalho com título e `Versão 1 · vigente desde <data por extenso>` na língua do app;
- documento com marcador de rascunho abre com a faixa `RASCUNHO` no topo;
- sem título nem linha de versão dentro da página — eles já estão no cabeçalho;
- com o backend parado, a mesma tela abre da cópia do bundle, com `Cópia salva no
  aparelho` no cabeçalho;
- os links "Termos de uso · Política de privacidade" aparecem no login, no cadastro
  (abaixo das caixas) e no rodapé do perfil, para visitante e para conta;
- no cadastro, "Enviar código" só libera com as duas caixas marcadas **e** as versões
  vigentes buscadas do servidor;
- a caixa de idade diz "Tenho N anos ou mais" com o N que o servidor devolveu para o
  país do aparelho (`GET /api/v1/legal/current?country=…`, 13 por padrão).

### 4.14 Análise de uso e exclusão com carência

```bash
xcrun simctl launch booted sh.logn.LogNiOS -LogNStartAsGuest 1 -LogNStartScreen perfil
```

**Conferir:**
- o interruptor "Análise de uso" fica no rodapé do perfil, acima dos links legais,
  para visitante e para conta, e vem ligado;
- desligar grava `analytics_disabled = 1` e vale na hora: o app reinicia o PostHog sem
  a captura de abertura e fechamento, e o Core para de mandar `identify` e os eventos de
  desafio (erros e medições continuam, com identificador anônimo);
- em Gerenciar conta, o bloco diz `EXCLUSÃO EM 30 DIAS`;
- excluir leva à tela "Conta desativada", com a data até quando entrar recupera;
- entrar de novo com a mesma conta mostra, uma vez, o aviso "Exclusão cancelada".

Pela API, sem tocar na tela: `POST /api/v1/users/me/delete` devolve `purge_after`, o
refresh seguinte dá 401, e o login devolve `account_restored: true`.

### 4.15 Conteúdo em outra língua

A língua do app vem de `-AppleLanguages`; o conteúdo segue a do app, e o que não saiu
na língua dele cai em português (ADR 0009). Antes, confira o que a API publica:

```bash
for l in pt-BR en es; do
  printf '%s: ' $l; curl -s "localhost:8080/api/v1/nodes?lang=$l" | python3 -c 'import json,sys;print(len(json.load(sys.stdin)),"nós")'
done
```

Instalação nova, sem rede, em cada língua — é a semente do bundle que aparece:

```bash
xcrun simctl uninstall booted sh.logn.LogNiOS
xcrun simctl install booted "$SP/dd/Build/Products/Debug-iphonesimulator/LogNiOS.app"
xcrun simctl launch booted sh.logn.LogNiOS -AppleLanguages '(es)' -LogNStartAsGuest 1
```

Repita com `'(en)'` e `'(de)'`.

**Conferir:**
- nome dos nós, enunciado, explicação e rótulos de TAG na língua do app quando a
  semente tem essa língua; em português, inteiros, quando não tem (`de` sempre cai);
- nunca mistura: um nó não tem desafio numa língua e desafio em outra;
- cor e ícone de cada nó iguais nas três línguas (vêm de `topic`, não do nome);
- cartão de WA e de TLE com a moldura ("RESPOSTA ERRADA", "TIME LIMIT EXCEEDED",
  "linha N") na língua do app e a explicação na língua do conteúdo;
- cadastro com e-mail inválido, senha curta e e-mail já usado dá a frase de cada caso,
  não a genérica;
- com conta e progresso, trocar a língua do app em Ajustes: o XP e os problemas
  resolvidos continuam, e o texto troca. Sem rede, troca pela semente; com rede, busca
  de novo.

---

## 5 · Lendo o que deu errado

```bash
# log do device, filtrado
xcrun simctl spawn booted log stream --predicate 'process == "LogNiOS"' --style compact \
  | tee "$SP/dev.log" &

# só a rede
xcrun simctl spawn booted log show --last 3m --predicate 'process == "LogNiOS"' \
  --style compact | grep -i '8080'
```

O backend loga o motivo de um sync recusado (`sync recusado:`, `sync não gravado:`,
`sync rebase:`) e de um OTP recusado. **Ausência de log de erro não é prova de
sucesso** — confira o banco.

---

## 6 · Armadilhas conhecidas

- **Nunca dispare `alert`/`confirm`** no app: o diálogo modal trava a sessão de
  automação inteira.
- **Toque em cima do frame certo e mesmo assim nada acontece** → tente `AXPress`
  antes de concluir que é bug do app.
- **Relógio da questão são 60s.** Ler um screenshot com calma já basta para o
  problema virar TLE. Isso mascarou um erro de julgamento nesta sessão.
- **Screenshot depois de uma transição** precisa de `sleep 2–3`; sem isso você
  fotografa a tela anterior e conclui que o toque não pegou.
- **`xcrun simctl spawn booted defaults` não é o `UserDefaults` do app.** Ele lê e
  grava num domínio fora do sandbox: o comando responde normalmente, e o app não vê
  nada. Leia e grave o plist em `$(xcrun simctl get_app_container booted
  sh.logn.LogNiOS data)/Library/Preferences/`, com o app fechado.
