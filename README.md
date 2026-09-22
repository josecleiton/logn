# LogN

Treino de programação competitiva em formato de contest, para iOS.

Monorepo com três camadas: backend em **Go** com PostgreSQL, um núcleo em **Rust/Crux**
que carrega toda a lógica de negócio e o motor offline-first, e um cliente **SwiftUI**
que só renderiza o que o núcleo manda e devolve eventos.

## O conteúdo não está aqui

Este repositório tem o app; não tem a trilha. Os desafios, enunciados, gabaritos e
explicações vivem separados, e um clone limpo sobe um backend com a estrutura certa e
nenhum problema para jogar.

A divisão é por natureza: o schema do banco, as `CHECK CONSTRAINTS` que recusam desafio
insolúvel, o motor que julga as respostas e as telas são engenharia, e estão todos aqui.
O currículo é outra coisa.

Para rodar com conteúdo próprio, escreva uma migração que popule `skill_nodes` e
`challenges` e ponha em `backend/schema/migrations/`. As constraints dizem na hora do
`INSERT` se o desafio tem resposta possível.

## Licença

Código sob **Apache License 2.0** — ver [LICENSE](LICENSE). Use, modifique e
redistribua, inclusive comercialmente.

**O nome LogN, o símbolo e a identidade visual não estão nessa licença.** Um derivado é
bem-vindo; ele só precisa de outro nome e outra cara. Os detalhes estão em
[TRADEMARKS.md](TRADEMARKS.md), e as atribuições de terceiros — incluindo as fontes IBM
Plex, que são OFL — em [NOTICE](NOTICE).
