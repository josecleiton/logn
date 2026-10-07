# LogN - PRD: Instituição de ensino no perfil

A fonte de telas é o projeto "LogN Instituicao" no Claude Design. O pedido ao designer
foi pôr a instituição no Perfil "para quando a gente tiver leaderboard e contests
rodando". Nenhum dos dois existe: a aba Placar mostra dados de exemplo, com o aviso de
`standings_are_sample`, e a aba Sede diz "UFC" para todo mundo (`mock_data.rs`). Esta
entrega não mexe nela.

Este PRD entrega a escolha da instituição, no Perfil e no cadastro, e a verificação do
vínculo por código no e-mail institucional. O que depende do placar e do contest fica
na seção 10, para o modelo de dados não fechar a porta.

## 1. Conceito

- **Vínculo:** a conta escolhe uma instituição da lista do e-MEC, mantida no servidor.
  A sigla aparece no topo do Perfil, antes do e-mail, e a instituição ganha uma seção
  abaixo dos números.
- **Verificação:** um código de 6 dígitos no e-mail institucional confirma que o
  endereço é da pessoa. O e-mail fica guardado cifrado e à parte, e não vira login.
- **Curadoria:** a lista vem de dado público e erra: sigla faltando, nome em caixa,
  domínio desconhecido. O jogador corrige ou sugere por um fluxo só, revisado por
  runbook.
- **Para que serve agora:** identidade no Perfil, e saber de onde vêm os jogadores. É o
  dado que decide onde o placar e o primeiro contest começam.
- **Para que serve depois:** chave do placar por instituição e das inscrições em
  contest, onde só o vínculo verificado conta.

## 2. Decisões

| Tema | Decisão |
|---|---|
| Lista | e-MEC, todas as instituições ativas, de todas as categorias e tipos (seção 4.1). Texto livre não vira instituição. |
| Crédito | "Fonte: e-MEC/MEC" na página Sobre e nos documentos legais. |
| Escopo geográfico | A lista tem `country` e nasce só com o Brasil. Quando o país da conta não tem lista, a seção do Perfil e o passo do cadastro não aparecem. |
| Onde se escolhe | No Perfil, e num passo pulável do cadastro, logo depois do código de login. No cadastro só se escolhe; verificar fica para o Perfil. |
| Quem escolhe | Só conta. O visitante vê o card como chamada para criar conta. |
| Sugestão | Quando o domínio do e-mail de login é de uma instituição da lista, ela vem sugerida e selecionada. |
| Sem sigla | O topo do Perfil não mostra nada antes do e-mail; o nome inteiro aparece no card e na lista. Sigla nunca é inventada. |
| Cidade | É a da sede, a única que o e-MEC traz. A linha diz "Sede · Cidade · UF". |
| Verificação | Código de 6 dígitos no e-mail institucional. Vale o domínio da instituição ou um subdomínio dele. |
| Domínio desconhecido | O servidor não manda código: registra um pedido de domínio, com o e-mail cifrado. Aprovado pelo runbook, o card avisa na próxima abertura, e um toque manda o código. |
| Login | Não muda. O e-mail institucional é guardado à parte. |
| E-mail de login do domínio | Vem preenchido na tela do código, e o código é mandado mesmo assim: o cadastro provou o e-mail na data do cadastro, não agora. |
| Guarda do e-mail | HMAC para a unicidade, AES-256-GCM para mandar código, domínio em claro para o selo. |
| Um e-mail, uma conta | Um e-mail institucional prova uma conta só. A recusa acontece no confirmar, depois de o código provar a posse, para ninguém descobrir se um endereço tem conta. |
| Revalidação | Anual, contada da confirmação. Trinta dias antes, o card avisa, e um toque manda o código ao endereço guardado. Vencida volta a não verificada, sem perder XP. Sem job. |
| Troca | Livre. Trocar ou remover apaga o e-mail cifrado, o HMAC e o domínio, e volta a não verificada. Toda troca e remoção vai para o histórico. |
| Curadoria | Um fluxo só, "Corrigir ou sugerir", com tipo: instituição nova, sigla, nome, domínio. Uma tabela e um runbook com SQL, sem painel. |
| Nome da instituição | Conteúdo: vem do servidor, sem tradução, normalizado na importação (seção 4.1). |
| Offline | O vínculo entra no retrato offline. Escolher, trocar, remover e verificar exigem rede. |
| Cadeia de hash | Não entra. Vínculo não é evento de jogo. |
| Telemetria | Só `institution_step_skipped` e `institution_step_chosen` no cadastro, sem dizer qual instituição. Qual foi escolhida sai do banco. |
| Métricas "em breve" do card | Fora. "Na instituição" e "próximo contest" voltam com o placar. |
| Cor do selo e do banner | `accent` ou `info`, não verde. O verde do design fere a regra do design system: verde só para resultado de resposta, célula de placar ou ação destrutiva. |
| Contagem de reenvio | Aparece logo depois do envio (o "reenviar em 0:42" do design), pelo mesmo `resend_cooldown`. Hoje ela só aparece depois de um 429. |
| Textos | Sem promessa de placar e sem "estuda": o código prova que o endereço é da pessoa, não matrícula. Professor e ex-aluno passam igual. |

