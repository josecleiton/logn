import SwiftUI
import LogNCoreFFI
import LogN
import PostHog

struct LoginView: View {
    @EnvironmentObject var core: CoreWrapper
    

    
    @State private var email = ""
    @State private var password = ""
    @State private var githubEnabled = false
    @State private var googleEnabled = false
    @State private var appleEnabled = false
    @State private var legalSheet: LegalKind? = LegalKind.launchOverride
    /// Só em DEBUG: `-LogNStartScreen cadastro` abre o cadastro direto. O toque
    /// sintético no link depende da janela do Simulator estar acessível, e nem sempre está.
    @State private var showsRegisterOnLaunch = LognTab.launchScreen == "cadastro"
    /// "Esqueci a senha" pede o código e já abre a tela onde ele é digitado. Antes só
    /// pedia: o e-mail chegava e o app não tinha onde usar o código.
    @State private var showsReset = false
    @FocusState private var emailFocused: Bool

    var body: some View {
        // Sem este container os `NavigationLink` daqui não empurram nada: "Criar conta"
        // e "Esqueci a senha" renderizavam como rótulo e não faziam nada.
        NavigationStack {
            content
        }
    }

    private var content: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                BalloonMarquee()
                    .padding(.top, 14)
                
                Spacer().frame(height: 35)
                
                // Brand Header
                VStack(spacing: 12) {
                    BrandLockup(fontSize: 40)
                    
                    Text(Str.Login.slogan)
                        .font(.plexMono(11))
                        .tracking(0.16 * 11) // letter-spacing: 0.16em
                        .foregroundColor(LognDark.textSecondary)
                }
                
                Spacer().frame(height: 40)
                
                // Content
                VStack(spacing: Space.sm) {
                    // O botão branco é da diretriz da Apple; o design system o mantém assim.
                    if appleEnabled {
                        Button(action: signInWithApple) {
                            HStack(spacing: 12) {
                                Image(systemName: "applelogo")
                                    .font(.system(size: 20))
                                    .foregroundColor(.black)
                                Text(Str.Login.sign_in_apple)
                                    .font(.plexSansMedium(15))
                                    .foregroundColor(.black)
                            }
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(Color.white)
                            .cornerRadius(Radius.sm)
                            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(Color.clear, lineWidth: 1))
                        }
                        .disabled(core.viewModel.isAuthenticating || signInLocked)
                    }

                    if googleEnabled {
                        GoogleSignInButton(action: signInWithGoogle)
                            .disabled(core.viewModel.isAuthenticating || signInLocked)
                    }

                    if githubEnabled {
                        Button(action: signInWithGitHub) {
                            HStack(spacing: 12) {
                                Image("GitHubIcon")
                                    .renderingMode(.original)
                                    .resizable()
                                    .scaledToFit()
                                    .frame(width: 20, height: 20)
                                Text(Str.Login.sign_in_github)
                                    .font(.plexSansMedium(15))
                                    .foregroundColor(LognDark.textPrimary)
                            }
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                        }
                        .disabled(core.viewModel.isAuthenticating || signInLocked)
                    }

                    if githubEnabled || googleEnabled || appleEnabled {
                        // Divider
                        HStack(spacing: 12) {
                            Rectangle().fill(LognDark.line).frame(height: 1)
                            Text(Str.Login.or_email)
                                .font(.plexMono(10))
                                .tracking(0.14 * 10)
                                .foregroundColor(LognDark.textMuted)
                            Rectangle().fill(LognDark.line).frame(height: 1)
                        }
                        .padding(.vertical, 8)
                        
                        
                }
                
