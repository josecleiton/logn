# LogN - PRD: Seleção de trilha e trilhas pagas

Terceiro de três PRDs em sequência, depois de `logn_i18n_conteudo_spec.md` e
`logn_legal_spec.md`. Depende dos dois: a trilha paga sai nas três línguas, e a licença mora
nos termos de uso.

O design das telas está no projeto de design do LogN, nos arquivos `LogN Trilhas.dc.html`
(catálogo, paywall, fluxo de compra, estados da comprada) e `LogN Validade Offline.dc.html`
(a escada de dias sem contato). A seção 9 lista os estados; o design é a fonte de como
eles ficam.

## 1. Conceito

- **Trilha principal:** a de problem solving, com sete nós e portões de XP. É gratuita,
  para todos, com e sem rede, como já funciona hoje, com a semente no bundle e o retrato
  offline.
- **Trilhas pagas:** entram por uma tela de seleção. Cada uma é comprada à parte, e a compra
  fica atribuída à conta. A trilha comprada pode ser baixada e guardada no aparelho,
  cifrada.
- **O que se vende é uma licença de uso.** Ela pode ser revogada nas hipóteses fechadas da
  seção 7, escritas nos termos.

## 2. Decisões

| Tema | Decisão |
|---|---|
| Cobrança | Compra avulsa por trilha: produto único no Google Play e não consumível na App Store, com o mesmo id nas duas. Sem assinatura. |
| Autoria | Só o dono do app. `author` existe como campo da trilha, para não fechar a porta. |
| Plataforma | Android no lançamento, com Google Play; iOS quando houver a conta da Apple (ADR 0022). O direito de acesso fica na conta, no servidor, e vale nas duas lojas. |
| Quem compra | Só com conta. O visitante é levado ao cadastro na hora de comprar. |
| Compartilhamento Familiar | Desligado. A Apple não deixa desligar depois de ligado. |
| Amostra | O primeiro nó de cada trilha paga é grátis e vai sem cifra. |
| Atualizações | Sem custo para quem comprou. |
| Trilha descontinuada | Sai do catálogo, e quem comprou continua abrindo, online e offline. |
| Ameaça da cifra | Cópia casual. Extração com jailbreak fica fora do alcance, porque nenhum desenho offline a impede. |
| Validade offline | 30 dias sem contato com o servidor. Depois, a trilha pede para conectar. |
| Aparelhos | Sem limite. O servidor registra os aparelhos, sem bloquear, para ter evidência de compartilhamento. |
| XP | Um nível global. Os portões de cada trilha contam só o XP ganho nela. |
| Depois de revogar | O XP já ganho fica, o progresso fica guardado, e a trilha para de abrir. |
| Conta excluída e recriada | "Restaurar compras" liga a compra à conta nova. Uma conta ativa por transação. |

## 3. Pré-requisitos

- **PRD de conteúdo em várias línguas** entregue: a trilha paga nasce nas três línguas.
- **PRD jurídico** entregue, e uma versão nova dos termos, marcada como relevante, com as
  cláusulas da seção 7. A política de privacidade ganha versão nova junto: o registro de
  aparelhos (`identifierForVendor`) e as compras passam a ser dados coletados.
- **Keychain.** Já feito: o refresh token está no Keychain desde `5e787d8`. A chave de
  conteúdo da trilha paga vai para o mesmo lugar. Hoje `CoreWrapper.keychainKeys` é um
  conjunto de nomes exatos (`["refresh_token"]`), e precisa aceitar um prefixo
  (`track_key:`), com uma chave por trilha.
- **App Store Connect:** acordo de apps pagos assinado, e dados bancários e fiscais da
  empresa. Sem isso a loja não vende compra dentro do app.

## 4. Modelo de dados

Migração nova de schema, liberada pelo nome no `.gitignore`. Toda tabela tem
`created_at`; as que sofrem `UPDATE` têm `updated_at` com `trigger` (regra 5 do AGENTS.md).