## 3. Pré-requisitos

- **Tela do passo no cadastro:** o projeto no Claude Design não tem. Pedir ao designer
  antes de codar esse passo.
- **Política de privacidade, versão nova:** entram como dados coletados a instituição
  escolhida, o e-mail institucional (cifrado) e a data da confirmação, e o crédito ao
  e-MEC. O texto vive em `logn-conteudo/legal/` e sai por `gen_documentos_legais.py`.
  A versão sai **sem** `material`: o dado só existe para quem escolhe. **Não passou por
  revisão jurídica**, como a v1 dos termos (ver `docs/ROADMAP.md`); a dívida é a mesma.
- **Segredo novo:** `INSTITUTION_EMAIL_KEY` (32 bytes) no Secret Manager do Cloud Run,
  separado do segredo do HMAC de OTP. Sem ele o backend não sobe. Rotação de chave fica
  fora desta entrega.
- **Número da migração:** as de schema param na `0043`; 0039, 0040 e 0042 são
  migrações de conteúdo, ignoradas pelo `.gitignore`. A nova é a 0044 ou a próxima livre
  depois do que `logn-conteudo` já gerou.

## 4. Modelo de dados

### 4.1 A lista

Fonte: `https://dadosabertos.mec.gov.br/images/conteudo/Ind-ensino-superior/2022/PDA_Lista_Instituicoes_Ensino_Superior_do_Brasil_EMEC.csv`,
copiado em 24/09/2026. O caminho diz 2022: instituições credenciadas depois podem
faltar, e quem não achar a sua usa "Corrigir ou sugerir". O arquivo não traz licença.

O que o arquivo tem, lido em 24/09/2026:

| Fato | Número |
|---|---|
| Linhas | 4.328, sem código repetido |
| Ativas | 3.117: 3.111 "Ativa" e 6 "Em atividade". 1.211 extintas ficam fora |
| Por categoria | 2.747 privadas, 364 públicas |
| Sem sigla | 575: 241 vazias e 332 com o texto `NULL`. Só 10 públicas |
| Sigla repetida | 210 siglas entre ativas; `FAP` aparece 17 vezes |
| Sigla mais longa | 20 caracteres |
| Nome em caixa alta | 2.129 (68%) |
| Domínio ou site | Não vem |

**Importação**, por um script em `tools/` que gera a migração de dados:
- ativa é "Ativa" ou "Em atividade";
- `NULL` e vazio viram sigla ausente;
- o nome vai para caixa de título em português ("da", "de", "do", "das", "dos", "e" em
  minúsculas), e `display_name` guarda a correção manual quando a regra erra;
- `emec_code` é o `CODIGO_DA_IES`, chave da reimportação;
- os domínios começam vazios, salvo os curados à mão.

### 4.2 Schema

Uma migração nova de schema, liberada pelo nome no `.gitignore`. A lista vai numa
segunda migração, também pública: é dado de referência, não currículo. Toda tabela tem
`created_at`; as que sofrem `UPDATE` têm `updated_at` com o trigger de `set_updated_at()`
(regra 5 do AGENTS.md).

