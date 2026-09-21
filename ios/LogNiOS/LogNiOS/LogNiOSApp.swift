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
            if core.viewModel.hasAccessToken || core.viewModel.isGuest {
                ContentView()
                    .environmentObject(core)
            } else {
                LoginView()
                    .environmentObject(core)
            }
        }
    }
}
