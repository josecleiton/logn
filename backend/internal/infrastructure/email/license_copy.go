package email

import (
	"context"
	"fmt"

	"github.com/josecleiton/logn/backend/internal/locale"
)

// O texto do aviso de revogação e de devolução manual de licença (ADR 0021), por
// língua. Segue a seção 10.5 dos termos: o motivo, o que acontece com XP e progresso,
// e como contestar. Campo com %s recebe o nome da trilha.

// licenseCopy é o texto de uma língua.
type licenseCopy struct {
	RevokedSubject, RevokedEyebrow, RevokedHeading, RevokedLead string // %s trilha
	RejectedSubject, RejectedHeading, RejectedLead              string // %s trilha
	ReasonLabel                                                 string
	ReasonRedistribution, ReasonAccountSharing                  string
	Consequence                                                 string
	Appeal, AppealSharing, RejectedContact                      string
	ReceivedSubject, ReceivedHeading, ReceivedLead              string // %s trilha
	ReviewSubject, ReviewEyebrow, ReviewHeading, ReviewLead     string // %s trilha
	AcceptedSubject, AcceptedHeading, AcceptedLead              string // %s trilha
	AcceptedRefundedLead                                        string // %s trilha
	Footer                                                      string
}

var licenseCopies = map[string]licenseCopy{
	locale.PtBR: {
		RevokedSubject:       "Sua licença de %s foi revogada — LogN",
		RevokedEyebrow:       "LICENÇA REVOGADA",
		RevokedHeading:       "Revogamos sua licença de %s",
		RevokedLead:          "A trilha %s deixou de abrir na sua conta.",
		RejectedSubject:      "Mantivemos a revogação da sua licença de %s — LogN",
		RejectedHeading:      "Analisamos sua contestação",
		RejectedLead:         "Analisamos sua contestação e mantivemos a revogação da licença de %s.",
		ReasonLabel:          "MOTIVO",
		ReasonRedistribution: "Conteúdo da trilha (enunciados, gabaritos ou explicações) foi publicado ou repassado fora do app. É o caso 3 da seção 10.5 dos Termos de Uso.",
		ReasonAccountSharing: "A conta foi usada ao mesmo tempo em aparelhos de pessoas diferentes. É o caso 4 da seção 10.5 dos Termos de Uso.",
		Consequence:          "O XP que você ganhou na trilha continua no seu nível, e o progresso fica guardado: se a licença voltar, ele volta junto. A cópia da trilha no aparelho é apagada na próxima vez que o app falar com o servidor.",
		Appeal:               "Se você discorda, escreva para contact@logn.sh. Confirmamos que recebemos e respondemos em até 5 dias.",
		AppealSharing:        "Se você discorda, escreva para contact@logn.sh. Confirmamos que recebemos e respondemos em até 5 dias, e a licença volta enquanto analisamos.",
		RejectedContact:      "Dúvidas sobre esta decisão: contact@logn.sh.",
		ReceivedSubject:      "Recebemos sua contestação sobre %s — LogN",
		ReceivedHeading:      "Recebemos sua contestação",
		ReceivedLead:         "Recebemos sua contestação da revogação da licença de %s e respondemos em até 5 dias. Enquanto analisamos, a trilha continua sem abrir.",
		ReviewSubject:        "Sua licença de %s voltou enquanto analisamos — LogN",
		ReviewEyebrow:        "CONTESTAÇÃO EM ANÁLISE",
		ReviewHeading:        "Sua licença voltou",
		ReviewLead:           "Recebemos sua contestação. Enquanto analisamos, a licença de %s volta a valer, com o seu progresso. Respondemos em até 5 dias.",
		AcceptedSubject:      "Sua licença de %s foi restabelecida — LogN",
		AcceptedHeading:      "Contestação aceita",
		AcceptedLead:         "Aceitamos sua contestação, e a licença de %s foi restabelecida, com o seu progresso. Abra o app com internet para a trilha voltar a abrir.",
		AcceptedRefundedLead: "Aceitamos sua contestação e desfizemos a revogação. A licença de %s continua fora porque a compra foi reembolsada pela Apple; ela volta, com o progresso, se o reembolso for revertido.",
		Footer:               "Você recebeu este e-mail porque sua conta no LogN tem uma trilha comprada.",
	},
	locale.En: {
		RevokedSubject:       "Your %s licence was revoked — LogN",
		RevokedEyebrow:       "LICENCE REVOKED",
		RevokedHeading:       "We revoked your %s licence",
		RevokedLead:          "The %s trail no longer opens on your account.",
		RejectedSubject:      "We kept the revocation of your %s licence — LogN",
		RejectedHeading:      "We reviewed your appeal",
		RejectedLead:         "We reviewed your appeal and kept the revocation of the %s licence.",
		ReasonLabel:          "REASON",
		ReasonRedistribution: "Content from the trail (problem statements, answer keys or explanations) was published or passed on outside the app. This is case 3 of section 10.5 of the Terms of Use.",
		ReasonAccountSharing: "The account was used at the same time on devices of different people. This is case 4 of section 10.5 of the Terms of Use.",
		Consequence:          "The XP you earned on the trail stays in your level, and your progress is kept: if the licence comes back, it comes back with it. The copy of the trail on the device is deleted the next time the app reaches the server.",
		Appeal:               "If you disagree, write to contact@logn.sh. We confirm that we received it and answer within 5 days.",
		AppealSharing:        "If you disagree, write to contact@logn.sh. We confirm that we received it and answer within 5 days, and the licence comes back while we review it.",
		RejectedContact:      "Questions about this decision: contact@logn.sh.",
		ReceivedSubject:      "We received your appeal about %s — LogN",
		ReceivedHeading:      "We received your appeal",
		ReceivedLead:         "We received your appeal against the revocation of the %s licence and will answer within 5 days. While we review it, the trail stays closed.",
		ReviewSubject:        "Your %s licence is back while we review — LogN",
		ReviewEyebrow:        "APPEAL UNDER REVIEW",
		ReviewHeading:        "Your licence is back",
		ReviewLead:           "We received your appeal. While we review it, the %s licence is valid again, with your progress. We answer within 5 days.",
		AcceptedSubject:      "Your %s licence was restored — LogN",
		AcceptedHeading:      "Appeal accepted",
		AcceptedLead:         "We accepted your appeal, and the %s licence was restored, with your progress. Open the app with an internet connection for the trail to open again.",
		AcceptedRefundedLead: "We accepted your appeal and undid the revocation. The %s licence stays out because Apple refunded the purchase; it comes back, with your progress, if the refund is reversed.",
		Footer:               "You got this email because your LogN account has a purchased trail.",
	},
	locale.Es: {
		RevokedSubject:       "Tu licencia de %s fue revocada — LogN",
		RevokedEyebrow:       "LICENCIA REVOCADA",
		RevokedHeading:       "Revocamos tu licencia de %s",
		RevokedLead:          "La ruta %s dejó de abrirse en tu cuenta.",
		RejectedSubject:      "Mantuvimos la revocación de tu licencia de %s — LogN",
		RejectedHeading:      "Analizamos tu impugnación",
		RejectedLead:         "Analizamos tu impugnación y mantuvimos la revocación de la licencia de %s.",
		ReasonLabel:          "MOTIVO",
		ReasonRedistribution: "Contenido de la ruta (enunciados, soluciones o explicaciones) se publicó o se pasó fuera de la app. Es el caso 3 de la sección 10.5 de los Términos de Uso.",
		ReasonAccountSharing: "La cuenta se usó al mismo tiempo en dispositivos de personas distintas. Es el caso 4 de la sección 10.5 de los Términos de Uso.",
		Consequence:          "El XP que ganaste en la ruta sigue en tu nivel, y el progreso queda guardado: si la licencia vuelve, vuelve con ella. La copia de la ruta en el dispositivo se borra la próxima vez que la app llega al servidor.",
		Appeal:               "Si no estás de acuerdo, escribe a contact@logn.sh. Confirmamos que lo recibimos y respondemos en un plazo de 5 días.",
		AppealSharing:        "Si no estás de acuerdo, escribe a contact@logn.sh. Confirmamos que lo recibimos y respondemos en un plazo de 5 días, y la licencia vuelve mientras lo analizamos.",
		RejectedContact:      "Dudas sobre esta decisión: contact@logn.sh.",
		ReceivedSubject:      "Recibimos tu impugnación sobre %s — LogN",
		ReceivedHeading:      "Recibimos tu impugnación",
		ReceivedLead:         "Recibimos tu impugnación de la revocación de la licencia de %s y respondemos en un plazo de 5 días. Mientras la analizamos, la ruta sigue sin abrirse.",
		ReviewSubject:        "Tu licencia de %s volvió mientras analizamos — LogN",
		ReviewEyebrow:        "IMPUGNACIÓN EN ANÁLISIS",
		ReviewHeading:        "Tu licencia volvió",
		ReviewLead:           "Recibimos tu impugnación. Mientras la analizamos, la licencia de %s vuelve a valer, con tu progreso. Respondemos en un plazo de 5 días.",
		AcceptedSubject:      "Tu licencia de %s fue restablecida — LogN",
		AcceptedHeading:      "Impugnación aceptada",
		AcceptedLead:         "Aceptamos tu impugnación, y la licencia de %s fue restablecida, con tu progreso. Abre la app con internet para que la ruta vuelva a abrirse.",
		AcceptedRefundedLead: "Aceptamos tu impugnación y deshicimos la revocación. La licencia de %s sigue fuera porque Apple reembolsó la compra; vuelve, con tu progreso, si el reembolso se revierte.",
		Footer:               "Recibiste este correo porque tu cuenta de LogN tiene una ruta comprada.",
	},
}