```sql
CREATE TABLE institutions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    emec_code    INT UNIQUE,                    -- CODIGO_DA_IES; nulo fora do Brasil
    acronym      VARCHAR(32),                   -- nulo quando o e-MEC não traz sigla
    name         VARCHAR(255) NOT NULL,         -- normalizado na importação
    display_name VARCHAR(255),                  -- correção manual; vence `name` quando existe
    city         VARCHAR(128) NOT NULL,         -- cidade da sede
    region       VARCHAR(8)   NOT NULL,         -- UF no Brasil
    country      CHAR(2)      NOT NULL CHECK (country ~ '^[A-Z]{2}$'),
    status       VARCHAR(16)  NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'retired')),
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER trg_institutions_updated_at BEFORE UPDATE ON institutions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX idx_institutions_country ON institutions (country) WHERE status = 'active';

-- Domínio-base. Vale o domínio igual ou um subdomínio dele (aluno.ufba.br).
-- Sem UNIQUE em domain: duas instituições podem dividir um domínio.
-- Domínio público (gmail.com, edu.br...) nunca entra; a lista de proibidos fica em Go.
CREATE TABLE institution_email_domains (
    institution_id UUID NOT NULL REFERENCES institutions(id) ON DELETE CASCADE,
    domain         VARCHAR(253) NOT NULL
        CHECK (domain = lower(domain) AND domain ~ '^[a-z0-9-]+(\.[a-z0-9-]+)+$'),
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (institution_id, domain)
);

-- Um vínculo por conta. Sai em cascata com a conta.
-- O e-mail institucional nunca fica em claro: HMAC para a unicidade, AES-256-GCM
-- (nonce junto do texto cifrado) para mandar o código da revalidação.
CREATE TABLE user_institutions (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    institution_id      UUID NOT NULL REFERENCES institutions(id),
    verified_email_hmac VARCHAR(64),
    verified_email_enc  BYTEA,
    verified_domain     VARCHAR(253),
    verified_at         TIMESTAMP WITH TIME ZONE,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_user_institutions_verified CHECK (
        (verified_at IS NULL) = (verified_email_hmac IS NULL)
        AND (verified_at IS NULL) = (verified_email_enc IS NULL)
        AND (verified_at IS NULL) = (verified_domain IS NULL))
);
CREATE TRIGGER trg_user_institutions_updated_at BEFORE UPDATE ON user_institutions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE UNIQUE INDEX user_institutions_one_account_per_email
    ON user_institutions (verified_email_hmac) WHERE verified_email_hmac IS NOT NULL;

-- Histórico de escolhas e remoções. Só inserção: fica com created_at.
-- Guarda instituição e data, nunca e-mail. É o que uma carência de troca vai ler.
CREATE TABLE user_institution_changes (
    id             BIGSERIAL PRIMARY KEY,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    institution_id UUID REFERENCES institutions(id),   -- NULL = removeu
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_user_institution_changes_user ON user_institution_changes (user_id, created_at);

-- Código em andamento: por conta, não por e-mail. A tabela `otps` é por (email, purpose):
-- duas contas pedindo código para o mesmo endereço sobrescreveriam uma à outra, e o
-- expurgo, que apaga `otps` pelo e-mail de login, deixaria este para trás.
CREATE TABLE institution_verifications (
    user_id           UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    institution_id    UUID NOT NULL REFERENCES institutions(id),
    email_enc         BYTEA NOT NULL,
    code_hash         VARCHAR(64) NOT NULL,
    attempts          INT NOT NULL DEFAULT 0,
    expires_at        TIMESTAMP WITH TIME ZONE NOT NULL,
    sent_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sends_in_window   INT NOT NULL DEFAULT 1,
    window_started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER trg_institution_verifications_updated_at BEFORE UPDATE ON institution_verifications
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- "Corrigir ou sugerir": instituição nova, sigla, nome ou domínio.
-- O pedido de domínio nasce também da verificação, com o e-mail cifrado, para o
-- código sair num toque depois da aprovação.
CREATE TABLE institution_requests (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind           VARCHAR(16) NOT NULL CHECK (kind IN ('new', 'acronym', 'name', 'domain')),
    institution_id UUID REFERENCES institutions(id),   -- nulo só em 'new'
    value          VARCHAR(255) NOT NULL,              -- nome, sigla ou domínio proposto
    city           VARCHAR(128),                       -- só em 'new'
    email_enc      BYTEA,                              -- só em 'domain' vindo da verificação
    status         VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'accepted', 'rejected')),
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_institution_requests_target CHECK ((kind = 'new') = (institution_id IS NULL))
);
CREATE TRIGGER trg_institution_requests_updated_at BEFORE UPDATE ON institution_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

Trocar ou remover a instituição limpa as quatro colunas de verificação no mesmo UPDATE e
apaga a linha de `institution_verifications`. O e-mail antigo não fica em lugar nenhum.
Um pedido de domínio aceito ou recusado tem `email_enc` apagado quando a pessoa verifica
ou quando é recusado. **Expirada não é gravada:** é `verified_at` mais antigo que um
ano, calculado na leitura. "Renovar em breve" é faltar 30 dias ou menos.

**Exclusão de conta:** `user_institutions`, `user_institution_changes`,
`institution_verifications` e `institution_requests` saem em cascata com `users`. O
comentário de `purgeAccount` passa a listá-las.

## 5. API

Todas autenticadas e sob o limite por IP (`auth(...)` em `main.go`). Erro é
`writeError(w, status, code)`.

| Rota | O que faz |
|---|---|
| `GET /api/v1/institutions` | Lista as ativas do país da conta, com `suggested_id` pelo domínio do e-mail de login. País sem lista responde `items: []` |
| `PUT /api/v1/users/me/institution` | `{institution_id}`. Grava o vínculo e o histórico numa transação, e limpa a verificação anterior |
| `DELETE /api/v1/users/me/institution` | Remove o vínculo e registra no histórico |
| `POST /api/v1/users/me/institution/verification` | `{email}`, ou `{}` para usar o endereço guardado (revalidação, domínio aprovado). Domínio conhecido: manda o código. Desconhecido: registra o pedido de domínio e responde `202` com `domain_pending` |
| `POST /api/v1/users/me/institution/verification/confirm` | `{code}`. Confere e grava HMAC, cifra, domínio e `verified_at` |
| `POST /api/v1/institutions/requests` | `{kind, institution_id?, value, city?}`. Até 3 em aberto por conta |
| `GET /api/v1/progress` | Ganha `institution` (objeto ou `null`), para o Perfil vir na mesma busca da abertura |

Resposta da lista:
`{"items":[{"id","acronym","name","city","region"}],"suggested_id":"…"|null}`, com `acronym`
nulo quando não há e `name` já resolvido (`display_name` quando existe). A lista vem
inteira, cerca de 3.100 itens, com gzip (`gzip.go`), e o Core filtra: a busca fica
instantânea e sem resposta velha chegando fora de ordem. Se o tamanho pesar na abertura
da tela, a busca vai para o servidor.

`institution` em `/progress`:
`{"id","acronym","name","city","region","status","verified_domain","renew_soon","domain_pending"}`.
`status` é `unverified`, `verified` ou `expired`. `domain_pending` é `pending`,
`approved` ou vazio.

O código reaproveita o `otpHash` com HMAC, com propósito `verify_institution` e o
`user_id` no HMAC. Validade, tentativas e intervalo seguem os OTPs de hoje
(`OTPMaxAttempts`, `OTPResendCooldown`), mais um teto de 5 envios em 24 h por conta.

**Códigos novos em `backend/internal/httpapi/api_errors.go`:**

| Código | Status | Quando |
|---|---|---|
| `institution_not_found` | 404 | `institution_id` inexistente ou aposentado |
| `institution_not_chosen` | 409 | Pediu código sem vínculo |
| `institution_email_domain_mismatch` | 422 | O domínio é conhecido e é de outra instituição, ou é público |
| `institution_email_taken` | 409 | O e-mail já prova outra conta. Só no confirmar |
| `institution_no_stored_email` | 409 | Pediu `{}` sem endereço guardado |
| `request_limit_reached` | 409 | Três pedidos em aberto |

Reusados: `otp_invalid`, mas com **422**, nunca 401. O Core trata qualquer 401 numa rota
autenticada como token vencido e dispara `AttemptRefresh`. `otp_resend_too_soon` (429,
com `Retry-After`) e `rate_limited` (429) para o teto diário e o limite por IP.

**E-mail:** template `institution_otp.html` e `Mailer.SendInstitutionOTP`, com o botão
saído de `appLink` (`https://logn.sh/app/verify#…&purpose=verify_institution`, ADR 0028)
e levado por `handleIncomingURL`. O `mailer.go`
só manda em pt-BR; o código institucional herda essa dívida (seção 12).