```sql
CREATE TABLE tracks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(64) UNIQUE NOT NULL,
    kind        VARCHAR(8)  NOT NULL CHECK (kind IN ('free', 'paid')),
    status      VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'discontinued')),
    author      VARCHAR(255) NOT NULL,
    store_product_id VARCHAR(255) UNIQUE,  -- nulo na trilha gratuita; o mesmo nas duas lojas (0066)
    content_version INT NOT NULL DEFAULT 1,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((kind = 'paid') = (store_product_id IS NOT NULL))
);

-- Nome e descrição da trilha por língua, no desenho do PRD de conteúdo.
CREATE TABLE track_translations ( ... );  -- (track_id, locale) como chave

ALTER TABLE skill_nodes ADD COLUMN track_id UUID REFERENCES tracks(id);
-- A migração cria a trilha principal, aponta os nós atuais para ela e torna a coluna NOT NULL.
-- A posição na grade passa a ser única por trilha: sem isto, a segunda trilha colide com a primeira.
ALTER TABLE skill_nodes DROP CONSTRAINT skill_nodes_row_idx_col_idx_key,
                        ADD UNIQUE (track_id, row_idx, col_idx);

CREATE TABLE entitlements (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id     UUID NOT NULL REFERENCES tracks(id),
    original_transaction_id VARCHAR(64) NOT NULL,
    status       VARCHAR(16) NOT NULL CHECK (status IN ('active', 'revoked')),
    revoked_reason VARCHAR(32) CHECK (revoked_reason IN ('refund', 'fraud', 'redistribution', 'account_sharing')),
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, track_id)
);
-- Uma transação liga-se a uma conta ativa por vez.
CREATE UNIQUE INDEX entitlements_one_active_owner
    ON entitlements (original_transaction_id) WHERE status = 'active';

CREATE TABLE entitlement_devices (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id    UUID NOT NULL REFERENCES tracks(id),
    device_id   VARCHAR(64) NOT NULL,   -- identifierForVendor, nunca identificador de publicidade
    last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, track_id, device_id)
);

CREATE TABLE track_keys (
    track_id    UUID NOT NULL REFERENCES tracks(id),
    content_version INT NOT NULL,
    wrapped_key BYTEA NOT NULL,   -- a chave de conteúdo, cifrada com um segredo do servidor
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (track_id, content_version)
);
```

A exclusão de conta do PRD jurídico apaga `entitlements` e `entitlement_devices` em
cascata. A transação continua na Apple, e é por ela que a compra volta numa conta nova.

## 5. Compra, validação e restauração

1. O app compra pelo StoreKit 2 e recebe a transação assinada (JWS).
2. O app manda a transação para `POST /api/v1/purchases`. O servidor verifica a assinatura
   com a cadeia de certificados da Apple, confere produto, bundle e ambiente, e grava o
   direito de acesso. **O app nunca decide sozinho que comprou.**
3. O app finaliza a transação no StoreKit só depois de o servidor confirmar. **Se a rede cair após a cobrança**, o listener `Transaction.updates` do StoreKit 2 re-entregará a transação pendente na próxima vez que o app abrir, e o app tentará o `POST` novamente sem exigir ação do usuário.
4. **Restaurar compras:** o app lê as transações do Apple ID e manda cada uma para
   `POST /api/v1/purchases/restore`.
   - Transação sem dono, ou de uma conta excluída: liga-se à conta que pediu.
   - Transação de outra conta ativa: é recusada com `purchase_owned_by_other_account`,
     um código novo no formato de erro do PRD de conteúdo.
5. **Notificações da App Store** (App Store Server Notifications V2) chegam em
   `POST /api/v1/appstore/notifications`, com a assinatura verificada. `REFUND` e `REVOKE`
   revogam o direito de acesso com `revoked_reason = 'refund'`.

## 6. Download, cifra e uso offline

- `GET /api/v1/tracks/{id}/package` (autenticado, com direito ativo) devolve o conteúdo da
  trilha, nas três línguas, cifrado com AES-256-GCM pela chave de conteúdo daquela versão.
- `GET /api/v1/tracks/{id}/license` devolve a chave de conteúdo e o prazo offline
  (`valid_until`, 30 dias a partir de agora), e registra o aparelho em
  `entitlement_devices`. Todo pedido traz `X-Device-ID` (o `identifierForVendor`);
  sem ele, 400 `device_id_required`. O registro não bloqueia.
- `GET /api/v1/tracks` é o catálogo das trilhas pagas, com o produto da App Store de
  cada uma e, com token, quais a conta comprou. O app compra pelo produto que vem dali.
