# Roteiro de teste no aparelho Android — tela a tela

O par do `roteiro-simulador-ios.md`: como percorrer o LogN num telefone Android conectado
por USB, com o backend local, só por linha de comando. Cada tela se confere lado a lado
com o simulador iOS, que é a referência (ADR 0023).

Uma seção por fase do cliente, da abertura ao build de release.

---

## 0 · Antes de começar

### Aparelho

Depuração USB ligada (Opções do desenvolvedor), e o Mac autorizado no diálogo do telefone.

```bash
adb devices                      # o serial aparece como "device", não "unauthorized"
adb shell wm size                # geometria, para converter dp em pixel
adb shell wm density
```

No aparelho de teste, 1080×2400 a 440 dpi: **1 dp = 2,75 px**. Toque e captura são em
pixel do aparelho.

### Infra

A mesma do iOS: Postgres, Mailpit e o backend em `:8080`.

```bash
docker compose up -d db mailpit
(cd backend && go run .) &
just android/reverse             # o localhost:8080 do telefone passa a ser o do Mac
```

O `adb reverse` cai quando o cabo sai ou o `adb` reinicia; `just android/install` e
`just android/guest` o refazem.

### Build e instalação

```bash
just android/generate            # depois de mexer no Core ou no catálogo
just android/seed                # semente e documentos legais nos assets
just android/install
```

---

## 1 · Ferramentas

```bash
just android/shot /tmp/a.png                       # captura
just android/tap 540 1200                          # toque, em pixel
adb shell input swipe 540 1800 540 600 300         # arrasto (x0 y0 x1 y1 ms)
adb shell input text 'a%sb'                        # texto; %s é espaço
adb shell input keyevent KEYCODE_BACK              # voltar
adb logcat -s logn-android                         # log do Core, só em debug
```

**Abertura em vídeo.** A splash do Compose dura o boot, e sem sessão isso é um quadro.
Para ver a transição, grave e corte em quadros:

```bash
adb shell am force-stop sh.logn.app
adb shell screenrecord --time-limit 5 /sdcard/boot.mp4 &
sleep 0.5; adb shell am start -n sh.logn.app/.MainActivity; sleep 6
adb pull /sdcard/boot.mp4 /tmp/boot.mp4
ffmpeg -loglevel error -y -i /tmp/boot.mp4 -vf "fps=4,scale=270:-1,tile=6x3" -frames:v 1 /tmp/boot.png
```

### Atalhos de DEBUG

| Extra | Efeito | iOS |
|---|---|---|
| `--ez logn.start_as_guest true` | entra como visitante | `-LogNStartAsGuest 1` |

`just android/guest` fecha o app e abre com esse extra.

### Links

```bash
adb shell am start -a android.intent.action.VIEW -d 'logn://verify?email=a@example.com&code=123456'
```

Os códigos saem do Mailpit (`localhost:8025`), como no iOS.

---

## 2 · Abertura

**Conferir:**

- A splash de sistema (o balão sobre o canvas) passa para a do Compose sem clarão branco.
- Sem sessão: a splash some em cerca de um quadro, com fade de 0,25 s. É o mesmo do iOS.
- Com sessão (depois da fase de login): o balão sobe em 0,9 s, e cada verificação imprime
  a sua linha e o veredito.
- Sem rede e com a sessão no prazo: para, mostra o aviso e os botões "Tentar de novo" e
  "Continuar offline".
- Animação reduzida (`adb shell settings put global animator_duration_scale 0`): o balão
  já nasce no lugar. Volte com `1`.
- Fonte em 200% (`adb shell settings put system font_scale 2.0`): as linhas de boot não
  cortam o veredito. Volte com `1.0`.
- Backend parado: o boot termina com erro de rede na linha, sem crash (`adb logcat -b
  crash` vazio).

---

## 3 · Login, cadastro e senha

As coordenadas abaixo são do aparelho de teste (1080×2400); a captura reduzida da
ferramenta de leitura mostra 900 de largura, então multiplique o que vê nela por 1,2.

**Login.**

```bash
adb shell input tap 540 948;  adb shell input text 'jogador@example.com'
adb shell input tap 540 1114; adb shell input text 'errada123'
adb shell input keyevent KEYCODE_BACK; adb shell input tap 540 1290
```

