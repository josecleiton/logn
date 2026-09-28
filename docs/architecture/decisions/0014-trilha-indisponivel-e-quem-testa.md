# ADR 0014: Trilha indisponível e quem a testa

## 1. Visão Geral

Uma trilha paga entra no banco antes de estar pronta para venda: é preciso jogá-la no aparelho de verdade, com a compra em Sandbox, antes de ela aparecer para todo mundo. Até aqui, trilha no banco com `status = 'active'` aparecia no catálogo de qualquer um, e a amostra dela saía em claro em `GET /api/v1/challenges`, sem conta.

## 2. Decisão

**Uma flag por trilha.** `tracks.available` (padrão `true`). Com `false`, a trilha sai da vitrine. A gratuita não pode ficar indisponível (`chk_tracks_free_available`): sem ela o app abre sem trilha nenhuma.

**Quem testa, por trilha.** `track_previewers(track_id, user_id)` libera uma conta numa trilha. Entra e sai por SQL, à mão; não há rota que escreva ali, e nenhum e-mail ou id de conta vai para o repositório. A linha some com a conta (`ON DELETE CASCADE`).

**A compra vale mais que a flag.** Quem tem direito ativo à trilha continua vendo, com ou sem flag. Tirar uma trilha da vitrine não pode sumir com o que alguém pagou. Direito revogado não conta, como já não contava para a descontinuada.

**Uma regra, num lugar só.** `visibleTrack` em `repository.go`, ao lado de `openNode`: disponível, ou a conta testa, ou a conta tem direito ativo. Ela filtra o catálogo (`GET /api/v1/tracks`), os nós (`GET /api/v1/nodes`), os desafios abertos (`GET /api/v1/challenges`) e decide o XP da amostra no sync. Mudar uma é mudar as quatro.

**As rotas de conteúdo passam a ler a conta.** `/nodes` e `/challenges` faziam como o visitante mesmo com token. Agora, como `/tracks` já fazia: sem cabeçalho é visitante; token válido vê o que a conta vê, e a resposta sai `private, no-store`; token presente e inválido é 401, para o app renovar a sessão em vez de receber a vitrine do visitante. O Core já mandava o token nessas rotas e já trata o 401 delas com refresh.

**Testar não é comprar.** Quem está em `track_previewers` vê o catálogo, os nós e a amostra. O resto da trilha continua só no pacote cifrado, com direito ativo; para jogar tudo, quem testa compra em Sandbox, como a App Review.

## 3. Alternativas descartadas

- **Um valor novo em `status`.** `status` já diz se a trilha está à venda ou foi descontinuada, e "ainda não abriu" é uma dimensão à parte: uma trilha pode estar indisponível e à venda para quem testa.
- **Uma marca na conta que libera toda trilha indisponível.** Mais curta, mas misturava "quem é da equipe" com "o que esta pessoa testa", e nasceria sem o recorte por trilha na primeira vez em que alguém de fora testasse uma só.
- **Filtrar só o catálogo.** A amostra continuava em claro em `/challenges` para qualquer um, e o XP dela seguia pago.

## 4. Consequências

- `/nodes` e `/challenges` com token passam por `IsUserActive`, uma leitura a mais por pedido de quem entrou.
- Um cache no caminho não guarda a resposta de quem entrou.
- Abrir uma trilha é `UPDATE tracks SET available = true`. Nada no app muda.
