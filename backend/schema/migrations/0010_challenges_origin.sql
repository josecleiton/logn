-- Alguns desafios não nascem aqui, e a atribuição precisava de um lugar.
--
-- Não é tag de TAG_THE_PATTERN: aquele array é o conteúdo da pergunta, e origem não é
-- resposta de nada. Também não podia ficar só no payload, porque as structs do Rust
-- têm campos fixos e descartam o que não conhecem — a marca nunca chegaria à tela, e
-- atribuição que ninguém vê não é atribuição.
--
-- Coluna própria, nula para o desafio que nasceu aqui, preenchida para o que veio de
-- fora. O cliente mostra um selo quando ela existe, e tocar nele abre a história.

ALTER TABLE challenges ADD COLUMN origin VARCHAR(50);

COMMENT ON COLUMN challenges.origin IS
    'De onde o desafio veio, quando não foi escrito para o LogN. NULL é o caso comum.';
