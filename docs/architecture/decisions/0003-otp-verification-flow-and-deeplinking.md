# ADR 0003: Fluxo de Verificação OTP e Deep Linking

**Status:** Aceito
**Data:** 21 de Setembro de 2026

## Contexto
O processo de criação de contas precisa validar ativamente e-mails em um fluxo unificado sem fricção. Foi desenhado um fluxo em 3 etapas na interface do iOS, mas o backend precisava suportar OTPs numéricos e interligar com abertura de aplicativos nativos a partir de links no e-mail.

## Decisão
1. **Lógica de OTP:** O Backend gerencia os códigos numa tabela `otps`. Códigos têm tempo de vida, propósito (e.g. `verify_email`, `reset_password`) e lógica de **single-use** (delete upon verification). O gerador numérico usa `crypto/rand` para prevenir adivinhações.
2. **Deep Links via Custom Scheme:** Como o app iOS é o principal, registramos o esquema próprio `logn://` e acoplamos manipuladores (`.onOpenURL`) no Root do aplicativo.
3. **Parametrização do E-mail:** Os botões no corpo do HTML chamam diretamente o Custom Scheme (ex: `logn://verify?code=123456&email=user@test.com&purpose=verify_email`).
4. O ciclo inteiro de verificação via link injeta a intenção diretamente na arquitetura Crux do Core (Rust), ignorando cliques e digitações extras pelo usuário.

## Consequências
* **Positivo:** Redução severa da fricção (Zero typing) com avanço automático pela máquina de estados.
* **Positivo:** Funcional offline (a interceptação URL não recarrega o app).