- Cada nó de `GET /api/v1/nodes` diz `requires_purchase`: o servidor decide o que é
  amostra, e o app não adivinha. Detalhe técnico na ADR 0013.
- **No aparelho:**
  - a chave vai para o Keychain, com acessibilidade *este aparelho só*, e não entra no
    backup;
  - o pacote cifrado vai para um arquivo no container do app, fora do `UserDefaults`, onde
    ficam a fila e o retrato;
  - o pacote pode ir para o backup, porque sem a chave ele não abre.
- **O Core confere a validade** com a hora que pede ao `crux_time`, como já faz na contagem
  do 429. Passado o `valid_until` sem contato, a trilha fica bloqueada até revalidar.
- **Com rede, cada abertura revalida.**
  - Licença ativa: renova o prazo.
  - Licença revogada: apaga a chave e o pacote.
  - Versão de conteúdo nova: baixa o pacote novo **em background, de forma invisível**. O progresso, que aponta para ids de
    desafio, continua valendo.
- **Amostra:** o primeiro nó da trilha paga vem aberto, sem cifra, nas rotas normais de
  conteúdo, e pode ir na semente do bundle.
- **Proteção:** contra cópia casual do arquivo. Quem controla o aparelho consegue extrair a
  chave, porque ela precisa estar lá para abrir sem rede. Fica fora do escopo tentar
  impedir isso.

## 7. Licença e revogação

Hipóteses fechadas, escritas nos termos:

| Hipótese | Como o servidor fica sabendo |
|---|---|
| Reembolso ou estorno | Notificação `REFUND` ou `REVOKE` da App Store, automática |
| Fraude na compra | Transação que não passa na verificação, ou marcada pela Apple |
| Redistribuição do conteúdo | Denúncia ou evidência de enunciados e gabaritos publicados fora do app |
| Compartilhamento de conta | Evidência no registro de aparelhos, considerando **apenas aparelhos ativos simultaneamente** (ex: janela de 7 dias), para não punir reinstalações ou troca de celular |

As duas últimas são revogação manual, com a evidência guardada e o motivo em
`revoked_reason`. Na primeira versão não há tela de administração: a revogação manual é um
runbook com SQL.

**Revogar sem motivo não é hipótese.** Uma trilha descontinuada continua aberta para quem
comprou (seção 2). O CDC trata como abusiva a cláusula que tira sem motivo o que a pessoa
pagou (art. 51).

**Depois de revogar:** o XP já ganho fica no nível global, o progresso fica guardado, e a
trilha para de abrir. Se o direito voltar, por recompra ou restauração, o progresso volta
junto.

## 8. XP, portões e sync

- **O nível é um só.** O XP de qualquer trilha soma no `global_xp`.
- **A ordem pedagógica continua obrigatória.** Comprar uma trilha destrava o paywall (o direito de jogar o Nó 2 em diante), mas **não pula os portões de XP**. O usuário ainda precisa acumular XP na trilha comprada para abrir os nós finais. O texto de venda precisa deixar claro que pagar não exime de resolver os problemas.
- **O portão de cada nó conta só o XP ganho na trilha do nó.** Hoje o Core compara
  `model.global_xp` com o `required_xp` do nó (`app.rs`, no `view`), e não guarda XP por nó
  nem por trilha. O `Model` passa a guardar XP por trilha, reconstruído a partir do
  `user_progress` que o `/progress` já devolve, e o portão lê esse número.
- **Sync de trilha sem direito ativo:** o evento entra na cadeia de hash, que não pode ter
  buraco, e não paga XP nem progresso. `ProcessEventXP` confere o direito antes de somar. A
  fila de quem jogou offline e teve a licença revogada no caminho continua andando.
- **O primeiro nó, a amostra, paga XP para todos**, com ou sem compra.

## 9. Estados a desenhar

- **Seleção de trilha:** a principal no topo, as pagas abaixo, com nome, ementa, número de
  nós, autor e preço.
- **Trilha paga não comprada:** o primeiro nó jogável, os outros bloqueados, e a chamada de
  compra.
