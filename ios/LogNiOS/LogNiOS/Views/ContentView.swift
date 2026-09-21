import SwiftUI
import LogN
import App

struct ContentView: View {
    @EnvironmentObject var core: CoreWrapper
    @State private var selectedTab = 0
    @AppStorage("isDarkMode") private var isDarkMode = true
    
    init() {
        // Setup TabBar Appearance for LogN Dark mode
        let appearance = UITabBarAppearance()
        appearance.configureWithOpaqueBackground()
        appearance.backgroundColor = UIColor(LognDark.surfaceRaised)
        
        // Font setup will fallback to system if IBM Plex is not bundled yet
        let font = UIFont(name: "IBMPlexSans-Medium", size: 10) ?? UIFont.systemFont(ofSize: 10, weight: .medium)
        
        let itemAppearance = UITabBarItemAppearance()
        itemAppearance.normal.iconColor = UIColor(LognDark.textMuted)
        itemAppearance.normal.titleTextAttributes = [.foregroundColor: UIColor(LognDark.textMuted), .font: font]
        
        itemAppearance.selected.iconColor = UIColor(LognDark.accent)
        itemAppearance.selected.titleTextAttributes = [.foregroundColor: UIColor(LognDark.accent), .font: font]
        
        appearance.stackedLayoutAppearance = itemAppearance
        appearance.inlineLayoutAppearance = itemAppearance
        appearance.compactInlineLayoutAppearance = itemAppearance
        
        UITabBar.appearance().standardAppearance = appearance
        if #available(iOS 15.0, *) {
            UITabBar.appearance().scrollEdgeAppearance = appearance
        }
    }
    
    var body: some View {
        TabView(selection: $selectedTab) {
            NavigationView {
                SkillTreeHostView()
                    .environmentObject(core)
            }
            .tabItem {
                Image(systemName: "point.3.connected.trianglepath.dotted")
                Text("TRILHAS")
            }
            .tag(0)
            
            NavigationView {
                ArenaHostView()
                    .environmentObject(core)
            }
            .tabItem {
                Image(systemName: "gamecontroller.fill")
                Text("ARENA")
            }
            .tag(1)
            
            NavigationView {
                StandingsHostView()
                    .environmentObject(core)
            }
            .tabItem {
                Image(systemName: "list.number")
                Text("PLACAR")
            }
            .tag(2)
        }
        .accentColor(LognDark.accent)
        .preferredColorScheme(isDarkMode ? .dark : .light)
    }
}

// MARK: - Trilhas
struct SkillTreeHostView: View {
    @EnvironmentObject var core: CoreWrapper
    
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            
            VStack(spacing: 0) {
                // Header (Mockup)
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("LogN")
                            .font(LognFont.headlineMedium)
                            .foregroundColor(LognDark.textPrimary)
                        Text(core.viewModel.isFetching ? "Atualizando mapa..." : "7 balões no ar")
                            .font(LognFont.label)
                            .foregroundColor(LognDark.textSecondary)
                    }
                    Spacer()
                    
                    // Profile Hub Trigger
                    NavigationLink(destination: ProfileHubView().environmentObject(core)) {
                        Circle()
                            .fill(LognDark.surfaceRaised)
                            .frame(width: 40, height: 40)
                            .overlay(
                                Image(systemName: "person.fill")
                                    .foregroundColor(LognDark.textSecondary)
                            )
                            .overlay(
                                Circle()
                                    .stroke(LognDark.line, lineWidth: 1)
                            )
                            .overlay(
                                // Sync indicator
                                Circle()
                                    .fill(core.viewModel.pendingSyncCount > 0 ? LognDark.warn : Color.clear)
                                    .frame(width: 10, height: 10)
                                    .offset(x: 12, y: -12),
                                alignment: .center
                            )
                    }
                }
                .padding(.horizontal, Space.screenMargin)
                .padding(.vertical, Space.md)
                .background(LognDark.surface)
                
                // Guest Warning
                if core.viewModel.isGuest {
                    HStack {
                        Image(systemName: "exclamationmark.triangle.fill")
                        Text(Str.Dashboard.sync_guest_warning)
                            .font(LognFont.label)
                    }
                    .foregroundColor(LognDark.onAccent)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, Space.sm)
                    .background(LognDark.warn)
                }
                
                // DAG View
                if core.viewModel.nodes.isEmpty {
                    Spacer()
                    VStack(spacing: 16) {
                        ProgressView().progressViewStyle(CircularProgressViewStyle(tint: LognDark.accent))
                        Text(core.viewModel.displayStatus)
                            .font(LognFont.bodyLarge)
                            .foregroundColor(LognDark.textSecondary)
                        
                        LognButton(title: "Tentar Novamente", variant: .secondary) {
                            core.dispatch(event: .fetchNodes)
                        }
                    }
                    Spacer()
                } else {
                    SkillTreeView(nodes: core.viewModel.nodes)
                }
            }
        }
        .navigationBarHidden(true)
    }
}

// MARK: - Arena Placeholder
struct ArenaHostView: View {
    @EnvironmentObject var core: CoreWrapper
    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            VStack(spacing: Space.lg) {
                Image(systemName: "flag.checkered.2.crossed")
                    .font(.system(size: 48))
                    .foregroundColor(LognDark.textDim)
                Text("ARENA LOGN")
                    .font(LognFont.headlineMedium)
                    .foregroundColor(LognDark.textPrimary)
                Text("Simulações de Maratona ICPC cronometradas ficarão disponíveis aqui.")
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 32)
            }
        }
        .navigationBarHidden(true)
    }
}

// MARK: - Placar Placeholder
struct StandingsHostView: View {
    @EnvironmentObject var core: CoreWrapper
    var body: some View {
        StandingsView()
            .environmentObject(core)
            .navigationBarHidden(true)
    }
}
