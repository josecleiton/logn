# ADR 0020: Novo aceite dos termos

## 1. Visão Geral

Os termos prometem, na seção de mudanças, pedir o aceite de uma versão relevante antes de a pessoa continuar usando a conta. O app não pede: nada na abertura confere se a versão aceita é a vigente, e por isso `gen_documentos_legais.py` recusa `--material` acima da versão 1 (`APP_PEDE_REACEITE`, ADR 0010). A primeira versão relevante é a das trilhas pagas. Esta ADR fecha o bloqueio, no desenho de `LogN Splash` (telas 01 a 03, "Fluxo · ordem das verificações", "Comparação de versão" e "Registro do aceite · LGPD").

O backend tinha `pending` e `accept` desde a 0008, sem ninguém chamando. O `pending` devolvia uma linha por língua com o corpo inteiro, ignorava `material` e `effective_at`, e o `accept` gravava um documento por pedido.

## 2. Decisão

**A abertura ganha a terceira linha: termos.** A ordem é a do design: sessão, sync, termos. O sync vem antes para nenhum XP depender do aceite. A linha só roda com sessão e rede; sem uma das duas, ela fecha como "adiado" e o app entra, com os termos conferidos na próxima abertura com rede. Ela espera a rede pelo mesmo `BOOT_WAIT_SECS` das outras; passou, é "adiado". O login também confere, porque uma conta antiga entrando num aparelho novo não passa pela splash com sessão.

**O servidor decide o que bloqueia; o app não lê número de versão.** `GET /api/v1/legal/pending` devolve, para a conta, cada documento cuja vigente é maior que a aceita:

```json
{
  "blocking": true,
  "documents": [
    { "kind": "terms", "version": 3, "effective_at": "2026-11-01", "material": true,
      "sha256": "…", "accepted_version": 2, "accepted_effective_at": "2026-09-28" }
  ],
  "changes": [
    { "id": "terms:3:trilhas-pagas", "kind": "terms", "version": 3,
      "change": "added", "section": "trilhas-pagas", "summary": "…" }
  ]
}
```

- A vigente é a maior versão com `effective_at` já passado. Publicar antes da vigência não põe a versão no ar, nem aqui, nem na página, nem no cadastro.
- `material` é verdadeiro se **qualquer** versão entre a aceita e a vigente for relevante: pular uma relevante no meio não a torna menor.
- `changes` soma todas as versões puladas, na língua negociada (`locale.Negotiate`), com o português quando faltar a língua, e cada item diz de que versão veio, como pede o design.
- `sha256` é do corpo HTML da vigente naquela língua, calculado pelo servidor.

**O "o que mudou" é texto escrito, por versão, no repositório de conteúdo.** Ao lado dos HTML, `logn-conteudo/legal/mudancas/v<N>.toml` lista cada mudança dos dois documentos: sinal (`added`, `changed`, `removed`), id da seção e a frase nas três línguas. `gen_documentos_legais.py` exige o arquivo em toda versão acima da 1, confere que cada seção existe no fragmento (ou, na remoção, que existia na versão anterior) e gera as linhas de `legal_document_changes`. Versão relevante sem mudança listada é recusada: bloquear alguém para dizer "algo mudou" não informa nada.

**Bloqueio e aviso, pela regra do design.**

- **Nada pendente:** nada aparece.
- **Só versões não relevantes:** uma faixa discreta na árvore, uma vez. O aceite é gravado quando a faixa aparece; fechar também conta.
- **Alguma relevante:** a tela 02 cobre o app. Ela mostra a versão aceita e a vigente, com as datas, e o diff. A caixa nunca vem marcada, e "Aceitar e continuar" fica desligado até marcar. "Não concordo" abre uma folha com "Sair da conta" e "Excluir conta": quem está bloqueado não alcança o Perfil, então a exclusão tem de estar ali.

**O aceite grava o que prova o consentimento.** `POST /api/v1/legal/accept` passa a receber os dois documentos num pedido e grava os dois numa transação:

```json
{
  "documents": [{ "kind": "terms", "version": 3, "locale": "pt-BR", "sha256": "…", "from_version": 2 }],
  "shown_changes": ["terms:3:trilhas-pagas"],
  "client": { "app": "1.8.0", "platform": "ios" },
  "source": "reaccept"
}
```