- **Visitante tocando em comprar:** vai para o cadastro e volta para a compra.
- **Comprando:** o StoreKit aberto, com espera pelo servidor.
- **Comprada e baixando:** progresso do download.
- **Baixada:** abre sem rede.
- **Validade offline perto do fim:** aviso a partir de 3 dias antes.
- **Validade offline vencida:** a trilha pede para conectar.
- **Revogada:** a trilha some da lista de compradas, com o motivo em linguagem simples.
- **Gerenciamento de armazenamento:** aba no menu para o usuário ver o peso do que está baixado e poder apagar as trilhas do aparelho.
- **Descontinuada e comprada:** continua na lista de quem comprou, com o selo de
  descontinuada.
- **Restaurar compras:** em Ajustes, com o resultado da restauração.
- **Compra de outra conta:** a transação pertence a outra conta ativa.

Como o design resolve os estados, e o que ficou decidido fora dele:

- **Seleção de trilha é o catálogo em grade**, um balão por trilha na cor dela, aberto pelo
  nome da trilha com chevron no cabeçalho da árvore. Aqui o app diverge do design, que tinha
  um botão "Trilhas" à parte: o nome cortava em tela pequena. O selo de validade fica colado
  ao chevron. A árvore mostra uma trilha por vez.
- **Paywall em três lugares**, mesmo produto e mesmo preço: o detalhe da trilha, o nó
  fechado tocado na árvore e o relatório do fim da amostra. "Agora não" na oferta do fim
  da amostra vale pela sessão.
- **Validade offline:** silêncio até 26 dias sem contato; de 27 a 30, selo junto do chevron
  (o de "N dias" some no dia em que o catálogo é aberto), card âmbar e bloco no detalhe;
  a partir de 31, só essa trilha fecha, numa tela própria.
- **Comprada e baixando:** a trilha abre quando o pacote termina de baixar; não há jogar
  durante o download, porque o pacote é um só.
- **Compra de outra conta:** a mensagem é genérica. O servidor não revela nada da outra
  conta, nem o e-mail mascarado.
- **Onboarding:** uma tela só, "Por onde começar?", na primeira abertura, sem preço.
- **Nivelador (nó zero):** está no design e fica para um PR próprio, com PRD.

## 10. Fora do escopo

- Assinatura.
- Compartilhamento Familiar.
- Autores de fora e repasse de receita.
- Limite de aparelhos.
- Tela de administração para revogação.
- Impedir extração com jailbreak.
- Preço: é campo em aberto, definido por trilha no App Store Connect.

## 11. Critérios de aceite

1. Uma trilha paga não comprada abre o primeiro nó, com XP, e bloqueia os outros.
2. Comprar e confirmar no servidor libera a trilha no mesmo aparelho e, depois do login, em
   outro.
3. Uma transação forjada ou de outro bundle não gera direito de acesso.
4. Com o aparelho em modo avião, a trilha comprada abre por 30 dias e bloqueia no 31º.
5. Um reembolso na App Store revoga o direito sozinho. Na abertura seguinte com rede, a
   chave e o pacote somem do aparelho, e o XP já ganho continua no nível.
6. Excluir a conta, criar outra e restaurar compras devolve a trilha. Restaurar a mesma
   compra numa segunda conta ativa é recusado.
7. O XP de uma trilha paga não destrava nó da trilha principal, e vice-versa. O nível
   global soma os dois.
8. Um evento de trilha sem direito ativo entra na cadeia e não muda XP nem progresso.
9. O arquivo do pacote copiado para outro aparelho não abre.
10. Uma trilha descontinuada some do catálogo de quem não comprou e continua abrindo para
    quem comprou.

## 12. Riscos

- **O banco ainda não tem backup.** Perder o banco apaga o registro das licenças. As
  compras voltam pelo "Restaurar compras", porque a transação está na Apple. Contas e
  progresso não voltam. Foi considerado, e a decisão é seguir assim por ora.
- **Compartilhamento de conta sem limite de aparelhos.** A revogação por esse motivo depende
  de evidência forte. Revogar com evidência fraca é o que o CDC chama de abusivo.
- Risco sobre a natureza jurídica da entidade: `logn-conteudo/legal/entidade.md`.
- **O pacote cifrado é tão seguro quanto o aparelho.** A proteção é contra cópia casual, e o
  texto não pode prometer mais que isso.
