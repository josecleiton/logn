import SwiftUI
import LogNCoreFFI
import LogN

/// O botão "Continuar com o Google", no desenho do design system: superfície escura com
/// borda, o G colorido à esquerda.
struct GoogleSignInButton: View {
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 12) {
                Image("GoogleIcon")
                    .renderingMode(.original)
                    .resizable()
                    .scaledToFit()
                    .frame(width: 20, height: 20)
                Text(Str.Login.sign_in_google)
                    .font(.plexSansMedium(15))
                    .foregroundColor(LognColors.textPrimary)
            }
            .frame(maxWidth: .infinity)
            .frame(height: 52)
            .background(LognColors.surface)
            .cornerRadius(Radius.sm)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognColors.line, lineWidth: 1))
        }
    }
}

/// Primeiro login com o Google: a conta ainda não existe (ADR 0016).
///
/// Pede o mesmo que o cadastro por e-mail, menos e-mail, código e senha, que o Google já
/// resolveu: a idade mínima do país e o aceite da versão vigente dos termos e da
/// política. O token do Google fica no Core; esta tela só responde sim ou não.
struct SocialSignupSheet: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.colorScheme) private var colorScheme

    @State private var ageConfirmed = false
    @State private var termsAccepted = false
    @State private var legalSheet: LegalKind?
    @State private var country = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            Text(Str.Login.google_signup_title)
                .font(.plexSansSemiBold(18))
                .foregroundColor(LognColors.textPrimary)

            Text(Str.Login.google_signup_reason)
                .font(.plexSans(14))
                .foregroundColor(LognColors.textSecondary)

            checkbox(isOn: $ageConfirmed, label: Str.Register.age_confirmation(Int(core.viewModel.minAge)))

            checkbox(isOn: $termsAccepted, label: Str.Register.terms_confirmation) {
                // A busca da abertura pode ter falhado sem rede; marcar é a hora de
                // tentar de novo.
                // Antes de o país chegar, a busca da abertura ainda está a caminho.
                if termsAccepted && !core.viewModel.legalVersionsReady && !country.isEmpty {
                    core.dispatch(event: .fetchLegalVersions(country: country))
                }
            }

            // Fora do botão da caixa: dentro dele, o toque no link marcaria a caixa.
            LegalLinksRow(presented: $legalSheet)

            if termsAccepted && !core.viewModel.legalVersionsReady {
                Text(Str.Register.legal_pending)
                    .font(.plexSans(13, relativeTo: .footnote))
                    .foregroundColor(LognColors.textMuted)
            }

            StatusLine(status: core.viewModel.status)

            Button(action: {
                core.dispatch(event: .completeSocialSignup(ageConfirmed: ageConfirmed, legalAccepted: termsAccepted))
            }) {
                Text(continueLabel)
                    .font(.plexSansSemiBold(15))
                    .monospacedDigit()
                    .foregroundColor(canContinue ? LognColors.onAccent : LognColors.textDim)
                    .frame(maxWidth: .infinity, minHeight: 52)
                    .background(canContinue ? LognColors.accent : LognColors.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canContinue || core.viewModel.isAuthenticating)

            Button(action: { core.dispatch(event: .cancelSocialSignup) }) {
                Text(Str.Login.google_signup_cancel)
                    .font(.plexSans(14))
                    .foregroundColor(LognColors.textSecondary)
                    .frame(maxWidth: .infinity, minHeight: 44)
            }
            .buttonStyle(.plain)
        }
        .padding(20)
        .background(LognColors.surfaceRaised)
        .task {
            country = await DeviceCountry.current()
            core.dispatch(event: .fetchLegalVersions(country: country))
        }
        .onChange(of: core.viewModel.minAge) { _ in
            // A frase da caixa mudou de idade: quem marcou antes declarou outra coisa.
            ageConfirmed = false
        }
        .sheet(item: $legalSheet) { kind in
            LegalDocumentView(kind: kind)
        }
    }

    @ViewBuilder
    private func checkbox(isOn: Binding<Bool>, label: String, onToggle: @escaping () -> Void = {}) -> some View {
        Button(action: {
            isOn.wrappedValue.toggle()
            onToggle()
        }) {
            HStack(alignment: .top, spacing: 10) {
                Image(systemName: isOn.wrappedValue ? "checkmark.square.fill" : "square")
                    .lognCheckbox(checked: isOn.wrappedValue, colorScheme)
                    .font(.system(size: 18))
                Text(label)
                    .font(.plexSans(13))
                    .foregroundColor(LognColors.textSecondary)
                    .multilineTextAlignment(.leading)
            }
        }
    }

    private var locked: Bool { core.viewModel.authCooldownSeconds > 0 }

    private var canContinue: Bool {
        ageConfirmed && termsAccepted && core.viewModel.legalVersionsReady && !locked
    }

    private var continueLabel: String {
        if locked { return Str.Status.wait_seconds(Int(core.viewModel.authCooldownSeconds)) }
        return Str.Login.google_signup_continue
    }
}
