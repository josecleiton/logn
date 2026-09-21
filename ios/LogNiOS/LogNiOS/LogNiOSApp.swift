import SwiftUI

@main
struct LogNiOSApp: App {
    @StateObject private var core = CoreWrapper()
    
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
