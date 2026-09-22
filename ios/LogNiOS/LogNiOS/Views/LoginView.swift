import SwiftUI
import LogN

struct LoginView: View {
    @EnvironmentObject var core: CoreWrapper
    
    private let ssoData: [SSOProvider] = [
        SSOProvider(id: "Apple", title: "Continuar com a Apple", isApple: true),
        SSOProvider(id: "GoogleIcon", title: "Continuar com o Google", isApple: false),
        SSOProvider(id: "GitHubIcon", title: "Continuar com o GitHub", isApple: false)
    ]
    
    @State private var email = ""
    @State private var password = ""
    
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
                    
                    Text("REDUCE THE COMPLEXITY OF YOUR SOLUTIONS")
                        .font(.plexMono(11))
                        .tracking(0.16 * 11) // letter-spacing: 0.16em
                        .foregroundColor(LognDark.textSecondary)
                }
                
                Spacer().frame(height: 40)
                
                // Content
                VStack(spacing: Space.sm) {
                    // SSO (Apple, Google, GitHub)
                    Button(action: {}) {
                        HStack(spacing: 12) {
                            Image(systemName: "applelogo")
                                .font(.system(size: 20))
                                .foregroundColor(.black)
                            Text("Continuar com a Apple")
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
                            Text("Continuar com o Google")
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
                            Text("Continuar com o GitHub")
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
                        Text("OU COM E-MAIL")
                            .font(.plexMono(10))
                            .tracking(0.14 * 10)
                            .foregroundColor(LognDark.textMuted)
                        Rectangle().fill(LognDark.line).frame(height: 1)
                    }
                    .padding(.vertical, 8)
                    
                    // E-mail field
                    TextField("", text: $email, prompt: Text("e-mail").foregroundColor(LognDark.textDim))
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
                    SecureField("", text: $password, prompt: Text("senha").foregroundColor(LognDark.textDim))
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
                        Text(core.viewModel.isAuthenticating ? "Carregando..." : "Entrar")
                            .font(.plexSansSemiBold(15))
                            .foregroundColor(email.isEmpty || password.isEmpty ? LognDark.textDim : LognDark.onAccent)
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(email.isEmpty || password.isEmpty ? LognDark.buttonDisabled : LognDark.accent)
                            .cornerRadius(Radius.sm)
                    }
                    .disabled(email.isEmpty || password.isEmpty || core.viewModel.isAuthenticating)
                    .padding(.top, 4)
                    
                    // Links
                    HStack {
                        NavigationLink(destination: RegisterView().environmentObject(core)) {
                            Text("Criar conta")
                                .font(.plexSans(13.5))
                                .foregroundColor(LognDark.textSecondary)
                        }
                        Spacer()
                        Button(action: {
                            if !email.isEmpty {
                                core.dispatch(event: .requestOtp(email: email, purpose: "reset_password"))
                            }
                        }) {
                            Text("Esqueci a senha")
                                .font(.plexSans(13.5))
                                .foregroundColor(LognDark.textSecondary)
                        }
                    }
                    .padding(.top, 6)
                }
                .padding(.horizontal, 20)
                
                if !core.viewModel.displayStatus.isEmpty {
                    Text(core.viewModel.displayStatus)
                        .font(.plexSans(12))
                        .foregroundColor(LognDark.wrongInk)
                        .padding(.top, 10)
                }
                
                Spacer()
                
                // Guest Button & Core Status
                VStack(spacing: Space.md) {
                    Rectangle().fill(LognDark.line).frame(height: 1)
                    
                    Button(action: {
                        core.dispatch(event: .continueAsGuest)
                    }) {
                        VStack(spacing: 2) {
                            Text("Jogar como visitante")
                                .font(.plexSansSemiBold(14))
                                .foregroundColor(LognDark.textPrimary)
                            Text("sem salvar")
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
                    
                    // Core Status
                    HStack(spacing: 7) {
                        Circle()
                            .fill(LognDark.correct) // 3DD68C
                            .frame(width: 6, height: 6)
                        Text("CORE PRONTO")
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
    }
}

struct SSOProvider: Identifiable {
    let id: String
    let title: String
    let isApple: Bool
}
