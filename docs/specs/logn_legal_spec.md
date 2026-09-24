# LogN - PRD: Termos de uso, política de privacidade e direitos do titular

Segundo de três PRDs em sequência, depois de `logn_i18n_conteudo_spec.md` e antes de
`logn_trilhas_pagas_spec.md`. Este lançamento destrava publicar o app gratuito nas lojas.
As cláusulas de licença das trilhas pagas entram depois, numa versão nova dos termos.

**O texto final dos documentos precisa de revisão jurídica.** Este PRD define o que cada
documento cobre, como é publicado e o que o app precisa fazer para o texto não mentir. Não
é a redação.

## 1. Por quê

As duas lojas pedem a URL da política de privacidade antes de publicar. Apple exige que a
conta possa ser excluída dentro do app (diretriz 5.1.1(v)). Hoje não existe nenhum
documento, e o botão "Sim, excluir conta" em `SettingsView.swift` só faz logout.

## 2. Quem responde

Dados da entidade e a decisão sobre natureza jurídica: `logn-conteudo/legal/entidade.md`.

## 3. Onde o app é distribuído

- **Países:** todos, menos União Europeia, Reino Unido e China.
  - UE e Reino Unido exigiriam um representante local (GDPR, art. 27, e o equivalente
    britânico), que é serviço pago.
  - China exige número de aprovação do governo para jogos, e o LogN está na categoria de
    jogos educativos.
- **Idade mínima:** 13 anos, ou mais onde a lei do país pedir. A tabela de idade por país
  mora no servidor e começa com 13 para todos os países liberados. Cada país que exija mais
  é acrescentado depois de revisão.
- **Língua dos documentos:** português, inglês e espanhol. O texto diz que a versão em
  português prevalece em caso de divergência, com foro no Brasil. Em alguns países a lei
  local de consumo vale de qualquer forma, e o texto não pode prometer o contrário.

## 4. O que cada documento cobre

### 4.1 Política de privacidade

Tem de descrever o que o app faz hoje, e cada item abaixo existe no código:

- **Dados coletados:**
  - e-mail e senha, esta guardada como hash Argon2id;
  - país e a confirmação de idade mínima;
  - progresso, XP e eventos de jogo;
  - versões de documento aceitas.
- **Guardado no aparelho:**
  - a sessão, no Keychain;
  - a fila offline, o retrato da trilha, o e-mail e o prazo da sessão, em `UserDefaults`,
    para jogar sem rede.
- **Telemetria de uso:** PostHog, por legítimo interesse, com opt-out em Ajustes. É análise
  própria, não rastreio entre apps, e por isso não usa o pedido de rastreio (ATT).
- **Operadores e onde ficam:**
  - Supabase: o banco, nos EUA (us-east-1);
  - Google Cloud Run: a API, nos EUA (us-east1);
  - Resend: o envio de e-mail;
  - PostHog: a telemetria, nos EUA.
- **Transferência internacional (LGPD, art. 33):** nenhum dado fica no Brasil.
- **Retenção:** enquanto a conta existir. Não há eliminação por inatividade.
- **Backups:** o banco ainda não tem backup. Quando a carência da exclusão acaba, a
  eliminação é definitiva. Se isso mudar, o prazo de backup entra numa versão nova.
- **Direitos do titular:** acesso, correção, exportação, eliminação e revogação do
  consentimento, pelo app ou por contact@logn.sh.
- **Menores:** mínimo de 13 anos, ou a idade do país. O servidor não guarda a data de
  nascimento.
- **Segurança:**
  - senha em Argon2id;
  - sessão rotativa, revogada ao trocar a senha;
  - HTTPS;
  - limite de tentativas nos códigos.

### 4.2 Termos de uso

- Quem oferece o serviço (seção 2).
- Conta pessoal, com idade mínima e responsabilidade pelas credenciais.
- Uso aceitável: não automatizar respostas, não atacar a API, não tentar contornar o limite
  de tentativas.