// LicenseNoticeKind diz qual aviso sai.
type LicenseNoticeKind int

const (
	// Revogação manual, a primeira.
	LicenseRevoked LicenseNoticeKind = iota
	// Contestação recusada: a revogação fica.
	LicenseRevokedAfterReview
	// Contestação recebida, com a licença ainda fora (redistribuição).
	LicenseAppealReceived
	// Licença devolvida enquanto a contestação é analisada (compartilhamento).
	LicenseUnderReview
	// Contestação aceita, licença restabelecida.
	LicenseRestored
	// Contestação aceita, mas a licença segue fora porque a Apple reembolsou a compra.
	LicenseAcceptedButRefunded
)

// LicenseData é o que o template license.html recebe.
type LicenseData struct {
	Lang string
	T    LicenseText
}

// LicenseText é o texto pronto do aviso. Campo vazio some do e-mail.
type LicenseText struct {
	Subject, Preheader, Eyebrow, Heading, Lead string
	ReasonLabel, Reason                        string
	Consequence, Appeal                        string
	Footer                                     string
}

// NewLicenseData monta o aviso na língua pedida, ou em locale.Default. `reason` é o
// motivo da revogação manual, como vai no banco.
func NewLicenseData(kind LicenseNoticeKind, lang, trackName, reason string) (LicenseData, error) {
	c, ok := licenseCopies[lang]
	if !ok {
		lang = locale.Default
		c = licenseCopies[lang]
	}
	var why string
	switch reason {
	case "redistribution":
		why = c.ReasonRedistribution
	case "account_sharing":
		why = c.ReasonAccountSharing
	default:
		return LicenseData{}, fmt.Errorf("motivo de revogação manual desconhecido: %q", reason)
	}
	f := func(s string) string { return fmt.Sprintf(s, trackName) }

	var t LicenseText
	switch kind {
	case LicenseRevoked:
		appeal := c.Appeal
		if reason == "account_sharing" {
			appeal = c.AppealSharing
		}
		t = LicenseText{
			Subject: f(c.RevokedSubject), Eyebrow: c.RevokedEyebrow,
			Heading: f(c.RevokedHeading), Lead: f(c.RevokedLead),
			ReasonLabel: c.ReasonLabel, Reason: why,
			Consequence: c.Consequence, Appeal: appeal,
		}
	case LicenseRevokedAfterReview:
		t = LicenseText{
			Subject: f(c.RejectedSubject), Eyebrow: c.RevokedEyebrow,
			Heading: c.RejectedHeading, Lead: f(c.RejectedLead),
			ReasonLabel: c.ReasonLabel, Reason: why,
			Consequence: c.Consequence, Appeal: c.RejectedContact,
		}
	case LicenseAppealReceived:
		t = LicenseText{
			Subject: f(c.ReceivedSubject), Eyebrow: c.ReviewEyebrow,
			Heading: c.ReceivedHeading, Lead: f(c.ReceivedLead),
		}
	case LicenseUnderReview:
		t = LicenseText{
			Subject: f(c.ReviewSubject), Eyebrow: c.ReviewEyebrow,
			Heading: c.ReviewHeading, Lead: f(c.ReviewLead),
		}
	case LicenseRestored:
		t = LicenseText{
			Subject: f(c.AcceptedSubject), Eyebrow: c.ReviewEyebrow,
			Heading: c.AcceptedHeading, Lead: f(c.AcceptedLead),
		}
	case LicenseAcceptedButRefunded:
		t = LicenseText{
			Subject: c.AcceptedHeading + " — LogN", Eyebrow: c.ReviewEyebrow,
			Heading: c.AcceptedHeading, Lead: f(c.AcceptedRefundedLead),
		}
	default:
		return LicenseData{}, fmt.Errorf("aviso de licença desconhecido: %d", kind)
	}
	t.Preheader = t.Lead
	t.Footer = c.Footer
	return LicenseData{Lang: lang, T: t}, nil
}

// SendLicenseNotice manda o aviso de revogação manual ou de resposta à contestação
// (ADR 0021).
func (m *Mailer) SendLicenseNotice(ctx context.Context, toEmail string, kind LicenseNoticeKind, lang, trackName, reason string) error {
	data, err := NewLicenseData(kind, lang, trackName, reason)
	if err != nil {
		return err
	}
	return m.send(ctx, toEmail, data.T.Subject, "license.html", data)
}
