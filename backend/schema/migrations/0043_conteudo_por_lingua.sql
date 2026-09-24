-- Conteúdo em mais de uma língua (PRD `docs/specs/logn_i18n_conteudo_spec.md`, ADR 0009).
--
-- O texto que a pessoa lê sai de `challenges.payload` e de `skill_nodes` e vai para uma
-- tabela de tradução por língua. O payload fica só com a estrutura neutra — código,
-- opções, gabarito —, que é uma só para as três línguas: o código dos desafios é
-- escrito em inglês, e a linha do bug, a saída esperada e as opções não mudam com a
-- língua de quem lê.
--
-- Tudo numa migração: cria as tabelas, copia o português que já existe, e só então tira
-- o texto do payload e as colunas de nome dos nós. Rodar `migrate-prod` e
-- `deploy-backend` em seguida: entre os dois, o backend antigo serve desafio sem título.
--
-- Em banco zerado a ordem também funciona: as migrações de conteúdo antigas (0021 a
-- 0035) inserem o payload com texto em português, e esta o move para as traduções. No
-- repositório público, sem conteúdo, esta roda sobre tabelas vazias.

CREATE TABLE skill_node_translations (
    node_id     UUID         NOT NULL REFERENCES skill_nodes(id) ON DELETE CASCADE,
    locale      VARCHAR(8)   NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    name        VARCHAR(255) NOT NULL CHECK (length(btrim(name)) > 0),
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (node_id, locale)
);

CREATE TABLE challenge_translations (
    challenge_id  VARCHAR(50) NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    locale        VARCHAR(8)  NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    title         TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    description   TEXT NOT NULL CHECK (length(btrim(description)) > 0),
    -- A regra da `chk_explanation_presente` (0005) muda de lugar: todo desafio explica
    -- a resposta, em toda língua em que aparece.
    explanation   TEXT NOT NULL CHECK (length(btrim(explanation)) > 0),
    -- Nota do painel de variáveis dos DRY_RUN.
    watch_note    TEXT,
    -- Rótulo de cada opção neutra nesta língua, `{"queue": "Fila"}`. As tags dos
    -- TAG_THE_PATTERN ficam no payload como identificadores; a API troca pelo rótulo.
    -- Nulo quando a opção já é o próprio texto (código, complexidade).
    option_labels JSONB CHECK (option_labels IS NULL OR jsonb_typeof(option_labels) = 'object'),
    created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (challenge_id, locale)
);

-- Regra 5 do AGENTS.md: `updated_at` só vale se for mantido.
CREATE TRIGGER trg_skill_node_translations_updated_at
    BEFORE UPDATE ON skill_node_translations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_challenge_translations_updated_at
    BEFORE UPDATE ON challenge_translations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- O português que já existe vira a tradução pt-BR.
INSERT INTO skill_node_translations (node_id, locale, name, description)
SELECT id, 'pt-BR', name, description FROM skill_nodes;

INSERT INTO challenge_translations (challenge_id, locale, title, description, explanation, watch_note)
SELECT id, 'pt-BR',
       payload->'content'->>'title',
       payload->'content'->>'description',
       payload->'validation'->>'explanation',
       payload->'content'->>'watch_note'
FROM challenges;

-- Cor e ícone do nó: identificador neutro (`adhoc`, `graphs`…). O app escolhia a cor
-- procurando palavras no nome, e em espanhol três nós caíam na cor errada. Preenchido
-- pelas migrações de conteúdo; nulo, o app ainda cai no nome.
ALTER TABLE skill_nodes
    ADD COLUMN topic VARCHAR(32) CHECK (topic IS NULL OR topic ~ '^[a-z][a-z_]*$');

-- Só depois de copiar: o texto sai do payload e do nó.
ALTER TABLE challenges DROP CONSTRAINT IF EXISTS chk_explanation_presente;

UPDATE challenges
SET payload = payload #- '{content,title}' #- '{content,description}'
                      #- '{content,watch_note}' #- '{validation,explanation}';

-- E não volta: texto de conteúdo mora nas traduções.
ALTER TABLE challenges ADD CONSTRAINT chk_payload_sem_texto CHECK (
    NOT (payload->'content' ?| ARRAY['title', 'description', 'watch_note'])
    AND NOT (payload->'validation' ? 'explanation')
);

ALTER TABLE skill_nodes DROP COLUMN name, DROP COLUMN description;
