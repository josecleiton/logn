import SwiftUI
import LogNCoreFFI
import App

struct ResetPasswordView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) var dismiss
    @Environment(\.colorScheme) private var colorScheme
    
    let email: String
    /// O código que veio no link do e-mail. Vazio quando a tela abre pelo "Esqueci a
    /// senha" do login: aí o jogador digita o código que recebeu.
    let otp: String

    @State private var typedCode = ""
    @State private var password = ""
    @State private var confirmPassword = ""

    /// Pedir o campo só quando o link não trouxe código.
    private var asksForCode: Bool { otp.isEmpty }
    private var code: String { asksForCode ? typedCode : otp }
    
    var body: some View {
        ZStack {
            LognColors.canvas.ignoresSafeArea()
            
            VStack(spacing: Space.xl) {
                // Header
                Image(systemName: "key.fill")
                    .font(.system(size: 48))
                    // Azul é informação; o que a tela pede é ação, e ação é o acento. A chave
                    // é forma cheia, então é a base que preenche, como o balão do logo, e não
                    // a tinta.
                    .foregroundColor(LognColors.accent)
                
                Text(Str.Reset.title)
                    .font(LognFont.headlineMedium)
                    .foregroundColor(LognColors.textPrimary)
                
                Text(asksForCode ? Str.Reset.subtitle_with_code(email) : Str.Reset.subtitle(email))
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognColors.textSecondary)
                    .multilineTextAlignment(.center)

                VStack(spacing: Space.md) {
                    if asksForCode {
                        codeField
                    }

                    PasswordField(Str.Reset.password_prompt, text: $password)
                        .textContentType(.newPassword)
                        .font(LognFont.bodyLarge)
                        .foregroundColor(LognColors.textPrimary)
                        .padding()
                        .background(LognColors.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(
                            RoundedRectangle(cornerRadius: Radius.sm)
                                .stroke(LognColors.lineDim, lineWidth: 1)
                        )
                    
                    PasswordField(Str.Reset.confirm_prompt, text: $confirmPassword)
                        .textContentType(.newPassword)
                        .font(LognFont.bodyLarge)
                        .foregroundColor(LognColors.textPrimary)
                        .padding()
                        .background(LognColors.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(
                            RoundedRectangle(cornerRadius: Radius.sm)
                                .stroke(passwordsMatch ? LognColors.lineDim : LognColors.warn, lineWidth: 1)
                        )
                    
                    Button(action: {
                        core.dispatch(event: .resetPassword(email: email, newPassword: password, otp: code))
                    }) {
                        Text(resetLocked
                             ? Str.Status.wait_seconds(Int(core.viewModel.authCooldownSeconds))
                             : Str.Reset.save_password)
                            .font(LognFont.titleMedium)
                            .monospacedDigit()
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, Space.md)
                            // Verde é veredito do juiz, não confirmação de formulário.
                            .background(canSubmit ? LognColors.accent : LognColors.buttonDisabled)
                            .foregroundColor(canSubmit ? LognColors.onAccent : LognColors.textDim)
                            .cornerRadius(Radius.sm)
                    }
                    .disabled(!canSubmit || core.viewModel.isAuthenticating)
                }
                
                StatusLine(status: core.viewModel.status)
                
                Spacer()
            }
            .padding(.horizontal, Space.screenMargin)
            .padding(.top, Space.xl)
        }
        .onChange(of: core.viewModel.hasAccessToken) { hasToken in
            if hasToken {
                dismiss() // Ou fechar todo o fluxo
            }
        }
    }
    
    /// O código num campo só, e não nos seis da `OTPInputView`: aquela confere o código
    /// no servidor ao completar, e aqui quem o consome é a própria troca de senha.
    @ViewBuilder
    private var codeField: some View {
        TextField(Str.Reset.code_prompt, text: $typedCode, prompt: lightPrompt(Str.Reset.code_prompt, colorScheme))
            .textContentType(.oneTimeCode)
            .keyboardType(.numberPad)
            .font(.plexMonoSemiBold(20))
            .foregroundColor(LognColors.textPrimary)
            .padding()
            .background(LognColors.surface)
            .cornerRadius(Radius.sm)
            .overlay(
                RoundedRectangle(cornerRadius: Radius.sm)
                    .stroke(LognColors.lineDim, lineWidth: 1)
            )
            .onChange(of: typedCode) { newValue in
                let digits = String(newValue.filter(\.isNumber).prefix(6))
                if digits != newValue { typedCode = digits }
            }

        Button(action: {
            core.dispatch(event: .requestOtp(email: email, purpose: "reset_password"))
        }) {
            Text(resendLocked
                 ? Str.Status.wait_seconds(Int(core.viewModel.resendCooldownSeconds))
                 : Str.Otp.resend_code)
                .font(.plexSans(13.5))
                .monospacedDigit()
                .foregroundColor(resendLocked ? LognColors.textDim : LognColors.textSecondary)
                .underline(!resendLocked)
                .frame(minHeight: Space.minTouch)
        }
        .buttonStyle(.plain)
        .disabled(resendLocked)
    }

    private var passwordsMatch: Bool {
        confirmPassword.isEmpty || password == confirmPassword
    }

    private var canSubmit: Bool {
        code.count == 6 && password.count >= 8 && password == confirmPassword && !resetLocked
    }

    /// Travado por um 429: o servidor mandou esperar, e o botão conta o tempo.
    private var resetLocked: Bool { core.viewModel.authCooldownSeconds > 0 }
    private var resendLocked: Bool { core.viewModel.resendCooldownSeconds > 0 }
}