- Senha errada: a linha "E-mail ou senha não conferem." em `wrongInk`, sob os links.
- O botão do GitHub (e o do Google) só aparece com a flag do PostHog ligada e o client no
  `.env`. Sem chave de telemetria, nenhum aparece, como no iOS.
- Sair para o cadastro depois de um erro **não** oferece salvar a senha errada.
- "Esqueci a senha" sem e-mail: aviso no topo e o foco volta ao campo.

**Cadastro.** Criar conta → e-mail, as duas caixas, "Enviar código". O código sai do
Mailpit:

```bash
curl -s "localhost:8025/api/v1/search?query=to:android1@example.com" \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['messages'][0]['Snippet'])"
adb shell input text 123456      # as seis caixas sobem sozinhas no sexto dígito
```

- Com o código aceito, o passo vira "Definir Senha". Senhas diferentes pintam a borda.
- Conta criada: o app entra, e o Android oferece salvar a senha (esta sim).

**Documentos.** "Termos de uso" no pé do login abre o documento com "Versão N · vigente
desde …". Com o backend parado, abre a cópia dos assets com "Cópia salva no aparelho".

**Link de redefinição.**

```bash
adb shell am start -a android.intent.action.VIEW -d 'logn://reset-password?email=jogador@example.com&code=123456'
```

A tela de nova senha sobe por cima de qualquer outra, sem o campo do código.

---

## 4 · Termos atualizados

Para ver a tela sem publicar versão nova, recue o aceite da conta de teste no banco
local:

```bash
docker exec logn-db-1 psql -U logn_user -d logn_db -c \
  "update legal_acceptances set version=3 where user_id=(select id from users where email='android1@example.com')"
adb shell am force-stop sh.logn.app && adb shell am start -n sh.logn.app/.MainActivity
```

- "DIFF DE TERMOS", v3 → v4 e a lista das mudanças com o sinal na faixa tingida.
- "Aceitar e continuar" só com a caixa marcada. Aceito, o registro sai com
  `platform = android` e a versão do app:

```bash
docker exec logn-db-1 psql -U logn_user -d logn_db -Atc \
  "select kind, version, platform, app_version, source from legal_acceptances a join users u on u.id = a.user_id where u.email = 'android1@example.com'"
```

- "Não concordo" abre a folha com sair e excluir a conta.

---

## 5 · Abas, mapa e nó

Pontos fixos no aparelho de teste: abas em `y = 2280` (Trilhas 180, Arena 540, Placar
900), avatar do perfil em `965 220`, seletor de trilha na seta ao lado do nome.

- Conta nova: o onboarding vem antes do mapa, uma vez.
- Mapa: nó concluído com o balão cheio e a contagem em verde; o nó em andamento em
  "INFLANDO · n/m RESOLVIDOS" com a borda acesa; arestas fechadas tracejadas, e a legenda
  "cordinha tracejada" no pé.
- Tocar num nó abre a folha com as três estatísticas na mesma altura e o botão de jogar.
- Sem rede: a faixa amarela sob o cabeçalho, com o ícone visível mesmo quando a frase quebra.

---

## 6 · Partida

Os seis formatos se conferem contra o simulador com o mesmo nó aberto nos dois.

```bash
# Arrasto longo de um chip (encaixar linha, ordenar): segurar, mover, soltar.
adb shell input motionevent DOWN 300 1700
sleep 0.6
adb shell input motionevent MOVE 500 1200
adb shell input motionevent UP 500 1200
```

- Tocar num chip e depois na lacuna também encaixa (o caminho de quem não arrasta).
- O relógio para com o app em segundo plano e volta de onde estava.
- Estourar o tempo: veredito TLE com a letra do desafio que estava em jogo, não da
  seguinte.
- Errar: o veredito treme (sem tremor com animação reduzida) e o relatório traz o cartão
  de revisão do erro.
- Sair no meio: a folha de confirmação; confirmado, volta ao mapa e não reabre a partida.

---

## 7 · Perfil, conta e saída

