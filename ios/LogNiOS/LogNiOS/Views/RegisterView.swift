import SwiftUI
import App

struct RegisterView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) var dismiss
    
    @State private var email = ""
    @State private var password = ""
    @State private var confirmPassword = ""
    @State private var otpCode = ""
    
    enum Step { case email, otp, password }
    @State private var step: Step = .email
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: Space.xl) {
                // Header
                Image(systemName: step == .email ? "envelope" : step == .otp ? "lock.shield" : "key.fill")
                    .font(.system(size: 48))
                    .foregroundColor(LognDark.info)
                
                Text(stepTitle)
                    .font(LognFont.headlineMedium)
                    .foregroundColor(LognDark.textPrimary)
                
                Text(stepSubtitle)
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
                
                switch step {
                case .email:
                    emailStep
                case .otp:
                    otpStep
                case .password:
                    passwordStep
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
        .onChange(of: core.viewModel.otpVerified) { verified in
            if verified {
                step = .password
            }
        }
        .onChange(of: core.viewModel.hasAccessToken) { hasToken in
            if hasToken {
                dismiss()
            }
        }
    }
    
    // MARK: - Steps
    
    private var emailStep: some View {
        VStack(spacing: Space.md) {
            TextField("your@email.com", text: $email)
                .textContentType(.emailAddress)
                .keyboardType(.emailAddress)
                .autocapitalization(.none)
                .font(LognFont.bodyLarge)
                .foregroundColor(LognDark.textPrimary)
                .padding()
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(LognDark.lineDim, lineWidth: 1)
                )
            
            Button(action: {
                core.dispatch(event: .requestOTP(email: email, purpose: "verify_email"))
                step = .otp
            }) {
                Text("Send Verification Code")
                    .font(LognFont.titleMedium)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, Space.md)
                    .background(email.contains("@") ? LognDark.info : LognDark.buttonDisabled)
                    .foregroundColor(LognDark.surface)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!email.contains("@") || core.viewModel.isAuthenticating)
        }
    }
    
    private var otpStep: some View {
        OTPInputView(email: email, purpose: "verify_email")
            .environmentObject(core)
    }
    
    private var passwordStep: some View {
        VStack(spacing: Space.md) {
            SecureField("Password (min 8 characters)", text: $password)
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
                core.dispatch(event: .register(email: email, password: password, otp: otpCode))
            }) {
                Text("Create Account")
                    .font(LognFont.titleMedium)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, Space.md)
                    .background(canRegister ? LognDark.success : LognDark.buttonDisabled)
                    .foregroundColor(LognDark.surface)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canRegister || core.viewModel.isAuthenticating)
        }
    }
    
    // MARK: - Helpers
    
    private var stepTitle: String {
        switch step {
        case .email: return "Create Account"
        case .otp: return "Verify E-mail"
        case .password: return "Set Password"
        }
    }
    
    private var stepSubtitle: String {
        switch step {
        case .email: return "Enter your e-mail to get started"
        case .otp: return "Enter the code sent to \(email)"
        case .password: return "Almost there! Choose a strong password"
        }
    }
    
    private var passwordsMatch: Bool {
        confirmPassword.isEmpty || password == confirmPassword
    }
    
    private var canRegister: Bool {
        password.count >= 8 && password == confirmPassword
    }
}