## 6. Core

Tipos novos em `domain.rs`, todos com `#[derive(Facet)]` e `#[facet(fg::namespace = "LogN")]`:

- `InstitutionRow { id, acronym: String, name, city, region }`, com `acronym` vazio
  quando não há.
- `InstitutionStatus` (`#[repr(u8)]`): `Unverified`, `Verified`, `Expired`.
- `DomainRequestState` (`#[repr(u8)]`): `None`, `Pending`, `Approved`.
- `InstitutionLink { institution, status, verified_domain, renew_soon, domain_request }`,
  com `#[serde(default)]` nos campos que não são a instituição.

Variante nova só no fim de qualquer enum (a FFI numera variante por posição).

**Model:**
- `institution: Option<InstitutionLink>`
- `institution_catalog: Vec<InstitutionRow>`, `institution_available: bool` (lista não vazia)
- `institution_suggested_id`, `institution_query`, `institution_selected_id`
- `institution_email` e o estado de envio e conferência do código
- `institution_loading` e `institution_saving`
- `registration_institution_step: bool`

**Eventos:**
- `OpenInstitutionPicker` busca a lista, e `InstitutionsFetched(HttpResult)`.
- `SetInstitutionQuery { query }` filtra localmente.
- `SelectInstitution { id }`
- `SaveInstitution` e `InstitutionSaved(HttpResult)`
- `RemoveInstitution` e `InstitutionRemoved(HttpResult)`
- `RequestInstitutionCode { email }` (vazio = endereço guardado) e
  `InstitutionCodeRequested(HttpResult)`
- `ConfirmInstitutionCode { code }` e `InstitutionCodeConfirmed(HttpResult)`
- `SendInstitutionRequest { kind, value, city }` e `InstitutionRequestSent(HttpResult)`
- `SkipRegistrationInstitution`
- `CloseInstitutionPicker`

**Regras:**
- A busca compara sigla, nome e cidade em minúsculas e sem acento. A dobra de acentos é
  uma função pura em Rust, testada.
- A sugerida vem primeiro e já selecionada.
- Sem lista para o país, o ViewModel não tem seção nem passo de cadastro.
- No cadastro, o passo aparece depois do código de login, quando há lista. Escolher ou
  pular emite a telemetria e segue o fluxo; não leva à tela do código.
- No Perfil, depois de salvar uma instituição, o Core leva à tela do código, com o
  e-mail de login preenchido quando ele é do domínio.
