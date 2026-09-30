-- Lista de espera do iPhone (ADR 0022): quem deixou o e-mail na landing para um aviso
-- só, quando o app sair para iPhone.
--
-- Sem chave para `users`: a lista é de quem não tem conta, e ter conta não muda nada
-- nela. O e-mail é guardado na forma de `NormalizeEmail`, e o CHECK segura quem gravar
-- por fora dela.
--
-- `confirmed_at` nulo é inscrição pendente: só recebe o aviso quem abriu o link da
-- confirmação. A pendente vive 7 dias contados de `created_at`, e sai na purga diária.
-- Contar do último envio deixava quem reenviasse todo dia manter a linha viva para
-- sempre, contra o que o e-mail promete.
--
-- `sent_at` é quando o e-mail de confirmação saiu. Nulo, ele não saiu (o SMTP falhou), e
-- só então o pedido seguinte manda de novo: a pendente recebe um e-mail só. Sair da
-- lista apaga a linha.
CREATE TABLE waitlist_entries (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email        VARCHAR(254) NOT NULL UNIQUE CHECK (email = lower(email) AND position('@' IN email) > 1),
    locale       VARCHAR(8)   NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    confirmed_at TIMESTAMP WITH TIME ZONE,
    sent_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A purga das pendentes varre por aqui.
CREATE INDEX waitlist_entries_pending ON waitlist_entries (created_at) WHERE confirmed_at IS NULL;
-- O teto global de envios por hora conta por aqui.
CREATE INDEX waitlist_entries_sent_at ON waitlist_entries (sent_at);

CREATE TRIGGER trg_waitlist_entries_updated_at
BEFORE UPDATE ON waitlist_entries
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
