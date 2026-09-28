import SwiftUI
import LogNCoreFFI
import App

struct ResetPasswordView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) var dismiss
    
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
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: Space.xl) {
                // Header
                Image(systemName: "key.fill")
                    .font(.system(size: 48))
                    // Azul é informação; o que a tela pede é ação, e ação é o acento.
                    .foregroundColor(LognDark.accentInk)
                
                Text(Str.Reset.title)
                    .font(LognFont.headlineMedium)
                    .foregroundColor(LognDark.textPrimary)
                
                Text(asksForCode ? Str.Reset.subtitle_with_code(email) : Str.Reset.subtitle(email))
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)

                VStack(spacing: Space.md) {
                    if asksForCode {
                        codeField
                    }

                    SecureField(Str.Reset.password_prompt, text: $password)
                        .textContentType(.newPassword)
                        .font(LognFont.bodyLarge)
                        .foregroundColor(LognDark.textPrimary)
                        .padding()
                        .background(LognDark.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(
                            RoundedRectangle(cornerRadius: Radius.sm)
                                .stroke(LognDark.lineDim, lineWidth: 1)
                        )
                    
                    SecureField(Str.Reset.confirm_prompt, text: $confirmPassword)
                        .textContentType(.newPassword)
                        .font(LognFont.bodyLarge)
                        .foregroundColor(LognDark.textPrimary)
                        .padding()
                        .background(LognDark.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(
                            RoundedRectangle(cornerRadius: Radius.sm)
                                .stroke(passwordsMatch ? LognDark.lineDim : LognDark.warn, lineWidth: 1)
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
                            .background(canSubmit ? LognDark.accent : LognDark.buttonDisabled)
                            .foregroundColor(canSubmit ? LognDark.onAccent : LognDark.textDim)
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
        TextField(Str.Reset.code_prompt, text: $typedCode)
            .textContentType(.oneTimeCode)
            .keyboardType(.numberPad)
            .font(.plexMonoSemiBold(20))
            .foregroundColor(LognDark.textPrimary)
            .padding()
            .background(LognDark.surface)
            .cornerRadius(Radius.sm)
            .overlay(
                RoundedRectangle(cornerRadius: Radius.sm)
                    .stroke(LognDark.lineDim, lineWidth: 1)
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
                .foregroundColor(resendLocked ? LognDark.textDim : LognDark.textSecondary)
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