- `202 domain_pending` vira o estado `Pending`, sem tela de código.
- `ProgressFetched` lê `institution` com `#[serde(default)]` e grava o retrato.
- `OfflineSnapshot` ganha `#[serde(default)] institution: Option<InstitutionLink>`.
- `LogoutSnapshot` ganha o vínculo: o desfazer da saída devolve, e `TokenCleared` limpa.
- 401 em rota nova segue o interceptor (`pending_retry_event`), como `FetchProgress`.
- 429 usa `rate_limited(...)` e alimenta `resend_cooldown`; o envio bem-sucedido também
  arma a contagem.

**ViewModel:**
- `institution: Option<InstitutionLink>`, `institution_available: bool`
- `institution_picker: InstitutionPickerView { query, results, total, suggested_id, selected_id, is_loading, is_saving }`
- o estado da tela do código: e-mail, contagem de reenvio, enviando e conferindo.

**StatusKey novos, no fim do enum:** `InstitutionsUnavailable`, `InstitutionSaveFailed`,
`InstitutionRequestSent`, `InstitutionRequestLimit`, `InstitutionEmailMismatch`,
`InstitutionEmailTaken`, `InstitutionDomainPending`. Reusa `SendingCode`,
`CheckingCode`, `CodeInvalid` e `RateLimited`.

**Telemetria:** `institution_step_skipped` e `institution_step_chosen`, sem propriedade
de instituição, pelo caminho de telemetria que já existe (ADR 0001, PostHog).

## 7. iOS

- **`ProfileHubView`:**
  - sigla antes do e-mail em `identity`, só quando há sigla;
  - seção `INSTITUIÇÃO` depois de `summary`, só quando `institution_available`. Card
    tracejado (`lineDim`, como a dropzone vazia) quando não há vínculo; card com sigla,
    nome, selo e chevron quando há;
  - quatro avisos: **não verificada** (também quando o domínio acabou de ser aprovado),
    com "Mandar código"; **em análise**, sem botão; **renovar**, nos 30 dias antes, com
    "Renovar"; **expirada**, com "Mandar código";
  - visitante vê o card como chamada: mesmo caminho do `conversionCard`.
- **`InstitutionPickerView`:** tela empilhada no `NavigationStack` do perfil, com detent
  `.large`, como Gerenciar conta. Campo de busca, contador, lista com "Sede · Cidade ·
  UF", banner do domínio, botão "Continuar com SIGLA" (ou com o nome, sem sigla), link
  "Corrigir ou sugerir" e "Remover instituição" quando há vínculo.
- **`InstitutionVerifyView`:** e-mail institucional, código de 6 dígitos com
  `OTPInputView` e contagem de reenvio. A `OTPInputView` hoje despacha `.verifyOtp`
  direto, e ganha um fecho `onSubmit` para despachar `.confirmInstitutionCode`.
- **`InstitutionRequestView`:** tipo, valor e cidade (em "instituição nova"), e enviar.
- **Passo do cadastro:** tela do designer (seção 3), reusando a lista do seletor, com
  "Pular".
- **Sigla longa:** a caixa da sigla corta visualmente; o `accessibilityLabel` leva o nome
  inteiro.
- **Altura do sheet:** `ProfileHubView` usa `.presentationDetents([.height(contentHeight)])`
  sem ScrollView. Conferir no iPhone menor e, se passar da tela, pôr o conteúdo num
  `ScrollView` com detent máximo.
- **DEBUG:** `-LogNStartScreen instituicao` abre o perfil já na tela de escolha.

## 8. i18n

Grupo novo `[groups.institution.keys]` em `i18n/keys.toml`, com texto em `pt-BR.toml`,
`en.toml` **e** `es.toml` (o validador exige as três). `just i18n` gera `Str.Institution.*`.

**Duas regras de texto:**
- **Vínculo é ter e-mail da instituição.** Os textos dizem "sua instituição", nunca
  "onde você estuda": o código não prova matrícula, e professor e ex-aluno passam igual.
  O PRD do placar pode restringir quem conta sem mudar texto.
- **A sigla nunca vem depois de preposição.** O placeholder não sabe o gênero (a UFBA,
  o IFBA, o CEFET), e artigo antes de nome próprio fica fora de qualquer jeito. A sigla
  aparece sozinha (título, selo, botão), e o corpo diz "e-mail institucional".

