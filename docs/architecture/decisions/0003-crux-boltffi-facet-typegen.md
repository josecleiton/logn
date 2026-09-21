# ADR 0003: Crux FFI via BoltFFI e Facet Typegen

## Status
Aceito

## Contexto
O UniFFI por padrão converte `enums` e `structs` globalmente, gerando colisões fáceis no módulo principal do Swift (ex: A struct `Event` cruzar nomes com `SwiftUI.Event`). Em um projeto complexo, isso escala mal e exige renomeações manuais extensivas ou compromete a performance através das conversões do UDL.

## Decisão
Adotar o modelo já provado em um projeto anterior:
- **Remoção do UniFFI**.
- Uso do **`boltffi`** para as interfaces C geradas e do **`crux_core::bridge`** para transmitir raw Bincode bytes.
- Uso exclusivo do gerador **`facet_typegen`** (`#[derive(Facet)]` com `#[facet(fg::namespace = "LogN")]`), emitindo código Swift rigidamente dentro de Namespaces sem colidir com as bibliotecas Nativas.

## Consequências
- Aumento de performance na ponte.
- O time perde a "simplicidade aparente" do UniFFI, necessitando usar o CLI de codegen customizado para gerar o pacote SPM (`App/Sources/App.swift`).
