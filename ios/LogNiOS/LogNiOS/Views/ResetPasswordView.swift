import SwiftUI
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
                    .foregroundColor(LognDark.info)
                
                Text("Reset Password")
                    .font(LognFont.headlineMedium)
                    .foregroundColor(LognDark.textPrimary)
                
                Text("Choose a new password for\n\(email)")
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
                
                VStack(spacing: Space.md) {
                    SecureField("New Password (min 8 characters)", text: $password)
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
                    
                    SecureField("Confirm Password", text: $confirmPassword)
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
                        Text("Save New Password")
                            .font(LognFont.titleMedium)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, Space.md)
                            .background(canSubmit ? LognDark.correct : LognDark.buttonDisabled)
                            .foregroundColor(LognDark.surface)
                            .cornerRadius(Radius.sm)
                    }
                    .disabled(!canSubmit || core.viewModel.isAuthenticating)
                }
                
                if !core.viewModel.displayStatus.isEmpty {
                    Text(core.viewModel.displayStatus)
                        .font(LognFont.label)
                        .foregroundColor(LognDark.info)
                }
                
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
        password.count >= 8 && password == confirmPassword
    }
}