| Chave | pt-BR |
|---|---|
| `section_title` | INSTITUIÇÃO |
| `add_title` | Adicionar instituição |
| `add_sub` | Sua sigla aparece no seu perfil. |
| `guest_cta` | Com uma conta, sua sigla aparece no seu perfil. |
| `picker_title` | Sua instituição |
| `search_prompt` | sigla, nome ou cidade |
| `results_count` | plural: 1 RESULTADO / %{count} RESULTADOS |
| `suggested` | SUGERIDA |
| `seat_city_region` | Sede · %{city} · %{region} |
| `domain_banner` | Você pode confirmar com seu e-mail @%{domain} no próximo passo. |
| `continue_with` | Continuar com %{label} |
| `remove` | Remover instituição |
| `verified_badge` | VERIFICADA · %{domain} |
| `unverified_notice` | Não verificada. Confirme com um código no seu e-mail institucional. |
| `send_code` | Mandar código |
| `domain_pending_notice` | Estamos conferindo o domínio @%{domain}. Quando der para confirmar, avisamos aqui. |
| `renew_soon_notice` | plural: Sua verificação vence em 1 dia. / Sua verificação vence em %{days} dias. |
| `renew` | Renovar |
| `expired_notice` | A verificação venceu. Confirme de novo para voltar a ter o selo. |
| `confirm_title` | Confirme o vínculo |
| `confirm_body` | Mandamos um código para o seu e-mail institucional. Ele não vira seu login — só confirma que o endereço é seu. |
| `email_prompt` | E-MAIL INSTITUCIONAL |
| `confirm` | Confirmar |
| `request_title` | Corrigir ou sugerir |
| `request_kind_new` / `_acronym` / `_name` / `_domain` | Instituição que falta · Sigla · Nome · Domínio de e-mail |
| `request_value_new` / `_acronym` / `_name` / `_domain` | Nome da instituição · Sigla correta · Nome correto · Domínio (ex.: ufba.br) |
| `request_city` | Cidade |
| `request_send` | Enviar |
| `registration_title` | Sua instituição |
| `registration_sub` | Opcional. Sua sigla aparece no seu perfil. |
| `registration_skip` | Pular |
| `card_accessibility` | Instituição: %{name}. %{state}. |
| `row_accessibility` | %{name}, sede em %{city}, %{region} |
| `row_selected_accessibility` | %{name}, sede em %{city}, %{region}, selecionada |

**`status.*`**, uma chave por `StatusKey` novo:

| StatusKey | pt-BR |
|---|---|
| `InstitutionsUnavailable` | Não deu para carregar a lista. Tente de novo. |
| `InstitutionSaveFailed` | Não deu para salvar. Tente de novo. |
| `InstitutionEmailMismatch` | Esse e-mail não é de um domínio desta instituição. |
| `InstitutionEmailTaken` | Esse e-mail já confirma outra conta. |
| `InstitutionDomainPending` | Ainda não conhecemos esse domínio. Vamos conferir e avisar aqui. |
| `InstitutionRequestSent` | Recebemos. Obrigado por ajudar a lista. |
| `InstitutionRequestLimit` | Você já tem 3 pedidos em análise. Espere a resposta de algum. |

`%{label}` é a sigla, ou o nome quando não há. `%{state}` é o aviso do estado. Nome,
sigla e cidade da instituição não entram no catálogo: são conteúdo. en e es saem na
implementação, a partir deste pt-BR; o espanhol passa pelo revisor técnico nativo que
já está pendente no roadmap.

## 9. Estados que o design cobre

| Tela do design | Esta entrega |
|---|---|
| 01 Sem instituição (card tracejado) | sim, com o texto novo |
| 02 Seletor com busca, contador, sugerida, banner e check | sim |
| 03 Com instituição: sigla no topo, card, selo, aviso de não verificada | sim, sem as métricas |
| 04 Confirmar vínculo (código) | sim, com o texto novo |
| Visitante | card vira chamada para conta |

**Fora do design, precisam de tela:**
- passo do cadastro (pedido ao designer);
- instituição sem sigla no topo e no card;
- expirada, renovar em breve, domínio em análise, domínio aprovado;
- erro de rede ao carregar a lista;
- remover;
- e-mail de outro domínio, e e-mail já usado por outra conta;
- "Corrigir ou sugerir", pedido enviado e limite de pedidos.

## 10. Fora de escopo

- Placar por instituição, inscrição em contest, e a sigla em linha de placar de verdade.
- A aba Sede com dados de exemplo continua como está.
- Carência de troca (90 dias, e nunca com contest em andamento): entra com o contest,
  lendo `user_institution_changes`.
- Métricas do card (posição na instituição, próximo contest).
- Painel de administração: curadoria é runbook com SQL.
- Cidades de campus. O e-MEC traz só a sede.
- Provar matrícula. O código prova acesso ao e-mail: muita instituição mantém o e-mail
  de ex-aluno, então a revalidação só derruba quem perdeu o endereço.
- Aviso de vencimento por e-mail. Só no app, sem job.
- Rotação de `INSTITUTION_EMAIL_KEY`.
- E-mail do código em en e es.
- Exportação de dados. A rota do PRD jurídico ainda não existe; quando existir, leva o
  vínculo e o histórico.

## 11. Critérios de aceite

