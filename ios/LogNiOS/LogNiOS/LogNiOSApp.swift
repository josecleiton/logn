import SwiftUI
import LogNCoreFFI
import LogN
import PostHog

@main
struct LogNiOSApp: App {
    @StateObject private var core = CoreWrapper()
    @StateObject private var storeKit = StoreKitManager.shared
    
    init() {
        // Com a escolha do interruptor "Análise de uso" guardada no aparelho.
        Telemetry.start()
    }
    
    /// O link de redefinição que chegou, como um dado só.
    ///
    /// Eram três `@State` separados — e-mail, código e um booleano — com o sheet por
    /// `isPresented` e um `if let` dentro. O SwiftUI monta o conteúdo antes das outras
    /// duas mudanças chegarem, o `if let` via `nil` e o sheet abria **em branco**.
    /// `sheet(item:)` existe exatamente para isso: o conteúdo nasce do dado.
    private struct ResetLink: Identifiable {
        let email: String
        let code: String
        var id: String { email + code }
    }

    @State private var resetLink: ResetLink?
    
    var body: some Scene {
        WindowGroup {
            Group {
                // A abertura na frente de tudo, até a última verificação fechar.
                if core.viewModel.boot.inProgress {
                    SplashView()
                        .environmentObject(core)
                        .transition(.opacity)
                // Sessão, não credencial: quem abre o app sem rede com a sessão dentro
                // do prazo entra no jogo, não na tela de login.
                } else if core.viewModel.hasSession || core.viewModel.isGuest {
                    ContentView()
                        .environmentObject(core)
                        .environmentObject(storeKit)
                        .overlay(alignment: .bottom) {
                            // A exclusão pedida foi cancelada por este login.
                            if core.viewModel.accountRestoredNotice {
                                AccountRestoredCard().environmentObject(core)
                            }
                        }
                } else if core.viewModel.deletionPurgeAfter > 0 {
                    // Acabou de pedir a exclusão: diz até quando entrar ainda recupera.
                    DeletionNoticeView()
                        .environmentObject(core)
                } else if core.wantsRegistration {
                    // Visitante que escolheu salvar o progresso cai direto no cadastro.
                    RegisterView()
                        .environmentObject(core)
                } else if core.viewModel.justLoggedOut {
                    // Saiu agora: tela de despedida com desfazer, no lugar do alerta.
                    LogoutNoticeView()
                        .environmentObject(core)
                } else {
                    LoginView()
                        .environmentObject(core)
                }
            }
            // A splash sai em fade: sem duração mínima, uma abertura rápida sem ele vira
            // um piscar.
            .animation(.easeOut(duration: 0.25), value: core.viewModel.boot.inProgress)
            .onOpenURL { url in
                handleIncomingURL(url)
            }
            .onAppear {
                // A escuta da loja começa com o app, não com a tela de compra: a
                // transação que ficou sem confirmar volta na abertura (spec, seção 5).
                storeKit.attach(core: core)
                #if DEBUG
                // Atalho de inspeção visual: `simctl launch … -LogNStartAsGuest 1` entra
                // direto no app. Existe para conferir tela contra o design system sem
                // depender de automação de toque; não muda nada em Release.
                if ProcessInfo.processInfo.arguments.contains("-LogNStartAsGuest"),
                   !core.viewModel.isGuest, !core.viewModel.hasSession {
                    core.dispatch(event: .continueAsGuest)
                }
                #endif
            }
            .sheet(item: $resetLink) { link in
                ResetPasswordView(email: link.email, otp: link.code)
                    .environmentObject(core)
            }
            .onChange(of: core.viewModel.hasAccessToken) { hasToken in
                if hasToken {
                    resetLink = nil
                }
            }
            // Quem já estava logado não vê token chegar, então a tela de redefinição
            // ficava aberta depois de salvar. O Core avisa que a senha trocou.
            .onChange(of: core.viewModel.passwordResetDone) { done in
                if done {
                    resetLink = nil
                    core.dispatch(event: .dismissPasswordReset)
                }
            }
        }
    }
    
    private func handleIncomingURL(_ url: URL) {
        guard url.scheme == "logn" else { return }
        
        let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
        guard let host = components?.host, let queryItems = components?.queryItems else { return }
        
        let code = queryItems.first(where: { $0.name == "code" })?.value ?? ""
        let email = queryItems.first(where: { $0.name == "email" })?.value ?? ""
        let purpose = queryItems.first(where: { $0.name == "purpose" })?.value ?? "verify_email"
        
        if host == "verify" {
            if !code.isEmpty && !email.isEmpty {
                core.dispatch(event: LogN.Event.verifyOtp(email: email, code: code, purpose: purpose))
            }
        } else if host == "reset-password" {
            if !code.isEmpty && !email.isEmpty {
                // Ao invés de apenas validar, já subimos a tela para o usuário digitar a nova senha
                resetLink = ResetLink(email: email, code: code)
            }
        }
    }
}