- O avatar abre o perfil: nível, barra de XP, as três estatísticas, "Armazenamento",
  "Restaurar compras", "Sair da conta", "Gerenciar conta", análise de uso e os documentos.
- Sair com fila pendente: o aviso crítico antes; sair limpo: a tela de despedida com
  "Desfazer".
- Gerenciar conta → excluir: só com a palavra de confirmação digitada.

---

## 8 · Catálogo, trilha paga e compra

Para ver uma trilha paga sem comprar, dê à conta de teste acesso de revisora, ou um
direito, **só no banco local**:

```bash
docker exec logn-db-1 psql -U logn_user -d logn_db -c \
  "insert into entitlements (user_id, track_id, original_transaction_id, status, provider)
   select id, '<uuid da trilha>', 'local-android-test', 'active', 'google_play'
   from users where email = 'android1@example.com'"
```

- Catálogo: a principal no topo com "PRINCIPAL · GRÁTIS", as pagas em grade com o selo
  ("COMPRADA", preço ou "VER").
- Página da trilha comprada: "vale até …" (a janela offline) e "Continuar · Nó n".
- Comprar sem produto criado no Play Console: "Esta trilha não está à venda agora". A
  compra de verdade só se testa com o produto ativo e o app vindo do Play (teste interno).
- Visitante que toca em comprar: a folha que pede conta, e a compra segue sozinha quando
  a conta abre.

---

## 9 · Placar e telão

- Aba Placar: "Global" e a sede do jogador, sublinhado de 2 dp na ativa; a linha do
  jogador grudada no pé, tingida, com a borda de acento em cima e embaixo.
- A tarja azul de dados de exemplo enquanto o ranking não tem servidor.
- "Ver o telão completo": rank e equipe ficam fixos e SLV, PEN e A–M rolam juntos.
  Arraste longe da borda direita, senão o Android lê o gesto de voltar:

```bash
adb shell input swipe 850 900 300 900 400
```

- A legenda das quatro cores sempre visível, em duas linhas, sem cortar texto.
- Fora de partida não há relógio na barra; o voltar do sistema fecha o telão.

---

## 10 · Armazenamento e sem rede

O pacote de uma trilha paga mora em `filesDir/TrackPackages`, com o nome em hash:

```bash
adb shell run-as sh.logn.app ls -la files/TrackPackages
```

- Com um direito ativo, a abertura com rede baixa o pacote sozinha. "Armazenamento"
  mostra o tamanho e a lixeira.
- Apagar: o arquivo some, a linha vira "Não baixada · Baixar". "Baixar" traz de volta.
- Sem rede (`adb reverse --remove tcp:8080`): o selo "SEM REDE", e a trilha comprada abre
  e joga a partir do pacote.
- Abrir sem rede, apagar, religar a rede (`just android/reverse`) e tocar em "Baixar": o
  Core renova o token antes de pedir a licença, e o pacote volta.

Cuidado ao repetir o teste por toque cego: depois da abertura com rede o pacote já voltou,
e o mesmo ponto da tela é a lixeira, não o "Baixar".

---

## 11 · Build de release

O release recusa `API_BASE_URL` sem `https://`, então ele fala com o servidor do `.env`.
A assinatura difere da de debug, e o Android recusa instalar por cima: desinstale o
debug antes (a receita para e diz o comando), e reinstale o debug depois.

```bash
adb uninstall sh.logn.app
just android/install-device          # release no celular, o install-ios-device do Android
```

Para um teste sem sujar a análise de uso, desligue a telemetria no build à mão:
`(cd android && ./gradlew :app:assembleRelease -Plogn.TELEMETRY_KEY=)`.

O build de loja é `just android/release`: confere conteúdo e documentos legais, empacota a
trilha e os documentos da produção nos assets e gera `android/build/store/LogN.aab`,
assinado, com o mapeamento do R8 e os símbolos do `.so`. Suba o `versionCode` antes de
cada envio: o Play recusa um repetido.

- Abre até o login com "CORE PRONTO" no pé: a ponte JNI sobreviveu ao R8.
- "Termos de uso" abre o documento.
- `adb logcat -b crash` vazio.
- Volte: `adb uninstall sh.logn.app && just android/generate && just android/install`.
