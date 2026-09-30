package email

import "github.com/josecleiton/logn/backend/internal/locale"

// O texto do e-mail de confirmação da lista de espera do iPhone (ADR 0022), por língua.
// Diz o que a pessoa assina: um e-mail só, no lançamento, e a saída a qualquer hora.

type waitlistCopy struct {
	Subject, Preheader, Eyebrow, Heading, Lead, Button string
	NotYou, Leave, Footer                              string
}

var waitlistCopies = map[string]waitlistCopy{
	locale.PtBR: {
		Subject:   "Confirme sua inscrição na lista do iPhone — LogN",
		Preheader: "Um clique para entrar na lista. Mandamos um e-mail só, quando a versão para iPhone sair.",
		Eyebrow:   "LISTA DE ESPERA · IPHONE",
		Heading:   "Confirme para entrar na lista",
		Lead:      "Alguém deixou este endereço na lista de espera da versão para iPhone. Confirme e mandamos um e-mail só, quando ela sair.",
		Button:    "Confirmar inscrição",
		NotYou:    "Não foi você? Ignore esta mensagem: sem a confirmação, o endereço sai da lista em 7 dias.",
		Leave:     "Sair da lista agora",
		Footer:    "Você recebeu este e-mail porque o endereço foi deixado na lista de espera em logn.sh.",
	},
	locale.En: {
		Subject:   "Confirm your spot on the iPhone list — LogN",
		Preheader: "One click to join the list. We send a single e-mail, when the iPhone version is out.",
		Eyebrow:   "WAITING LIST · IPHONE",
		Heading:   "Confirm to join the list",
		Lead:      "Someone left this address on the waiting list for the iPhone version. Confirm and we will send a single e-mail, when it is out.",
		Button:    "Confirm",
		NotYou:    "Wasn't you? Ignore this message: without the confirmation, the address leaves the list in 7 days.",
		Leave:     "Leave the list now",
		Footer:    "You got this e-mail because the address was left on the waiting list at logn.sh.",
	},
	locale.Es: {
		Subject:   "Confirma tu inscripción en la lista del iPhone — LogN",
		Preheader: "Un clic para entrar en la lista. Enviamos un solo correo, cuando salga la versión para iPhone.",
		Eyebrow:   "LISTA DE ESPERA · IPHONE",
		Heading:   "Confirma para entrar en la lista",
		Lead:      "Alguien dejó esta dirección en la lista de espera de la versión para iPhone. Confirma y te enviamos un solo correo, cuando salga.",
		Button:    "Confirmar inscripción",
		NotYou:    "¿No fuiste tú? Ignora este mensaje: sin la confirmación, la dirección sale de la lista en 7 días.",
		Leave:     "Salir de la lista ahora",
		Footer:    "Recibiste este correo porque la dirección se dejó en la lista de espera en logn.sh.",
	},
}

// WaitlistData é o que o e-mail de confirmação recebe.
type WaitlistData struct {
	Lang       string
	ConfirmURL string
	LeaveURL   string
	T          waitlistCopy
}

// NewWaitlistData monta os dados do e-mail, para teste e para o envio. Língua fora das
// servidas sai em locale.Default.
func NewWaitlistData(lang, confirmURL, leaveURL string) WaitlistData {
	c, ok := waitlistCopies[lang]
	if !ok {
		lang = locale.Default
		c = waitlistCopies[lang]
	}
	return WaitlistData{Lang: lang, ConfirmURL: confirmURL, LeaveURL: leaveURL, T: c}
}

// SendWaitlistConfirmation manda o link de confirmação da lista de espera.
func (m *Mailer) SendWaitlistConfirmation(toEmail, lang, confirmURL, leaveURL string) error {
	data := NewWaitlistData(lang, confirmURL, leaveURL)
	return m.sendWithHeaders(toEmail, data.T.Subject, "waitlist.html", data, WaitlistHeaders(leaveURL))
}

// WaitlistHeaders são os cabeçalhos a mais do e-mail: o `List-Unsubscribe` com o link
// de saída, que o cliente de e-mail abre no navegador, na página do botão.
//
// Sem `List-Unsubscribe-Post` (RFC 8058) por enquanto. Com ele, o clique de saída é um
// POST do servidor do provedor de e-mail, e o Bot Fight Mode da borda desafia servidor
// com JS, sem exceção (ADR 0013): a saída falharia calada. Ele volta quando houver
// caminho que chegue ao backend sem passar pelo desafio, e o DKIM tem de assinar os
// dois cabeçalhos.
func WaitlistHeaders(leaveURL string) map[string]string {
	return map[string]string{"List-Unsubscribe": "<" + leaveURL + ">"}
}
