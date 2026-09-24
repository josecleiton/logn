import SwiftUI
import LogNCoreFFI
import LogN
import PostHog

struct LoginView: View {
    @EnvironmentObject var core: CoreWrapper
    

    
    @State private var email = ""
    @State private var password = ""
    @State private var ssoEnabled = false
    @State private var legalSheet: LegalKind? = LegalKind.launchOverride
    /// Só em DEBUG: `-LogNStartScreen cadastro` abre o cadastro direto. O toque
    /// sintético no link depende da janela do Simulator estar acessível, e nem sempre está.
    @State private var showsRegisterOnLaunch = LognTab.launchScreen == "cadastro"

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
                    if ssoEnabled {
                    // SSO (Apple, Google, GitHub)
                        Button(action: {}) {
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
                        
                        Button(action: {}) {
                            HStack(spacing: 12) {
                                Image("GoogleIcon")
                                    .renderingMode(.original)
                                    .resizable()
                                    .scaledToFit()
                                    .frame(width: 20, height: 20)
                                Text(Str.Login.sign_in_google)
                                    .font(.plexSansMedium(15))
                                    .foregroundColor(LognDark.textPrimary)
                            }
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                        }
                        
                        Button(action: {}) {
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
                        Button(action: {
                            if !email.isEmpty {
                                core.dispatch(event: .requestOtp(email: email, purpose: "reset_password"))
                            }
                        }) {
                            Text(resendLocked
                                 ? Str.Status.wait_seconds(Int(core.viewModel.resendCooldownSeconds))
                                 : Str.Login.forgot_password)
                                .font(.plexSans(13.5))
                                .monospacedDigit()
                                .foregroundColor(resendLocked ? LognDark.textDim : LognDark.textSecondary)
                        }
                        .disabled(resendLocked)
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
            self.ssoEnabled = PostHogSDK.shared.isFeatureEnabled("sso_enabled")
        }
        .sheet(item: $legalSheet) { kind in
            LegalDocumentView(kind: kind)
        }
        .navigationDestination(isPresented: $showsRegisterOnLaunch) {
            RegisterView().environmentObject(core)
        }

    }

    /// Travado por um 429: o servidor mandou esperar, e o botão conta o tempo.
    private var signInLocked: Bool { core.viewModel.authCooldownSeconds > 0 }
    private var resendLocked: Bool { core.viewModel.resendCooldownSeconds > 0 }

    private var canSignIn: Bool { !email.isEmpty && !password.isEmpty && !signInLocked }

    private var signInLabel: String {
        if signInLocked { return Str.Status.wait_seconds(Int(core.viewModel.authCooldownSeconds)) }
        return core.viewModel.isAuthenticating ? Str.Login.loading : Str.Login.sign_in
    }
}

