import SwiftUI
import LogNCoreFFI
import App

struct ResetPasswordView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) var dismiss
    
    let email: String
    let otp: String
    
    @State private var password = ""
    @State private var confirmPassword = ""
    
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
                
                Text(Str.Reset.subtitle(email))
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
                
                VStack(spacing: Space.md) {
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
                        core.dispatch(event: .resetPassword(email: email, newPassword: password, otp: otp))
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
    
    private var passwordsMatch: Bool {
        confirmPassword.isEmpty || password == confirmPassword
    }
    
    private var canSubmit: Bool {
        password.count >= 8 && password == confirmPassword && !resetLocked
    }

    /// Travado por um 429: o servidor mandou esperar, e o botão conta o tempo.
    private var resetLocked: Bool { core.viewModel.authCooldownSeconds > 0 }
}
