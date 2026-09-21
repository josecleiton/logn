import SwiftUI
import AuthenticationServices
import App

struct LoginView: View {
    @EnvironmentObject var core: CoreWrapper
    @State private var email = ""
    @State private var password = ""
    
    var body: some View {
        NavigationView {
            ZStack {
                LognDark.canvas.ignoresSafeArea()
                
                VStack(spacing: Space.xl) {
                    Spacer()
                    
                    Text(Str.App.name)
                        .font(.system(size: 48, weight: .bold, design: .monospaced))
                        .foregroundColor(LognDark.textPrimary)
                    
                    Text(Str.App.subtitle)
                        .font(LognFont.bodyLarge)
                        .foregroundColor(LognDark.textSecondary)
                    
                    Spacer()
                    
                    VStack(spacing: Space.md) {
                        // Native Apple Sign In Button
                        SignInWithAppleButton(
                            .signIn,
                            onRequest: { request in
                                request.requestedScopes = [.fullName, .email]
                            },
                            onCompletion: { result in
                                switch result {
                                case .success(let authorization):
                                    if let appleIDCredential = authorization.credential as? ASAuthorizationAppleIDCredential,
                                       let identityToken = appleIDCredential.identityToken,
                                       let tokenString = String(data: identityToken, encoding: .utf8) {
                                        // TODO: Pass native provider token to Crux/Go
                                        print("Apple Sign In Success! Token: \(tokenString.prefix(10))...")
                                    }
                                case .failure(let error):
                                    print("Apple Sign In Failed: \(error.localizedDescription)")
                                }
                            }
                        )
                        .signInWithAppleButtonStyle(.white) // Strict Apple requirement for Dark Mode backgrounds
                        .frame(height: 50)
                        .cornerRadius(Radius.sm)
                        
                        Button(action: {
                            // TODO: Trigger native Google SDK
                        }) {
                            HStack {
                                Image(systemName: "g.circle.fill")
                                Text(Str.Login.sign_in_google)
                            }
                            .font(.system(size: 18, weight: .semibold))
                            .frame(maxWidth: .infinity)
                            .frame(height: 50)
                            .background(Color.white)
                            .foregroundColor(Color.black)
                            .cornerRadius(Radius.sm)
                        }
                        
                        Button(action: {
                            // TODO: Trigger Github custom tab
                        }) {
                            HStack {
                                Image(systemName: "apple.terminal.fill")
                                Text(Str.Login.sign_in_github)
                            }
                            .font(.system(size: 18, weight: .semibold))
                            .frame(maxWidth: .infinity)
                            .frame(height: 50)
                            .background(Color(white: 0.15))
                            .foregroundColor(Color.white)
                            .cornerRadius(Radius.sm)
                        }
                    }
                    .padding(.horizontal, Space.screenMargin)
                    
                    HStack {
                        VStack { Divider().background(LognDark.line) }
                        Text(Str.Login.or_separator)
                            .font(LognFont.label)
                            .foregroundColor(LognDark.textMuted)
                        VStack { Divider().background(LognDark.line) }
                    }
                    .padding(.horizontal, Space.screenMargin)
                    
                    VStack(spacing: Space.md) {
                        TextField("Email", text: $email)
                            .padding(Space.md)
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .foregroundColor(LognDark.textPrimary)
                            .keyboardType(.emailAddress)
                            .autocapitalization(.none)
                        
                        SecureField("Password", text: $password)
                            .padding(Space.md)
                            .background(LognDark.surface)
                            .cornerRadius(Radius.sm)
                            .foregroundColor(LognDark.textPrimary)
                        
                        Button(action: {
                            core.dispatch(event: .login(email: email, passwordHash: password))
                        }) {
                            if core.viewModel.isAuthenticating {
                                ProgressView()
                                    .progressViewStyle(CircularProgressViewStyle(tint: LognDark.onAccent))
                                    .frame(maxWidth: .infinity)
                                    .padding(.vertical, Space.md)
                                    .background(LognDark.accent)
                                    .cornerRadius(Radius.sm)
                            } else {
                                Text(Str.Login.action_sign_in)
                                    .font(LognFont.titleMedium)
                                    .frame(maxWidth: .infinity)
                                    .padding(.vertical, Space.md)
                                    .background(LognDark.accent)
                                    .foregroundColor(LognDark.onAccent)
                                    .cornerRadius(Radius.sm)
                            }
                        }
                        .disabled(core.viewModel.isAuthenticating || email.isEmpty || password.isEmpty)
                        
                        // Status Box just to show the login status
                        if !core.viewModel.displayStatus.isEmpty {
                        Button(action: {
                            core.dispatch(event: .continueAsGuest)
                        }) {
                            Text(Str.Login.continue_as_guest)
                                .font(LognFont.bodyLarge)
                                .foregroundColor(LognDark.textSecondary)
                                .padding(.top, Space.sm)
                                .underline()
                        }
                            Text(core.viewModel.displayStatus)
                                .font(LognFont.label)
                                .foregroundColor(LognDark.info)
                                .multilineTextAlignment(.center)
                                .padding(.top, Space.sm)
                        }
                    }
                    .padding(.horizontal, Space.screenMargin)
                    
                    Spacer()
                }
            }
            .colorScheme(.dark)
        }
    }
}

struct LoginView_Previews: PreviewProvider {
    static var previews: some View {
        LoginView().environmentObject(CoreWrapper())
    }
}
