# ADR 0002: Envio de E-mails com Suporte a Imagens Inline (CID) e Gomail

**Status:** Substituída pela ADR 0027. As imagens por CID saíram dos templates antes
disso, e gomail deu lugar a go-mail.
**Data:** 21 de Setembro de 2026

## Contexto
O LogN possui um Design System rigoroso (Dark Mode e cores neon). E-mails em HTML puros falham com frequência na renderização de imagens, ou exigem que o usuário force o download de imagens bloqueadas no Gmail/Outlook.

## Decisão
1. Adotamos o padrão **MIME `multipart/related`** para e-mails, injetando as imagens diretamente no corpo da mensagem através da especificação **CID (Content-ID)**.
2. Adotamos a biblioteca `gopkg.in/gomail.v2` no backend Go, que abstrai a construção dolorosa dos *boundaries* do protocolo MIME e gerencia os CIDs automaticamente.
3. Usamos o recurso nativo de binários embutidos do Go (`//go:embed`) para injetar a pasta `templates/assets` na compilação, para que as imagens sejam lidas em memória e transportadas inline nos e-mails.

## Consequências
* **Positivo:** Clientes de e-mail tenderão a não bloquear a imagem de logo da LogN, reduzindo o atrito cognitivo.
* **Positivo:** A infraestrutura de envio fica coesa sem precisar de um bucket S3/CloudFront apenas para hospedar uma logo estática de e-mail.
* **Negativo:** O tamanho do e-mail trafegado aumenta, exigindo que os `assets` da logo sejam muito bem otimizados e compactos.
