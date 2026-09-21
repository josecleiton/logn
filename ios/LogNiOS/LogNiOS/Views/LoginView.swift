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
                // Mini Balloons Row
                HStack(spacing: 7) {
                    let colors: [Color] = [
                        Color(hex: "E4572E"), // A
                        Color(hex: "F5C451"), // B
                        Color(hex: "3DB2FF"), // C
                        Color(hex: "6BCB77"), // D
                        Color(hex: "C77DFF"), // E
                        Color(hex: "FF6FB5"), // F
                        Color(hex: "4ECDC4"), // G
                        Color(hex: "F4A261"), // H
                        Color(hex: "9BC53D"), // I
                        Color(hex: "D64550"), // J
                        Color(hex: "7C8BFF"), // K
                        Color(hex: "D8DEE4"), // L
                        Color(hex: "00B894")  // M
                    ]
                    ForEach(0..<13) { i in
                        BalloonShape(
                            color: colors[i],
                            state: i < 5 ? .filled : .outline,
                            bodySize: 9,
                            showString: false,
                            showHighlight: false
                        )
                    }
                }
                .opacity(0.5)
                .padding(.top, 14)
                
                Spacer().frame(height: 35)
                
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
                        ("Apple", "Continuar com a Apple", true),
                        ("GoogleIcon", "Continuar com o Google", false),
                        ("GitHubIcon", "Continuar com o GitHub", false)
                    ]
                    
                    ForEach(ssoData, id: \.1) { item in
                        Button(action: {}) {
                            HStack(spacing: 12) {
                                if item.2 {
                                    Image(systemName: "applelogo")
                                        .font(.system(size: 20))
                                        .foregroundColor(.black)
                                } else {
                                    Image(item.0)
                                        .renderingMode(.alwaysOriginal)
                                        .resizable()
                                        .scaledToFit()
                                        .frame(width: 20, height: 20)
                                }
                                
                                Text(item.1)
                                    .font(.system(size: 15, weight: .medium))
                                    .foregroundColor(item.2 ? .black : LognDark.textPrimary)
                            }
                            .frame(maxWidth: .infinity)
                            .frame(height: 52)
                            .background(item.2 ? Color.white : LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .overlay(
                                RoundedRectangle(cornerRadius: Radius.sm)
                                    .stroke(item.2 ? Color.clear : LognDark.line, lineWidth: 1)
                            )
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
                    TextField("", text: $email, prompt: Text("e-mail").foregroundColor(LognDark.textDim))
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
                    SecureField("", text: $password, prompt: Text("senha").foregroundColor(LognDark.textDim))
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
                
                // Guest Button & Core Status
                VStack(spacing: Space.md) {
                    Rectangle().fill(LognDark.line).frame(height: 1)
                    
                    Button(action: {
                        core.dispatch(event: .continueAsGuest)
                    }) {
                        VStack(spacing: 2) {
                            Text("Jogar como visitante")
                                .font(.system(size: 14, weight: .semibold))
                                .foregroundColor(LognDark.textPrimary)
                            Text("sem salvar")
                                .font(.custom("IBMPlexMono-Regular", size: 10.5))
                                .foregroundColor(LognDark.textDim)
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
                            .font(.custom("IBMPlexMono-Regular", size: 10))
                            .tracking(0.1 * 10)
                            .foregroundColor(LognDark.textDim)
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
