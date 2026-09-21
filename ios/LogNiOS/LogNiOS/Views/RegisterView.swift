import SwiftUI
import LogN
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
                HStack(alignment: .center, spacing: 12) {
                    BalloonShape(
                        color: LognDark.accent,
                        state: .filled,
                        bodySize: 20,
                        showString: true,
                        showHighlight: true
                    )
                    .frame(width: 34, height: 43)
                    
                    Text("LogN")
                        .font(.system(size: 40, weight: .semibold))
                        .tracking(-0.04 * 40) // letter-spacing: -0.04em
                        .foregroundColor(LognDark.textPrimary)
                }
                
                VStack(spacing: 4) {
                    Text(stepTitle)
                        .font(.system(size: 24, weight: .semibold))
                        .foregroundColor(LognDark.textPrimary)
                    
                    Text(stepSubtitle)
                        .font(.custom("IBMPlexMono-Regular", size: 14))
                        .foregroundColor(LognDark.textSecondary)
                        .multilineTextAlignment(.center)
                }
                
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
        VStack(spacing: Space.sm) {
            TextField("e-mail", text: $email)
                .textContentType(.emailAddress)
                .keyboardType(.emailAddress)
                .autocapitalization(.none)
                .font(.custom("IBMPlexMono-Regular", size: 14))
                .foregroundColor(LognDark.textPrimary)
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(LognDark.line, lineWidth: 1)
                )
            
            Button(action: {
                core.dispatch(event: LogN.Event.requestOtp(email: email, purpose: "verify_email"))
                step = .otp
            }) {
                Text(core.viewModel.isAuthenticating ? "Enviando..." : "Enviar Código")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundColor(email.contains("@") ? LognDark.surface : LognDark.textDim)
                    .frame(maxWidth: .infinity)
                    .frame(height: 52)
                    .background(email.contains("@") ? LognDark.accent : Color(hex: "1B1D20"))
                    .cornerRadius(Radius.sm)
            }
            .disabled(!email.contains("@") || core.viewModel.isAuthenticating)
            .padding(.top, 4)
        }
    }
    
    private var otpStep: some View {
        OTPInputView(email: email, purpose: "verify_email")
            .environmentObject(core)
    }
    
    private var passwordStep: some View {
        VStack(spacing: Space.sm) {
            SecureField("senha (mín 8 chars)", text: $password)
                .textContentType(.newPassword)
                .font(.custom("IBMPlexMono-Regular", size: 14))
                .foregroundColor(LognDark.textPrimary)
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(LognDark.line, lineWidth: 1)
                )
            
            SecureField("confirmar senha", text: $confirmPassword)
                .textContentType(.newPassword)
                .font(.custom("IBMPlexMono-Regular", size: 14))
                .foregroundColor(LognDark.textPrimary)
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(passwordsMatch ? LognDark.line : LognDark.wrong, lineWidth: 1)
                )
            
            Button(action: {
                core.dispatch(event: .register(email: email, password: password, otp: core.viewModel.otpEmail))
            }) {
                Text("Criar Conta")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundColor(canRegister ? LognDark.surface : LognDark.textDim)
                    .frame(maxWidth: .infinity)
                    .frame(height: 52)
                    .background(canRegister ? LognDark.accent : Color(hex: "1B1D20"))
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canRegister || core.viewModel.isAuthenticating)
            .padding(.top, 4)
        }
    }
    
    // MARK: - Helpers
    
    private var stepTitle: String {
        switch step {
        case .email: return "Criar Conta"
        case .otp: return "Verificar e-mail"
        case .password: return "Definir Senha"
        }
    }
    
    private var stepSubtitle: String {
        switch step {
        case .email: return "Para salvar seu progresso"
        case .otp: return "Digite o código enviado para \(email)"
        case .password: return "Quase lá! Escolha sua senha"
        }
    }
    
    private var passwordsMatch: Bool {
        confirmPassword.isEmpty || password == confirmPassword
    }
    
    private var canRegister: Bool {
        password.count >= 8 && password == confirmPassword
    }
}