1. Uma conta com e-mail `@ufba.br` abre a escolha com UFBA sugerida e selecionada.
2. Escolher UFBA mostra "UFBA" antes do e-mail no Perfil, e continua mostrando com o
   aparelho em modo avião depois de fechar e abrir o app.
3. Uma instituição sem sigla não mostra nada antes do e-mail, e o card mostra o nome.
4. `user_institution_changes` ganha uma linha por escolha, troca e remoção.
5. Buscar "feira" acha UEFS pela cidade da sede, e "ufrb" acha pela sigla, sem diferença
   de acento.
6. Nenhuma instituição extinta aparece; as "Em atividade" aparecem; nenhuma sigla é
   `NULL`.
7. `PUT` com id inexistente responde 404 `institution_not_found`.
8. Pedir código para `@gmail.com` responde 422 `institution_email_domain_mismatch`;
   `@aluno.ufba.br` é aceito com UFBA.
9. Pedir código com domínio desconhecido responde 202 `domain_pending`, cria o pedido
   com o e-mail cifrado e não manda e-mail. Aceito o pedido, o card avisa, e `{}` manda
   o código.
10. Código certo grava HMAC, cifra, domínio e `verified_at`; nenhuma coluna tem o e-mail
    em claro.
11. Código errado responde 422 `otp_invalid` e **não** derruba a sessão.
12. Um e-mail confirmado numa conta recusa a segunda com `institution_email_taken`.
13. Trocar de instituição apaga as quatro colunas de verificação e a verificação pendente.
14. Com `verified_at` de 340 dias, `/progress` responde `renew_soon: true`, e `{}`
    manda o código ao endereço guardado. Com mais de um ano, `status: expired`.
15. O quarto pedido em aberto responde 409 `request_limit_reached`.
16. Conta de país sem lista não vê a seção nem o passo do cadastro.
17. Pular e escolher no cadastro emitem os dois eventos, sem a instituição.
18. O visitante não vê lista: o card leva ao cadastro.
19. No dia 31 depois da exclusão não sobra linha da conta nas tabelas novas ligadas a ela.
20. Nenhum texto das telas novas é literal no código (`just i18n-check`).

## 12. Riscos

- **Lista velha:** a cópia é de 2022. Instituição nova falta até a reimportação ou um
  pedido. O runbook diz como reimportar por `emec_code`.
- **Siglas repetidas:** 17 "FAP" na mesma busca. A linha "Sede · Cidade · UF" separa, mas
  a sigla no topo do Perfil pode confundir quem vê.
- **Normalização de nome erra:** siglas dentro do nome ("SENAC") viram "Senac".
  `display_name` e "Corrigir ou sugerir" consertam.
- **Poucos domínios:** quase toda instituição começa sem domínio, e a primeira pessoa de
  cada uma espera a curadoria. A fila precisa andar.
- **Domínio errado aprovado:** aceita e-mail de fora. Domínio só entra com fonte
  conferida.
- **Relé de e-mail:** uma conta pode fazer o LogN mandar código a endereços de um
  domínio aceito. O domínio restringe o alvo; o teto de 5 por dia por conta e o limite
  por IP seguram o volume.
- **Chave de cifra:** perder `INSTITUTION_EMAIL_KEY` torna os endereços guardados
  ilegíveis, e a revalidação cai para "digite o e-mail de novo". Vazar a chave junto com
  o banco expõe os endereços.
- **Verificação sem uso:** o selo existe antes do placar e vence em um ano. Se o PRD do
  placar decidir que professor ou ex-aluno não conta, a verificação por e-mail não
  distingue nenhum deles.
- **Política sem revisão jurídica:** mesma dívida da v1 dos termos.
- **E-mail em português:** o `mailer.go` só manda em pt-BR.
- **O banco ainda não tem backup:** perder o banco apaga os vínculos, como apaga as contas.

## 13. Perguntas abertas

Nenhuma. Quem conta no placar (aluno, professor, ex-aluno) é decisão do PRD do placar;
esta entrega fixou só o que o texto afirma (seção 8).

## 14. Plano de implementação

**1. Lista**
- `tools/` ganha o script que lê o CSV do e-MEC e gera a migração de dados: filtra
  ativas, trata `NULL`, normaliza o nome, e usa `INSERT … ON CONFLICT (emec_code) DO
  UPDATE` sem tocar em `display_name` nem nos domínios curados.
- Runbook de reimportação e de curadoria (aceitar ou recusar pedido, curar domínio) em
  `docs/`.

**2. Migrações**
- `backend/schema/migrations/00NN_instituicao.sql` com o schema da seção 4.2.
- `backend/schema/migrations/00NN+1_instituicoes_emec.sql` gerada pelo script.
- As duas liberadas pelo nome no `.gitignore`.

**3. Backend**
- Cifra: `backend/internal/infrastructure/crypto` (ou vizinho) com AES-256-GCM e a chave
  de `INSTITUTION_EMAIL_KEY`, lida na subida junto com os outros segredos; ADR 0004
  (variáveis de ambiente) ganha a variável.
