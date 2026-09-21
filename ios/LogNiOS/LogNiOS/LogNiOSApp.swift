import SwiftUI
import LogN
import PostHog

@main
struct LogNiOSApp: App {
    @StateObject private var core = CoreWrapper()
    
    init() {
        if let key = Bundle.main.object(forInfoDictionaryKey: "LogNTelemetryKey") as? String, !key.isEmpty {
            let config = PostHogConfig(apiKey: key, host: "https://app.posthog.com")
            config.captureApplicationLifecycleEvents = true
            PostHogSDK.shared.setup(config)
        } else {
            print("PostHog telemetry is disabled (no key provided)")
        }
    }
    
    @State private var resetPasswordEmail: String?
    @State private var resetPasswordCode: String?
    @State private var showResetPassword = false
    
    var body: some Scene {
        WindowGroup {
            Group {
                if core.viewModel.hasAccessToken || core.viewModel.isGuest {
                    ContentView()
                        .environmentObject(core)
                } else {
                    LoginView()
                        .environmentObject(core)
                }
            }
            .onOpenURL { url in
                handleIncomingURL(url)
            }
            .sheet(isPresented: $showResetPassword) {
                if let email = resetPasswordEmail, let code = resetPasswordCode {
                    ResetPasswordView(email: email, otp: code)
                        .environmentObject(core)
                }
            }
            .onChange(of: core.viewModel.hasAccessToken) { hasToken in
                if hasToken {
                    showResetPassword = false
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
                resetPasswordEmail = email
                resetPasswordCode = code
                showResetPassword = true
            }
        }
    }
}

