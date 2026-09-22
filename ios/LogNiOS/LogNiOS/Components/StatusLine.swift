import SwiftUI
import LogN

/// A linha de status das telas de autenticação.
///
/// Existe em um lugar só porque estava em quatro, cada uma com sua cor: o cadastro
/// mostrava a mesma frase duas vezes, uma em `warn` e outra em `info`, porque a tela
/// hospedeira e a de código desenhavam as duas.
///
/// Estado em andamento sai em `textSecondary`; problema, em `wrongInk`. Sucesso não
/// aparece — a prova é a tela avançar.
struct StatusLine: View {
    let status: LogN.StatusKey

    var body: some View {
        if let copy = status.copy {
            Text(copy)
                .lognLabel()
                .foregroundColor(status.isInProgress ? LognDark.textSecondary : LognDark.wrongInk)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(status.isInProgress ? [] : .isStaticText)
        }
    }
}