- `backend/internal/domain/institution.go`: `ListInstitutions(ctx, country)`,
  `SuggestedInstitutionID(ctx, email)` (domínio igual ou sufixo `.`+domínio, fora os
  públicos), `GetUserInstitution` (calcula `expired`, `renew_soon` e `domain_pending`),
  `SetUserInstitution` (upsert, histórico e limpeza da verificação numa transação),
  `RemoveUserInstitution`, `CreateInstitutionRequest` (conta os abertos dentro da
  transação).
- `backend/internal/domain/institution_verification.go`: pedir código (domínio conhecido,
  desconhecido ou endereço guardado; intervalo; teto diário) e confirmar (tentativas,
  validade, índice único).
- `backend/internal/infrastructure/email/templates/institution_otp.html` e
  `Mailer.SendInstitutionOTP`.
- `backend/internal/domain/repository.go`: `UserStats` ganha
  `Institution *UserInstitution json:"institution"`, preenchido em `GetUserStats`.
  Atualizar o comentário de `purgeAccount`.
- `backend/internal/httpapi/api_errors.go`: os códigos da seção 5.
- `backend/internal/httpapi/institution_handlers.go` e as rotas dentro de `auth(...)` em
  `backend/internal/httpapi/routes.go`.
- Testes: domínio e sufixo, domínio compartilhado, histórico, limite de pedidos,
  vencimento e renovação, domínio desconhecido vira pedido, cifra e HMAC sem e-mail em
  claro, código errado, e-mail já usado, troca apaga, expurgo leva as tabelas novas, e
  em `backend/internal/httpapi/server_test.go` os códigos, status, 202, 422 em vez de 401 e 401 sem token.
- ADR 0011, "Prova de vínculo institucional separada do login, com e-mail cifrado".

**4. Core**
- `shared_core/src/domain.rs`: tipos da seção 6 e os StatusKey no fim do enum.
- `shared_core/src/app.rs`: eventos, Model, ViewModel, `update`, passo do cadastro,
  telemetria, `ProgressFetched`, `OfflineSnapshot`, `LogoutSnapshot`, `TokenCleared`,
  `UndoLogout`, `fold_search()`, e o mapa dos códigos novos para `StatusKey`.
- Testes no estilo dos que existem: lista e sugerida; filtro sem acento; sem lista, sem
  seção nem passo; salvar atualiza o Perfil e grava o retrato; retrato sem o campo
  continua abrindo; 401 dispara o refresh e repete; 422 de código errado não dispara
  refresh; 202 vira `Pending`; logout limpa e o desfazer devolve; 409 vira o `StatusKey`
  certo; envio arma a contagem; pular e escolher emitem a telemetria.

**5. Codegen**
- `just test-core`, depois `just build-ios-ffi` (inclui `codegen`).

**6. i18n**
- `i18n/keys.toml`, `i18n/locales/pt-BR.toml`, `en.toml` e `es.toml`, depois `just i18n`.
- Os StatusKey novos em `ios/LogNiOS/LogNiOS/DesignSystem/StatusKeyCopy.swift`.

**7. iOS**
- `ios/LogNiOS/LogNiOS/Views/ProfileHubView.swift`: sigla, seção, avisos, navegação,
  detent e altura.
- `InstitutionPickerView.swift`, `InstitutionVerifyView.swift`,
  `InstitutionRequestView.swift` (novos), o passo do cadastro, e o fecho `onSubmit` em
  `OTPInputView`.
- `ios/LogNiOS/LogNiOS/Views/ContentView.swift`: `-LogNStartScreen instituicao`, e o
  deep link `verify_institution` em `handleIncomingURL`.
- `just i18n-check`, depois `just xcode`.

**8. Documentos legais**
- Versão nova da política em `logn-conteudo/legal/` com `gen_documentos_legais.py`, com
  o crédito ao e-MEC, depois `just legal-check release` e `just legal-bundle`. O crédito
  entra também na página Sobre.

**9. Testes de ponta a ponta e roteiro**
- `just test-backend`, `just test-core`.
- Seção nova em `docs/testing/roteiro-simulador-ios.md`: cadastro com o passo (pular e
  escolher); perfil sem vínculo; conta de domínio curado com sugerida selecionada;
  buscar por cidade; instituição sem sigla; salvar e ver a sigla; mandar código, errar
  uma vez (a sessão continua), acertar e ver o selo; domínio desconhecido vira "em
  análise", aceitar pelo runbook, reabrir e verificar num toque; `psql` conferindo que
  não há e-mail em claro; modo avião e reabrir; trocar e ver o selo sumir; remover;
  quatro pedidos; visitante; conta de país sem lista; repetir com
  `-AppleLanguages '(es)'`; conferir que o sheet cabe no iPhone menor.

**10. Documentação**
- `docs/specs/logn_profile_hub_spec.md`: uma linha na seção 2 com a seção nova.