- Conteúdo dos desafios: propriedade do autor, com licença de uso no app.
- Exclusão de conta pelo app, com 30 dias de carência.
- Mudanças nos termos: aviso no app, e novo aceite quando a mudança for relevante.
- Língua que prevalece e foro (seção 3).
- **Espaço reservado para a licença das trilhas pagas.** Entra com o terceiro PRD, numa
  versão marcada como relevante.

## 5. Como os documentos são publicados

O texto vive no repositório privado de conteúdo. Cada versão é uma migração SQL de lá,
publicada pelo `just migrate-prod`, no mesmo caminho do conteúdo dos desafios.

```sql
CREATE TABLE legal_documents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind         VARCHAR(16) NOT NULL CHECK (kind IN ('terms', 'privacy')),
    locale       VARCHAR(8)  NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    version      INT NOT NULL,
    effective_at TIMESTAMP WITH TIME ZONE NOT NULL,
    material     BOOLEAN NOT NULL,
    body_html    TEXT NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (kind, locale, version)
);

CREATE TABLE legal_acceptances (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        VARCHAR(16) NOT NULL,
    version     INT NOT NULL,
    locale      VARCHAR(8) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, kind, version)
);
```

As duas tabelas são só de inserção, sem `UPDATE`, então só levam `created_at` (regra 5 do
AGENTS.md). Uma versão publicada não se edita: corrigir é publicar a próxima. `material`
diz se a versão exige novo aceite.

### Rotas

| Rota | Auth | O que faz |
|---|---|---|
| `GET /legal/terms` | pública | Versão vigente dos termos, em HTML, na língua do `Accept-Language` ou do parâmetro `?lang=` |
| `GET /legal/privacy` | pública | O mesmo para a política |
| `GET /legal/{kind}/{version}` | pública | Uma versão antiga, para quem quer conferir o que aceitou |
| `GET /api/v1/legal/pending` | sim | Versões relevantes que a conta ainda não aceitou |
| `POST /api/v1/legal/accept` | sim | Registra o aceite de `{kind, version}` |

As rotas públicas levam `Cache-Control` curto e a data de vigência no topo do HTML.
`/legal/privacy` é a URL que vai para o App Store Connect.

## 6. O que muda no app

### 6.1 Cadastro

- Pede país e data de nascimento. O servidor recebe a data, confere contra a idade do país
  e a descarta. Guarda só o país e a confirmação, com a data dela.
- Mostra os links dos dois documentos e pede aceite explícito antes de criar a conta. O
  `register` passa a exigir as versões aceitas, e grava em `legal_acceptances`.
- Quem está abaixo da idade do país não cria conta, e a mensagem não sugere mentir a data.

Contas criadas antes deste lançamento passam pela mesma confirmação de idade e de aceite no
próximo login.

### 6.2 Novo aceite

Na abertura com rede, o app consulta `/api/v1/legal/pending`.

- **Versão com `material`:** bloqueia o app até aceitar. A tela mostra o documento e só
  tem aceitar ou sair.
- **Versão sem `material`:** só avisa, uma vez.

### 6.3 Exclusão de conta

Ajustes → "Excluir conta" passa a excluir de verdade.

1. `POST /api/v1/account/delete` marca a conta com `deletion_requested_at`, revoga todas as
   sessões e desloga o aparelho. A conta fica desativada.
2. Um login dentro de 30 dias cancela a exclusão. A tela do login diz que a conta estava
   agendada para exclusão e foi recuperada.
3. Depois de 30 dias, a eliminação é definitiva. Some:
   - a linha em `users` e, em cascata, `refresh_tokens` e `user_progress`;
   - `user_sync_state` e `game_events`, que não têm chave estrangeira para `users` e
     precisam de exclusão explícita;
   - `otps` do e-mail;
   - `legal_acceptances`;
   - a pessoa no PostHog, pela API de exclusão dele.
