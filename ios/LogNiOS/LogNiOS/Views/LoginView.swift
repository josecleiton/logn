import SwiftUI
import LogN

struct LoginView: View {
    @EnvironmentObject var core: CoreWrapper
    
    @State private var email = ""
    @State private var password = ""
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: 0) {
                Spacer().frame(height: 60)
                
                // Brand Header
                VStack(spacing: 12) {
                    HStack(alignment: .center, spacing: 12) {
                        // Symbol
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
                    
                    Text("REDUCE THE COMPLEXITY OF YOUR SOLUTIONS")
                        .font(.custom("IBMPlexMono-Regular", size: 11))
                        .tracking(0.16 * 11) // letter-spacing: 0.16em
                        .foregroundColor(LognDark.textSecondary)
                }
                
                Spacer().frame(height: 40)
                
                // Content
                VStack(spacing: Space.sm) {
                    // SSO Placeholders (Apple, Google, GitHub)
                    let ssoData = [
                        ("applelogo", "Continuar com a Apple"),
                        ("g.circle.fill", "Continuar com o Google"),
                        ("curlybraces", "Continuar com o GitHub")
                    ]
                    
                    ForEach(ssoData, id: \.0) { item in
                        Button(action: {}) {
                            HStack(spacing: 12) {
                                Image(systemName: item.0)
                                    .font(.system(size: 20))
                                    .foregroundColor(LognDark.textPrimary)
                                
                                Text(item.1)
                                    .font(.system(size: 15, weight: .medium))
                                    .foregroundColor(LognDark.textPrimary)
                            }
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                        }
                    }
                    
                    // Divider
                    HStack(spacing: 12) {
                        Rectangle().fill(LognDark.line).frame(height: 1)
                        Text("OU COM E-MAIL")
                            .font(.custom("IBMPlexMono-Regular", size: 10))
                            .tracking(0.14 * 10)
                            .foregroundColor(LognDark.textMuted)
                        Rectangle().fill(LognDark.line).frame(height: 1)
                    }
                    .padding(.vertical, 8)
                    
                    // E-mail field
                    TextField("e-mail", text: $email)
                        .font(.custom("IBMPlexMono-Regular", size: 14))
                        .padding(.horizontal, 14)
                        .frame(height: 52)
                        .background(LognDark.surface)
                        .cornerRadius(Radius.sm)
                        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
                        .foregroundColor(LognDark.textPrimary)
                        .autocapitalization(.none)
                        .keyboardType(.emailAddress)
                        
                    // Password field
                    SecureField("senha", text: $password)
                        .font(.custom("IBMPlexMono-Regular", size: 14))
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
                            .font(.system(size: 15, weight: .semibold))
                            .foregroundColor(email.isEmpty || password.isEmpty ? LognDark.textDim : LognDark.surface)
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(email.isEmpty || password.isEmpty ? Color(hex: "1B1D20") : LognDark.accent)
                            .cornerRadius(Radius.sm)
                    }
                    .disabled(email.isEmpty || password.isEmpty || core.viewModel.isAuthenticating)
                    .padding(.top, 4)
                    
                    // Links
                    HStack {
                        NavigationLink(destination: RegisterView().environmentObject(core)) {
                            Text("Criar conta")
                                .font(.system(size: 13.5))
                                .foregroundColor(LognDark.textSecondary)
                        }
                        Spacer()
                        Button(action: {
                            if !email.isEmpty {
                                core.dispatch(event: .requestOtp(email: email, purpose: "reset_password"))
                            }
                        }) {
                            Text("Esqueci a senha")
                                .font(.system(size: 13.5))
                                .foregroundColor(LognDark.textSecondary)
                        }
                    }
                    .padding(.top, 6)
                }
                .padding(.horizontal, 20)
                
                if !core.viewModel.displayStatus.isEmpty {
                    Text(core.viewModel.displayStatus)
                        .font(.system(size: 12))
                        .foregroundColor(LognDark.wrong)
                        .padding(.top, 10)
                }
                
                Spacer()
                
                // Guest Button
                VStack(spacing: Space.md) {
                    Rectangle().fill(LognDark.line).frame(height: 1)
                    
                    Button(action: {
                        core.dispatch(event: .continueAsGuest)
                    }) {
                        Text("Jogar como visitante")
                            .font(.system(size: 14, weight: .medium))
                            .foregroundColor(LognDark.textPrimary)
                            .frame(height: 46)
                            .frame(maxWidth: .infinity)
                            .background(LognDark.canvas)
                            .cornerRadius(Radius.sm)
                            .overlay(
                                RoundedRectangle(cornerRadius: Radius.sm)
                                    .stroke(LognDark.line, style: StrokeStyle(lineWidth: 1, dash: [4]))
                            )
                    }
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 20)
            }
        }
        .navigationBarHidden(true)
    }
}