- Só a vigente se aceita (`legal_version_outdated`), e o `sha256` tem de ser o do documento que o servidor serve naquela língua: é a prova de que o texto aceito é o que estava na tela.
- `shown_changes` tem de estar contido nas mudanças que o servidor mostraria para aquela conta; id desconhecido é 400. No `reaccept`, tem de ser o diff inteiro: a tela mostra tudo, e aceite que diz ter mostrado menos não prova nada.
- `notice` com versão relevante pendente é 409: a faixa não serve de consentimento para o que bloqueia, nem vinda de um cliente velho ou adulterado.
- A vigente sai do português; o corpo e o hash, dessa versão na língua pedida, ou do português se ela faltar.
- `pending` e `accept` têm balde de rate limit próprio, como as trilhas. No balde do login, a pergunta de toda abertura gastava a entrada de quem está atrás de NAT, e um 429 pulava o bloqueio.
- `source` é `reaccept` (tela 02) ou `notice` (faixa). O cadastro grava `signup`, com o hash calculado pelo servidor.
- A hora é sempre a do servidor (`created_at`), nunca a do aparelho.
- Campos de texto do cliente com tamanho limitado (`client.app` até 32, `platform` de lista fechada) e corpo por `authBodyLimit`.

`legal_acceptances` ganha `body_sha256`, `from_version`, `shown_changes` (JSONB, array de strings, com `CHECK`), `app_version`, `platform` e `source`. Linhas antigas ficam com os campos nulos; as novas os exigem por `CHECK` quando `source` está preenchido. A tabela continua só de inserção.

**Versão na tela é o inteiro.** "v2 → v3", com a data de vigência de cada uma. O "1.4" do design era exemplo.

**`APP_PEDE_REACEITE` foi ligado em 2026-09-30, com a v3.** A trava existia para uma versão relevante não sair com app sem bloqueio instalado. O app não estava na loja nem no TestFlight, só no aparelho do dono pelo Xcode (ADR 0022), e o próximo build já sai da `main` com o bloqueio; não havia instalação que ficasse sem pedir.

## 3. Alternativas descartadas

- **Só destacar as seções mudadas (`?highlight`).** Sem texto novo, mas a pessoa teria de achar a mudança lendo o documento inteiro, e o design pede o diff.
- **Gerar o diff comparando os HTML.** Mudança de vírgula viraria item, e mudança de sentido numa frase reescrita não viraria nada que alguém entenda.
- **O cliente mandar a hora do aceite.** Relógio do aparelho não prova nada.
- **Confiar no `sha256` que o cliente manda sem conferir.** Gravaria como aceito um texto que o servidor nunca serviu.
- **Termos antes do sync.** Seguraria o XP de quem jogou offline atrás de uma decisão jurídica. O design escolheu sync primeiro; o risco (eventos subirem antes do novo aceite de uma política nova) fica para a revisão jurídica da v3.

## 4. Consequências e risco conhecido

- **O bloqueio mora no cliente.** `/sync`, `/tracks` e o resto não conferem o aceite; um app velho ou adulterado passa por cima. É o que `APP_PEDE_REACEITE` segura do lado da publicação.
- **O hash prova a versão na tela, não a leitura.** Os corpos são públicos; o que ele garante é que o texto aceito é o que o servidor serve.
- **A tela é do conteúdo.** Versão ou lista de mudanças nova (depois de um 409) monta a tela de novo, com a caixa desmarcada; a resposta de pendência de uma conta que já saiu é descartada; e o bloqueio da mesma conta só sai com resposta do servidor, nunca porque uma busca falhou.
- **Volta do segundo plano não confere.** A splash cobre a abertura a frio. Quem deixa o app aberto por dias só vê a v3 na próxima abertura.
- **Visitante não tem aceite a guardar.** O design pede que o visitante guarde a versão aceita no aparelho; hoje o visitante não aceita nada no app, e o cadastro já grava o aceite da vigente. Fica de fora até existir aceite de visitante.
- **App antigo nunca pede.** Por isso a trava do gerador; ligá-la é decisão de quando o app com o bloqueio estiver na loja.
- **A v2 foi publicada sem arquivo de mudanças.** O gerador ganha `--so-mudancas`, para gerar só as linhas de mudança de uma versão já publicada.
- **Uma frase de mudança é conteúdo jurídico.** Ela passa pela mesma revisão do texto dos documentos.
