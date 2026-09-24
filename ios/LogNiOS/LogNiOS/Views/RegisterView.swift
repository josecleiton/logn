import SwiftUI
import LogNCoreFFI
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
    @State private var ageConfirmed = false
    @State private var termsAccepted = false
    @State private var legalSheet: LegalKind?

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: Space.xl) {
                BalloonMarquee()
                    .padding(.top, 14)

                BrandLockup(fontSize: 40)
                
                VStack(spacing: 4) {
                    Text(stepTitle)
                        .font(LognFont.headlineMedium)
                        .foregroundColor(LognDark.textPrimary)
                    
                    Text(stepSubtitle)
                        .font(.plexMono(14))
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
                
                // Uma linha de status na tela inteira: a de código não repete.
                StatusLine(status: core.viewModel.status)
                
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
        .onAppear {
            // O cadastro aceita a versão vigente dos documentos, e só o servidor sabe
            // qual é.
            core.dispatch(event: .fetchLegalVersions)
        }
        .sheet(item: $legalSheet) { kind in
            LegalDocumentView(kind: kind)
        }
    }
    
    // MARK: - Steps
    
    private var emailStep: some View {
        VStack(spacing: Space.sm) {
            TextField(Str.Register.email_prompt, text: $email)
                .textContentType(.emailAddress)
                .keyboardType(.emailAddress)
                .autocapitalization(.none)
                .font(.plexMono(14))
                .foregroundColor(LognDark.textPrimary)
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(LognDark.line, lineWidth: 1)
                )
                
            VStack(alignment: .leading, spacing: 12) {
                Button(action: { ageConfirmed.toggle() }) {
                    HStack(alignment: .top, spacing: 10) {
                        Image(systemName: ageConfirmed ? "checkmark.square.fill" : "square")
                            .foregroundColor(ageConfirmed ? LognDark.accent : LognDark.textDim)
                            .font(.system(size: 18))
                        Text(Str.Register.age_confirmation)
                            .font(.plexSans(13))
                            .foregroundColor(LognDark.textSecondary)
                            .multilineTextAlignment(.leading)
                    }
                }
                
                Button(action: {
                    termsAccepted.toggle()
                    // A busca da abertura pode ter falhado sem rede; marcar a caixa é a
                    // hora de tentar de novo.
                    if termsAccepted && !core.viewModel.legalVersionsReady {
                        core.dispatch(event: .fetchLegalVersions)
                    }
                }) {
                    HStack(alignment: .top, spacing: 10) {
                        Image(systemName: termsAccepted ? "checkmark.square.fill" : "square")
                            .foregroundColor(termsAccepted ? LognDark.accent : LognDark.textDim)
                            .font(.system(size: 18))
                        Text(Str.Register.terms_confirmation)
                            .font(.plexSans(13))
                            .foregroundColor(LognDark.textSecondary)
                            .multilineTextAlignment(.leading)
                    }
                }

                // Os links ficam fora do botão da caixa: dentro dele, o toque no link
                // marcaria a caixa em vez de abrir o documento.
                LegalLinksRow(presented: $legalSheet)

                if termsAccepted && !core.viewModel.legalVersionsReady {
                    Text(Str.Register.legal_pending)
                        .font(.plexSans(13, relativeTo: .footnote))
                        .foregroundColor(LognDark.textMuted)
                }
            }
            .padding(.top, 4)
            .padding(.bottom, 8)
            
            Button(action: {
                core.dispatch(event: LogN.Event.requestOtp(email: email, purpose: "verify_email"))
                step = .otp
            }) {
                Text(sendCodeLabel)
                    .font(.plexSansSemiBold(15))
                    .monospacedDigit()
                    .foregroundColor(canSendCode ? LognDark.onAccent : LognDark.textDim)
                    .frame(maxWidth: .infinity)
                    .frame(height: 52)
                    .background(canSendCode ? LognDark.accent : LognDark.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canSendCode || core.viewModel.isAuthenticating)
            .padding(.top, 4)
        }
    }
    
    private var otpStep: some View {
        OTPInputView(email: email, purpose: "verify_email", code: $otpCode)
            .environmentObject(core)
    }
    
    private var passwordStep: some View {
        VStack(spacing: Space.sm) {
            SecureField(Str.Register.password_prompt, text: $password)
                .textContentType(.newPassword)
                .font(.plexMono(14))
                .foregroundColor(LognDark.textPrimary)
                .padding(.horizontal, 14)
                .frame(height: 52)
                .background(LognDark.surface)
                .cornerRadius(Radius.sm)
                .overlay(
                    RoundedRectangle(cornerRadius: Radius.sm)
                        .stroke(LognDark.line, lineWidth: 1)
                )
            
            SecureField(Str.Register.confirm_prompt, text: $confirmPassword)
                .textContentType(.newPassword)
                .font(.plexMono(14))
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
                // O código que o jogador digitou, não o e-mail: ia `otpEmail` aqui, e
                // "Criar Conta" chegava ao servidor com o endereço no campo do OTP.
                // Quais versões e em que língua, o Core decide com o que o servidor disse.
                core.dispatch(event: .register(email: email, password: password, otp: otpCode, ageConfirmed: ageConfirmed, legalAccepted: termsAccepted))
            }) {
                Text(accountLocked
                     ? Str.Status.wait_seconds(Int(core.viewModel.authCooldownSeconds))
                     : Str.Register.create_account)
                    .font(.plexSansSemiBold(15))
                    .monospacedDigit()
                    .foregroundColor(canRegister ? LognDark.onAccent : LognDark.textDim)
                    .frame(maxWidth: .infinity)
                    .frame(height: 52)
                    .background(canRegister ? LognDark.accent : LognDark.buttonDisabled)
                    .cornerRadius(Radius.sm)
            }
            .disabled(!canRegister || core.viewModel.isAuthenticating)
            .padding(.top, 4)
        }
    }
    
    // MARK: - Helpers
    
    private var stepTitle: String {
        switch step {
        case .email: return Str.Register.create_account
        case .otp: return Str.Register.verify_email
        case .password: return Str.Register.set_password
        }
    }
    
    private var stepSubtitle: String {
        switch step {
        case .email: return Str.Register.reason_email
        case .otp: return Str.Register.reason_otp(email)
        case .password: return Str.Register.reason_password
        }
    }
    
    private var passwordsMatch: Bool {
        confirmPassword.isEmpty || password == confirmPassword
    }
    
    private var canRegister: Bool {
        password.count >= 8 && password == confirmPassword && !accountLocked
    }

    /// Travados por um 429: o servidor mandou esperar, e o botão conta o tempo.
    private var accountLocked: Bool { core.viewModel.authCooldownSeconds > 0 }
    private var sendCodeLocked: Bool { core.viewModel.resendCooldownSeconds > 0 }

    /// Sem as versões vigentes não há o que aceitar: mandar o código levaria a um
    /// cadastro que o servidor recusa depois de gastar o código.
    private var canSendCode: Bool {
        email.contains("@") && !sendCodeLocked && ageConfirmed && termsAccepted && core.viewModel.legalVersionsReady
    }

    private var sendCodeLabel: String {
        if sendCodeLocked { return Str.Status.wait_seconds(Int(core.viewModel.resendCooldownSeconds)) }
        return core.viewModel.isAuthenticating ? Str.Register.sending : Str.Register.send_code
    }
}
