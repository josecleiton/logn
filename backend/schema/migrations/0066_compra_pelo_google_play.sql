-- Compra pelo Google Play, ao lado da App Store (ADR 0022).
--
-- O id do produto é o mesmo nas duas lojas, e a coluna perde o nome da Apple. O CHECK
-- que liga trilha paga a produto acompanha a coluna sozinho; o índice único ganha o nome
-- novo para não mentir.
ALTER TABLE tracks RENAME COLUMN app_store_product_id TO store_product_id;
ALTER INDEX tracks_app_store_product_id_key RENAME TO tracks_store_product_id_key;

-- A licença diz de que loja veio. Sem isto, a trava de dono ativo era só o id da
-- transação, e as duas lojas dividiam o mesmo espaço de ids. Toda licença até aqui é da
-- App Store: não houve outra loja.
ALTER TABLE entitlements
    ADD COLUMN provider VARCHAR(32) NOT NULL DEFAULT 'apple_storekit'
        CHECK (provider IN ('apple_storekit', 'google_play', 'stripe'));
ALTER TABLE entitlements ALTER COLUMN provider DROP DEFAULT;

DROP INDEX entitlements_one_active_owner;
CREATE UNIQUE INDEX entitlements_one_active_owner
    ON entitlements (provider, original_transaction_id) WHERE status = 'active';

-- O ambiente do Play: `Production` para a compra de verdade (e a de código promocional,
-- que é compra de verdade de graça), `Test` para a de testador de licença, que só as
-- contas cadastradas no Play Console conseguem fazer.
ALTER TABLE store_transactions DROP CONSTRAINT store_transactions_environment_check;
ALTER TABLE store_transactions ADD CONSTRAINT store_transactions_environment_check
    CHECK (environment IN ('Production', 'Sandbox', 'Xcode', 'Test'));
