import SwiftUI
import PostHog

@main
struct LogNiOSApp: App {
    @StateObject private var core = CoreWrapper()
    
    init() {
        let config = PostHogConfig(apiKey: "phc_mock_key_logn_telemetry")
        config.host = "https://app.posthog.com"
        config.captureApplicationLifecycleEvents = true
        PostHogSDK.shared.setup(config)
    }
    
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
        }
    }
    
    private func handleIncomingURL(_ url: URL) {
        guard url.scheme == "logn" else { return }
        
        let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
        guard let host = components?.host, let queryItems = components?.queryItems else { return }
        
        let code = queryItems.first(where: { $0.name == "code" })?.value ?? ""
        let email = queryItems.first(where: { $0.name == "email" })?.value ?? ""
        let purpose = queryItems.first(where: { $0.name == "purpose" })?.value ?? "verify_email"
        
        if host == "verify" || host == "reset-password" {
            if !code.isEmpty && !email.isEmpty {
                core.dispatch(event: .verifyOTP(email: email, code: code, purpose: purpose))
            }
        }
    }
}