4. A eliminação roda num endpoint interno autenticado, chamado pelo Cloud Scheduler uma vez
   por dia. Não pode ser uma goroutine no servidor, porque o Cloud Run escala a zero e ela
   não roda. O endpoint é idempotente.

`users` ganha `deletion_requested_at`. O `updated_at`, com o `trigger`, já existe desde a
migração `0013`. O "estado atual" da regra 5 no AGENTS.md já foi corrigido para refletir isso.

### 6.4 Exportação

Ajustes → "Exportar dados" chama `GET /api/v1/account/export` e abre a folha de
compartilhar do iOS com um JSON:
- conta: e-mail, criação, país e confirmação de idade;
- XP e progresso por nó;
- eventos de jogo;
- aceites de documentos.

### 6.5 Telemetria

Ajustes ganha um interruptor "Análise de uso", ligado por padrão. Desligar chama o opt-out
do PostHog, e a escolha fica no aparelho. Vale também para o visitante.

### 6.6 Links

Ajustes, cadastro e login ganham os links "Termos de uso" e "Política de privacidade",
abertos numa tela do app com o HTML das rotas públicas.

## 7. Lojas

- **App Store Connect:** URL da política (`/legal/privacy`) e a etiqueta de privacidade.
  - Dados ligados à pessoa: e-mail, identificador do usuário, progresso de jogo.
  - Dados de uso: telemetria do PostHog.
  - Nada é usado para rastreio.
- **Disponibilidade:** todos os países, menos UE, Reino Unido e China, e só depois de o PRD
  de conteúdo completar as três línguas.
- **Play Console:** fica para quando existir cliente Android. A mesma URL serve.

## 8. E-mail

- O remetente padrão passou a ser `noreply@logn.sh` (`mailer.go`), no domínio do contato.
  Se a variável `SMTP_FROM` estiver definida no Cloud Run, ela precisa acompanhar.
- O domínio `logn.sh` precisa de SPF, DKIM e DMARC configurados para a Resend. Sem isso, o
  código de verificação cai no spam.
- O `welcome.html` já aponta para `logn.sh` em todos os links. O e-mail de boas-vindas não
  é enviado por nenhum fluxo hoje.

## 9. Fora do escopo

- Cláusulas de licença das trilhas pagas: terceiro PRD.
- Representante na UE e no Reino Unido.
- Consentimento dos pais para menores de 13 anos.
- Eliminação automática por inatividade.

## 10. Critérios de aceite

1. `curl /legal/privacy -H 'Accept-Language: es'` devolve a política vigente em espanhol,
   com a data de vigência no topo.
2. Não existe `UPDATE` em `legal_documents` nem em `legal_acceptances` no código.
3. O cadastro sem aceite dos dois documentos é recusado, e o aceite fica gravado com a
   versão e a língua.
4. Uma data de nascimento abaixo de 13 anos não cria conta, e o banco não guarda a data em
   nenhuma tabela.
5. Publicar uma versão com `material` bloqueia o app de quem já tem conta, na abertura
   seguinte, até aceitar.
6. Excluir a conta desloga, e fazer login no dia 29 recupera a conta com o progresso.
7. No dia 31 não sobra linha da conta em nenhuma tabela, nem a pessoa no PostHog.
8. A exportação traz todos os eventos de jogo da conta, e só dela.
9. Com "Análise de uso" desligada, nenhum evento chega ao PostHog.

## 11. Riscos

- **O banco ainda não tem backup.** Perder o banco apaga contas e progresso sem volta. Foi
  considerado, e a decisão é seguir assim por ora.
- Risco sobre a natureza jurídica da entidade: `logn-conteudo/legal/entidade.md`.
- **Tabela de idade incompleta:** um país que exija mais de 13 e não esteja na tabela fica
  com a regra errada até alguém revisar.
