# ADR 0029: Conteúdo revalidado por ETag, guardado só no aparelho

Substitui, na ADR 0014, a frase "a resposta sai `private, no-store`".

## 1. Contexto

`/challenges`, `/nodes` e `/tracks` respondem o conteúdo da trilha na língua pedida. O
visitante recebia `Cache-Control: no-cache`; a conta, `private, no-store` (ADR 0014), para
que um cache no caminho nunca servisse a uma pessoa o que só outra vê: a amostra da
trilha indisponível que ela testa, a trilha que ela comprou.

Nenhuma das duas respostas tinha validador. Sem ETag, o `no-cache` do visitante nunca
virava 304, e o `no-store` da conta impedia até o aparelho de guardar. Toda abertura do
app baixava a trilha inteira de novo, e o Android, sem cache HTTP no OkHttp, nem tentava.
O conteúdo só muda quando sai migração nova, então quase sempre o que descia era igual ao
que o aparelho já tinha.

## 2. Decisão

**ETag fraco tirado do corpo.** As três rotas passam por `writeRevalidatedJSON`
(`backend/internal/httpapi/server.go`): o corpo é serializado, o ETag é
`W/"<16 bytes do SHA-256 em hex>"`, e um `If-None-Match` que bate responde 304 sem corpo.
Fraco porque o gzip muda os bytes que saem, e um ETag forte prometeria os mesmos bytes. O
banco roda igual; o que se poupa é a banda e o parse no app.

**A conta passa a `private, no-cache`, com `Vary: Accept-Language`.** `private` continua
proibindo cache compartilhado, que é o que a ADR 0014 protegia. O `no-cache` deixa o
aparelho guardar, mas obriga a perguntar ao servidor a cada uso. A pergunta leva o token
da conta que está no app, e o servidor confere o ETag contra o corpo dessa conta: outra
conta no mesmo aparelho, com outro corpo, recebe 200, nunca o 304 da primeira. O
visitante segue `no-cache`.

`Authorization` fica fora do `Vary` de propósito. O access token troca a cada 15 minutos,
e o cache só casa um pedido com a entrada guardada quando todo cabeçalho do `Vary` é
igual: com ele, quase toda abertura do app, que vem depois de um refresh, baixava a trilha
inteira de novo. A proteção que ele daria já está no `no-cache` e na conferência do
servidor.

**A conta saindo leva o cache.** O Core apaga o retrato offline (`offline_snapshot`) só
quando tira a conta do aparelho: ao sair e ao excluir a conta. Os dois shells usam esse
apagamento como sinal e esvaziam o cache HTTP no mesmo passo, para o conteúdo e o catálogo
da conta (o que ela comprou) não ficarem no disco depois dela.

**Toda rota autenticada nasce `private, no-store`.** `authenticate` põe o padrão, e só a
rota que sabe o que faz troca, como as de conteúdo. O `/progress` saía sem `Cache-Control`,
e com o cache do Android ligado o XP da conta iria para o disco.

**Os clientes revalidam sozinhos.** No iOS, o `URLSession.shared` já tinha `URLCache`. No
Android, o `OkHttpClient` ganha um `Cache` de 10 MiB em `cacheDir/http`. Nos dois, o 304
chega ao Core como 200 com o corpo guardado, e o Core não muda. O do Android está provado
em `OkHttpPortTest`, inclusive com o token trocado entre os pedidos; o do `URLSession` foi
conferido no macOS, que usa a mesma pilha do Foundation, e não no simulador.

## 3. Alternativas descartadas

- **Cache das consultas na memória do servidor.** Tiraria o banco do caminho, mas a
  resposta depende da conta (`visibleTrack`), e filtrar em Go duplicaria a regra que a
  ADR 0014 quer num lugar só. As consultas levam menos de 10 ms.
- **O Core guardar o ETag e mandar o `If-None-Match`.** Funciona igual, mas põe no Core e
  no armazenamento offline o que os caches HTTP das plataformas já fazem.

## 4. Consequências

- O conteúdo das rotas abertas fica no cache HTTP do aparelho enquanto a conta está nele,
  junto da cópia offline que o app já guardava, e sai com ela. Conteúdo fechado continua
  só no pacote cifrado (ADR 0013), que segue `no-store`.
- Quem mudar o Core para apagar `offline_snapshot` em outro momento leva o cache HTTP
  junto. Não quebra nada, só faz o aparelho baixar de novo.
- O pedido continua indo ao servidor a cada abertura, e o banco continua rodando: o ganho
  é a trilha não descer de novo, não o servidor deixar de trabalhar.
- Quem mudar a forma de serializar essas respostas muda o ETag de todo mundo, e cada
  aparelho baixa uma vez de novo. Não quebra nada.
