package email

import (
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// O texto dos e-mails de código, por língua. O HTML é um só por e-mail; o que muda de
// língua para língua mora aqui. Eram templates escritos em português, e quem usava o
// app em inglês ou espanhol recebia o código numa língua que não escolheu.
//
// Campo com verbo de formatação (%s, %d) é preenchido em newOTPData: o código, o prazo
// de domain.OTPValidity e a data do pedido.

// otpCopy é o texto de um e-mail de código numa língua.
type otpCopy struct {
	Subject   string
	Preheader string // %s código espaçado, %d minutos
	Eyebrow   string
	Heading   string
	Lead      string // %d minutos; na redefinição vem antes do e-mail da conta
	LeadAfter string // %d minutos; na redefinição, depois do e-mail da conta
	Button    string
	// Aviso para quem não pediu. A redefinição o põe sob um rótulo.
	NotYouLabel string
	NotYou      string
}

// commonCopy é o texto que os dois e-mails de código repetem.
type commonCopy struct {
	Expires     string // %d minutos
	RequestedAt string // %s data do pedido
	DateLayout  string
	Footer      string
}

var commonCopies = map[string]commonCopy{
	locale.PtBR: {
		Expires:     "EXPIRA EM %d:00",
		RequestedAt: "solicitado em %s",
		DateLayout:  "02/01/2006, 15:04",
		Footer:      "E-mail automático de segurança do LogN. Nunca pedimos seu código por telefone, chat ou resposta a esta mensagem.",
	},
	locale.En: {
		Expires:     "EXPIRES IN %d:00",
		RequestedAt: "requested on %s",
		DateLayout:  "Jan 2, 2006, 15:04",
		Footer:      "Automated security email from LogN. We never ask for your code by phone, chat or in a reply to this message.",
	},
	locale.Es: {
		Expires:     "EXPIRA EN %d:00",
		RequestedAt: "solicitado el %s",
		DateLayout:  "02/01/2006, 15:04",
		Footer:      "Correo automático de seguridad de LogN. Nunca pedimos tu código por teléfono, chat ni respondiendo a este mensaje.",
	},
}

var otpCopies = map[string]map[string]otpCopy{
	locale.PtBR: {
		domain.OTPPurposeVerifyEmail: {
			Subject:   "Seu código de verificação — LogN",
			Preheader: "Código %s. Vale por %d minutos. Não compartilhe com ninguém.",
			Eyebrow:   "VERIFICAÇÃO DE CONTA",
			Heading:   "Confirme seu e-mail",
			Lead:      "Digite este código no app para liberar sua conta. Ele vale por %d minutos.",
			Button:    "Abrir o LogN e colar o código",
			NotYou:    "Não foi você que pediu? Ignore esta mensagem — sem o código, nada acontece na conta.",
		},
		domain.OTPPurposeResetPassword: {
			Subject:     "Redefinição de senha — LogN",
			Preheader:   "Código %s. Vale por %d minutos. Se não foi você, ignore.",
			Eyebrow:     "REDEFINIÇÃO DE SENHA",
			Heading:     "Seu código para criar uma senha nova",
			Lead:        "Alguém pediu a redefinição da senha de ",
			LeadAfter:   ". Digite o código no app para escolher outra. Ele vale por %d minutos.",
			Button:      "Redefinir senha",
			NotYouLabel: "NÃO FOI VOCÊ",
			NotYou:      "Sua senha atual continua valendo e nada muda sozinho. Se não reconhece este pedido, ignore este e-mail.",
		},
	},
	locale.En: {
		domain.OTPPurposeVerifyEmail: {
			Subject:   "Your verification code — LogN",
			Preheader: "Code %s. Valid for %d minutes. Don't share it with anyone.",
			Eyebrow:   "ACCOUNT VERIFICATION",
			Heading:   "Confirm your email",
			Lead:      "Enter this code in the app to unlock your account. It's valid for %d minutes.",
			Button:    "Open LogN and paste the code",
			NotYou:    "Didn't ask for this? Ignore this message — without the code, nothing happens to the account.",
		},
		domain.OTPPurposeResetPassword: {
			Subject:     "Password reset — LogN",
			Preheader:   "Code %s. Valid for %d minutes. If it wasn't you, ignore it.",
			Eyebrow:     "PASSWORD RESET",
			Heading:     "Your code to set a new password",
			Lead:        "Someone asked to reset the password for ",
			LeadAfter:   ". Enter the code in the app to choose a new one. It's valid for %d minutes.",
			Button:      "Reset password",
			NotYouLabel: "WASN'T YOU",
			NotYou:      "Your current password still works and nothing changes on its own. If you don't recognize this request, ignore this email.",
		},
	},
	locale.Es: {
		domain.OTPPurposeVerifyEmail: {
			Subject:   "Tu código de verificación — LogN",
			Preheader: "Código %s. Vale por %d minutos. No lo compartas con nadie.",
			Eyebrow:   "VERIFICACIÓN DE CUENTA",
			Heading:   "Confirma tu correo",
			Lead:      "Escribe este código en la app para activar tu cuenta. Vale por %d minutos.",
			Button:    "Abrir LogN y pegar el código",
			NotYou:    "¿No lo pediste tú? Ignora este mensaje: sin el código, no pasa nada en la cuenta.",
		},
		domain.OTPPurposeResetPassword: {
			Subject:     "Restablecer contraseña — LogN",
			Preheader:   "Código %s. Vale por %d minutos. Si no fuiste tú, ignóralo.",
			Eyebrow:     "RESTABLECER CONTRASEÑA",
			Heading:     "Tu código para crear una contraseña nueva",
			Lead:        "Alguien pidió restablecer la contraseña de ",
			LeadAfter:   ". Escribe el código en la app para elegir otra. Vale por %d minutos.",
			Button:      "Restablecer contraseña",
			NotYouLabel: "NO FUISTE TÚ",
			NotYou:      "Tu contraseña actual sigue valiendo y nada cambia solo. Si no reconoces este pedido, ignora este correo.",
		},
	},
}

// welcomeCopy é o texto do e-mail de boas-vindas numa língua. Os rótulos das seções
// são os nomes das abas do app (`tabs` no catálogo de i18n).
type welcomeCopy struct {
	Subject, Preheader, Eyebrow, Heading, Lead, Accepted string
	TrailsLabel, TrailsTitle, TrailsBody                 string
	ArenaLabel, ArenaTitle, ArenaBody                    string
	BoardLabel, BoardTitle, BoardBody                    string
	Footer                                               string
}

var welcomeCopies = map[string]welcomeCopy{
	locale.PtBR: {
		Subject:     "Bem-vindo ao LogN!",
		Preheader:   "Sua conta está pronta. Três minutos por sessão, treze problemas por contest.",
		Eyebrow:     "SESSÃO 01 · 13 PROBLEMAS",
		Heading:     "O ginásio do ICPC cabe no bolso",
		Lead:        "Sua conta está pronta. O LogN treina padrão algorítmico em sessões de três minutos, com a gramática de um contest de verdade: problemas por letra, balão por cor, placar que congela na última hora.",
		Accepted:    "4 de 13 aceitos na sessão de boas-vindas",
		TrailsLabel: "TRILHAS",
		TrailsTitle: "Do primeiro nó ao último",
		TrailsBody:  "Cada trilha termina num desafio que destrava a próxima.",
		ArenaLabel:  "ARENA",
		ArenaTitle:  "Três vidas, relógio correndo",
		ArenaBody:   "Seis formatos de questão — complete a linha, ache o bug, case a complexidade, marque o padrão, pese o trade-off e preveja a saída. Errou numa trap clássica, o app explica por quê.",
		BoardLabel:  "PLACAR",
		BoardTitle:  "Sua faculdade contra as outras",
		BoardBody:   "Standings global e por sede, com penalidade somada como num regional.",
		Footer:      "Você recebeu este e-mail porque criou uma conta no LogN.",
	},
	locale.En: {
		Subject:     "Welcome to LogN!",
		Preheader:   "Your account is ready. Three minutes per session, thirteen problems per contest.",
		Eyebrow:     "SESSION 01 · 13 PROBLEMS",
		Heading:     "The ICPC gym fits in your pocket",
		Lead:        "Your account is ready. LogN trains algorithmic patterns in three-minute sessions, with the grammar of a real contest: problems by letter, balloons by color, a scoreboard that freezes in the last hour.",
		Accepted:    "4 of 13 accepted in the welcome session",
		TrailsLabel: "TRAILS",
		TrailsTitle: "From the first node to the last",
		TrailsBody:  "Each trail ends in a challenge that unlocks the next one.",
		ArenaLabel:  "ARENA",
		ArenaTitle:  "Three lives, clock running",
		ArenaBody:   "Six question formats — complete the line, spot the bug, match the complexity, tag the pattern, weigh the trade-off and predict the output. Fall for a classic trap and the app explains why.",
		BoardLabel:  "STANDINGS",
		BoardTitle:  "Your university against the others",
		BoardBody:   "Global and per-site standings, with penalty added up like at a regional.",
		Footer:      "You got this email because you created a LogN account.",
	},
	locale.Es: {
		Subject:     "¡Bienvenido a LogN!",
		Preheader:   "Tu cuenta está lista. Tres minutos por sesión, trece problemas por contest.",
		Eyebrow:     "SESIÓN 01 · 13 PROBLEMAS",
		Heading:     "El gimnasio del ICPC cabe en tu bolsillo",
		Lead:        "Tu cuenta está lista. LogN entrena patrones algorítmicos en sesiones de tres minutos, con la gramática de un contest de verdad: problemas por letra, globos por color, marcador que se congela en la última hora.",
		Accepted:    "4 de 13 aceptados en la sesión de bienvenida",
		TrailsLabel: "RUTAS",
		TrailsTitle: "Del primer nodo al último",
		TrailsBody:  "Cada ruta termina en un desafío que desbloquea la siguiente.",
		ArenaLabel:  "ARENA",
		ArenaTitle:  "Tres vidas, reloj corriendo",
		ArenaBody:   "Seis formatos de pregunta: completa la línea, encuentra el bug, empareja la complejidad, marca el patrón, sopesa el trade-off y predice la salida. Si caes en una trampa clásica, la app te explica por qué.",
		BoardLabel:  "POSICIONES",
		BoardTitle:  "Tu universidad contra las demás",
		BoardBody:   "Posiciones globales y por sede, con penalización sumada como en un regional.",
		Footer:      "Recibiste este correo porque creaste una cuenta en LogN.",
	},
}

// welcomeFor devolve o texto de boas-vindas na língua pedida, ou na padrão.
func welcomeFor(lang string) (welcomeCopy, string) {
	if c, ok := welcomeCopies[lang]; ok {
		return c, lang
	}
	return welcomeCopies[locale.Default], locale.Default
}

// copyFor devolve o texto na língua pedida, e em locale.Default se ela não existir.
func copyFor(lang, purpose string) (otpCopy, commonCopy, string) {
	if _, ok := otpCopies[lang]; !ok {
		lang = locale.Default
	}
	return otpCopies[lang][purpose], commonCopies[lang], lang
}
