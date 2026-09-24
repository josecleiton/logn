-- País considerado na confirmação de idade, em ISO 3166-1 alfa-2 (`BR`, `US`).
--
-- A idade mínima pode variar por país, e o cadastro confere a caixa "tenho N anos ou
-- mais" contra a tabela do servidor. O país vem da loja do aparelho, com a região do
-- iPhone como alternativa; sem nenhum dos dois, fica NULL e vale a idade padrão.
--
-- Contas criadas antes desta migração ficam com NULL: não há de onde tirar o país delas.
-- A coluna só é escrita no cadastro, então `updated_at` já cobre o resto da linha.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS country CHAR(2),
    ADD CONSTRAINT chk_users_country CHECK (country IS NULL OR country ~ '^[A-Z]{2}$');