                // E-mail field
                    TextField("", text: $email, prompt: Text(Str.Login.email_prompt).foregroundColor(LognDark.textDim))
                        .font(.plexMono(14))
                        .padding(.horizontal, 14)
                        .frame(height: 52)
                        .background(LognDark.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                        .foregroundColor(LognDark.textPrimary)
                        .autocapitalization(.none)
                        .keyboardType(.emailAddress)
                        .focused($emailFocused)

                    // Password field
                    SecureField("", text: $password, prompt: Text(Str.Login.password_prompt).foregroundColor(LognDark.textDim))
                        .font(.plexMono(14))
                        .padding(.horizontal, 14)
                        .frame(height: 52)
                        .background(LognDark.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                        .foregroundColor(LognDark.textPrimary)
                    
                    // Submit button
                    Button(action: {
                        if !email.isEmpty && !password.isEmpty {
                            core.dispatch(event: .login(email: email, passwordHash: password))
                        }
                    }) {
                        Text(signInLabel)
                            .font(.plexSansSemiBold(15))
                            .foregroundColor(canSignIn ? LognDark.onAccent : LognDark.textDim)
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(canSignIn ? LognDark.accent : LognDark.buttonDisabled)
                            .cornerRadius(Radius.sm)
                    }
                    .disabled(!canSignIn || core.viewModel.isAuthenticating)
                    .padding(.top, 4)
                    
                    // Links
                    HStack {
                        NavigationLink(destination: RegisterView().environmentObject(core)) {
                            Text(Str.Login.create_account)
                                .font(.plexSans(13.5))
                                .foregroundColor(LognDark.textSecondary)
                        }
                        Spacer()
                        Button(action: requestReset) {
                            Text(resendLocked
                                 ? Str.Status.wait_seconds(Int(core.viewModel.resendCooldownSeconds))
                                 : Str.Login.forgot_password)
                                .font(.plexSans(13.5))
                                .monospacedDigit()
                                .foregroundColor(canRequestReset ? LognDark.textSecondary : LognDark.textDim)
                        }
                        .disabled(!canRequestReset)
                    }
                    .padding(.top, 6)
                }
                .padding(.horizontal, 20)
                
                StatusLine(status: core.viewModel.status)
                    .padding(.top, 10)
                
                Spacer()
                
                // Guest Button & Core Status
                VStack(spacing: Space.md) {
                    Rectangle().fill(LognDark.line).frame(height: 1)
                    
                    Button(action: {
                        core.dispatch(event: .continueAsGuest)
                    }) {
                        VStack(spacing: 2) {
                            Text(Str.Login.guest_mode)
                                .font(.plexSansSemiBold(14))
                                .foregroundColor(LognDark.textPrimary)
                            Text(Str.Login.without_saving)
                                .font(.plexMono(10.5))
                                .foregroundColor(LognDark.textMuted)
                        }
                        .frame(height: 46)
                        .frame(maxWidth: .infinity)
                        .background(LognDark.canvas)
                        .cornerRadius(Radius.sm)
                        .overlay(
                            RoundedRectangle(cornerRadius: Radius.sm)
                                .stroke(LognDark.line, style: StrokeStyle(lineWidth: 1, dash: [4]))
                        )
                    }
                    
                    LegalLinksRow(presented: $legalSheet)

                    // Core Status
                    HStack(spacing: 7) {
                        Circle()
                            .fill(LognDark.correct) // 3DD68C
                            .frame(width: 6, height: 6)
                        Text(Str.Login.core_ready)
                            .font(.plexMono(10))
                            .tracking(0.1 * 10)
                            .foregroundColor(LognDark.textMuted)
                    }
                    .padding(.top, 4)
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 20)
            }
        }
        .navigationBarHidden(true)
        .onAppear {
            readFlags()
            prefillResumeEmail(core.viewModel.resumeEmail)
        }
        // Na primeira abertura, e depois de sair (o `reset` troca o id), as flags chegam
        // depois de a tela aparecer: lidas só no `onAppear`, o botão nunca aparecia.
        .onReceive(NotificationCenter.default.publisher(for: PostHogSDK.didReceiveFeatureFlags)) { _ in
            readFlags()
        }
        .sheet(isPresented: socialSignupBinding) {
            SocialSignupSheet()
                .environmentObject(core)
                // Puxar para baixo com o pedido no ar descartava o login no meio.
                .interactiveDismissDisabled(core.viewModel.isAuthenticating)
        }
        // A sessão acabou: o login vem com o e-mail dela. Ele pode chegar depois de a
        // tela aparecer, porque o Core o lê do aparelho.
        .onChange(of: core.viewModel.resumeEmail) { prefillResumeEmail($0) }
        .sheet(item: $legalSheet) { kind in
            LegalDocumentView(kind: kind)
        }
        .navigationDestination(isPresented: $showsRegisterOnLaunch) {
            RegisterView().environmentObject(core)
        }
        .navigationDestination(isPresented: $showsReset) {
            ResetPasswordView(email: email, otp: "").environmentObject(core)
        }

    }

    /// Só preenche o campo vazio: o que a pessoa já digitou manda.
    private func prefillResumeEmail(_ resume: String) {
        if email.isEmpty && !resume.isEmpty {
            email = resume
        }
    }

    /// Travado por um 429: o servidor mandou esperar, e o botão conta o tempo.
    private var signInLocked: Bool { core.viewModel.authCooldownSeconds > 0 }
    private var resendLocked: Bool { core.viewModel.resendCooldownSeconds > 0 }
    /// Só o intervalo de reenvio trava o link. Sem e-mail ele continua tocável e diz o
    /// que falta: apagado, o toque não fazia nada e parecia defeito.
    private var canRequestReset: Bool { !resendLocked }

    private func requestReset() {
        guard !email.trimmingCharacters(in: .whitespaces).isEmpty else {
            ToastCenter.shared.show(.attention, Str.Login.email_first, id: "login.email_first")
            // Com VoiceOver, o foco espera o anúncio: mudar antes corta a fala.
            let delay = UIAccessibility.isVoiceOverRunning ? 1.5 : 0.06
            DispatchQueue.main.asyncAfter(deadline: .now() + delay) { emailFocused = true }
            return
        }
        core.dispatch(event: .requestOtp(email: email, purpose: "reset_password"))
        showsReset = true
    }

    private var canSignIn: Bool { !email.isEmpty && !password.isEmpty && !signInLocked }

    private func readFlags() {
        // A chave para desligar cada provedor sem versão nova: se o login quebrar do lado
        // dele, a flag some com o botão e quem entra por e-mail segue entrando.
        githubEnabled = GitHubAuth.shared.isConfigured
            && PostHogSDK.shared.isFeatureEnabled("sso_github_enabled")
        googleEnabled = GoogleAuth.shared.isConfigured
            && PostHogSDK.shared.isFeatureEnabled("sso_google_enabled")
        appleEnabled = AppleAuth.shared.isConfigured
            && PostHogSDK.shared.isFeatureEnabled("sso_apple_enabled")
    }

    private func signInWithApple() {
        Task {
            do {
                let credential = try await AppleAuth.shared.signIn()
                core.dispatch(event: .socialLogin(provider: "apple", idToken: credential.idToken, nonce: credential.nonce))
            } catch AppleAuth.Failure.cancelled {
                // Fechou a folha da Apple: nada a dizer.
            } catch {
                core.dispatch(event: .socialLoginFailed)
            }
        }
    }

    private func signInWithGoogle() {
        Task {
            do {
                let credential = try await GoogleAuth.shared.signIn()
                core.dispatch(event: .socialLogin(provider: "google", idToken: credential.idToken, nonce: credential.nonce))
            } catch GoogleAuth.Failure.cancelled {
                // Fechou a janela do Google: nada a dizer.
            } catch {
                core.dispatch(event: .socialLoginFailed)
            }
        }
    }

    private func signInWithGitHub() {
        Task {
            do {
                let credential = try await GitHubAuth.shared.signIn()
                // O Core troca o código pelo bilhete no servidor e segue o login.
                core.dispatch(event: .gitHubCodeReceived(code: credential.code, codeVerifier: credential.verifier, nonce: credential.nonce))
            } catch GitHubAuth.Failure.cancelled {
                // Fechou a janela do GitHub: nada a dizer.
            } catch {
                core.dispatch(event: .socialLoginFailed)
            }
        }
    }

    /// A tela de idade e termos abre quando o Core diz que a conta ainda não existe.
    /// Fechar puxando para baixo é o mesmo que "Agora não".
    private var socialSignupBinding: Binding<Bool> {
        Binding(
            get: { core.viewModel.socialSignupRequired },
            // Só fechar pela pessoa cancela. Quando o próprio Core fecha a tela (login
            // pronto ou erro), não há o que cancelar.
            set: { shown in
                if !shown && core.viewModel.socialSignupRequired {
                    core.dispatch(event: .cancelSocialSignup)
                }
            }
        )
    }

    private var signInLabel: String {
        if signInLocked { return Str.Status.wait_seconds(Int(core.viewModel.authCooldownSeconds)) }
        // Com a sessão aberta esta tela está saindo, e ainda aparece por um quadro: sem
        // isto o botão voltava a "Entrar" no caminho para o app.
        return core.viewModel.isAuthenticating || core.viewModel.hasSession ? Str.Login.loading : Str.Login.sign_in
    }
}

