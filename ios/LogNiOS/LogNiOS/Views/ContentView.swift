import SwiftUI
import LogN
import App

struct ContentView: View {
    @EnvironmentObject var core: CoreWrapper
    @State private var tab: LognTab = LognTab.launchOverride ?? .trilhas

    var body: some View {
        // A navegação mora dentro de cada tela raiz, não em volta delas: quando a
        // partida é empurrada, ela toma a tela inteira e a barra sai junto — que é o
        // comportamento que o documento de gameplay mostra.
        Group {
            switch tab {
            case .trilhas:
                NavigationStack { SkillTreeHostView(tab: $tab).environmentObject(core) }
            case .arena:
                NavigationStack { ArenaHostView(tab: $tab).environmentObject(core) }
            case .placar:
                NavigationStack { StandingsHostView(tab: $tab).environmentObject(core) }
            }
        }
        .tint(LognDark.accent)
        // Dark-first e, por ora, dark-only: `LognLight` existe nos tokens mas nenhum
        // mock desenha o app em claro, então o app não oferece a escolha.
        .preferredColorScheme(.dark)
    }
}

// MARK: - Barra de navegação

enum LognTab: CaseIterable {
    case trilhas, arena, placar

    var title: String {
        switch self {
        case .trilhas: return "TRILHAS"
        case .arena:   return "ARENA"
        case .placar:  return "PLACAR"
        }
    }

    /// Aba inicial forçada por argumento de lançamento, só em DEBUG:
    /// `simctl launch … -LogNStartTab placar`. Serve para conferir tela contra o
    /// design system sem depender de automação de toque.
    static var launchOverride: LognTab? {
        #if DEBUG
        let args = ProcessInfo.processInfo.arguments
        guard let i = args.firstIndex(of: "-LogNStartTab"), i + 1 < args.count else { return nil }
        switch args[i + 1] {
        case "trilhas": return .trilhas
        case "arena":   return .arena
        case "placar":  return .placar
        default:        return nil
        }
        #else
        return nil
        #endif
    }

    /// Tela empilhada a abrir no lançamento, só em DEBUG: `-LogNStartScreen telao`.
    static var launchScreen: String? {
        #if DEBUG
        let args = ProcessInfo.processInfo.arguments
        guard let i = args.firstIndex(of: "-LogNStartScreen"), i + 1 < args.count else { return nil }
        return args[i + 1]
        #else
        return nil
        #endif
    }
}

/// Avatar de entrada do perfil: círculo 40dp com a inicial em `accentInk` sobre accent @18%,
/// borda accent, e badge `warn` de 11dp quando há evento na fila de sync.
struct ProfileAvatar: View {
    /// E-mail ou nome; só a primeira letra aparece.
    let initial: String
    var size: CGFloat = 40
    var hasPending: Bool = false

    private var letter: String {
        String(initial.first.map(Character.init) ?? "?").uppercased()
    }

    var body: some View {
        Circle()
            .fill(LognDark.accent.opacity(0.18))
            .frame(width: size, height: size)
            .overlay(Circle().stroke(LognDark.accent, lineWidth: 1))
            .overlay(
                Text(letter)
                    .font(.plexSansSemiBold(size * 15 / 40))
                    .foregroundColor(LognDark.accentInk)
            )
            .overlay(alignment: .topTrailing) {
                if hasPending {
                    Circle()
                        .fill(LognDark.warn)
                        .frame(width: 11, height: 11)
                        .overlay(Circle().stroke(LognDark.canvas, lineWidth: 2))
                        .offset(x: 1, y: -1)
                }
            }
    }
}

/// Bottom nav de 3 itens — exploração `4b`.
///
/// Rótulos em `label` (Plex Mono 11, tracking +0.1em) sobre `surface`, divisor superior
/// em `line`, e o item ativo em `accent` com uma borda superior de 2dp que cobre o divisor.
///
/// É desenhada à mão porque a `TabView` do iOS 26 renderiza uma cápsula flutuante e
/// ignora a geometria que o design pede.
struct LognBottomNav: View {
    @Binding var selection: LognTab

    var body: some View {
        HStack(spacing: 0) {
            ForEach(LognTab.allCases, id: \.self) { item in
                let isActive = item == selection

                Button {
                    selection = item
                } label: {
                    Text(item.title)
                        .font(.plexMono(10.5))
                        .tracking(0.1 * 10.5)
                        .foregroundColor(isActive ? LognDark.accentInk : LognDark.textMuted)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 13)
                        .contentShape(Rectangle())
                        .overlay(alignment: .top) {
                            if isActive {
                                Rectangle()
                                    .frame(height: 2)
                                    .foregroundColor(LognDark.accent)
                                    .offset(y: -1)
                            }
                        }
                }
                .buttonStyle(.plain)
                .accessibilityAddTraits(isActive ? [.isSelected] : [])
            }
        }
        .background(LognDark.surface)
        .overlay(alignment: .top) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }
}

// MARK: - Trilhas

struct SkillTreeHostView: View {
    @EnvironmentObject var core: CoreWrapper
    @Binding var tab: LognTab
    @State private var showsProfile = LognTab.launchScreen == "perfil"

    /// "N balões no ar" — o contador é de nós conquistados, não de problemas aceitos.
    private var balloonsUp: Int {
        core.viewModel.nodes.filter { $0.status == .completed }.count
    }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                header

                if core.viewModel.isGuest {
                    guestWarning
                }

                if core.viewModel.nodes.isEmpty {
                    emptyState
                } else {
                    SkillTreeView(nodes: core.viewModel.nodes)
                }

                LognBottomNav(selection: $tab)
            }
        }
        .navigationBarHidden(true)
        .onAppear(perform: loadNodesIfNeeded)
        .onChange(of: core.viewModel.hasAccessToken) { _ in loadNodesIfNeeded() }
        .sheet(isPresented: $showsProfile) {
            // Detent, grabber e cantos são do próprio hub: ele se mede.
            ProfileHubView()
                .environmentObject(core)
        }
    }

    /// Busca a árvore assim que a sessão existe.
    ///
    /// Nada disparava `fetchNodes` depois do login: entrar autenticado caía no estado
    /// vazio e só o "Tentar de novo" carregava o mapa. O visitante não entra aqui porque
    /// o Core já lhe entrega os nós locais.
    private func loadNodesIfNeeded() {
        guard core.viewModel.hasAccessToken,
              core.viewModel.nodes.isEmpty,
              !core.viewModel.isFetching else { return }
        core.dispatch(event: .fetchNodes)
    }

    private var header: some View {
        HStack(spacing: 12) {
            Text(core.viewModel.isFetching ? "Atualizando mapa" : "\(balloonsUp) balões no ar")
                .font(.plexSansSemiBold(20, relativeTo: .title3))
                .tracking(-0.02 * 20)
                .foregroundColor(LognDark.textPrimary)

            Spacer(minLength: 0)

            Text("\(core.viewModel.globalXp) XP")
                .font(.plexMono(12))
                .monospacedDigit()
                .foregroundColor(LognDark.accentInk)

            // O perfil sobe como sheet sobre a árvore — ela fica visível atrás e o
            // jogador não perde o lugar.
            Button { showsProfile = true } label: {
                ProfileAvatar(
                    initial: core.viewModel.isGuest ? "?" : core.viewModel.accountEmail,
                    size: 40,
                    hasPending: core.viewModel.pendingSyncCount > 0
                )
                // O círculo tem 40dp e a área de toque saía ainda menor que isso.
                // A borda do desenho continua em 40; só o alvo cresce para os 44
                // mínimos, invisível, como o DS pede para todo controle.
                .frame(width: 44, height: 44)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Perfil")
        }
        .padding(.horizontal, 20)
        .padding(.top, 16)
        .padding(.bottom, 12)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    private var guestWarning: some View {
        HStack(spacing: Space.sm) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.system(size: 11))
            Text(Str.Dashboard.sync_guest_warning)
                .lognLabel()
        }
        .foregroundColor(LognDark.onAccent)
        .frame(maxWidth: .infinity)
        .padding(.vertical, Space.sm)
        .background(LognDark.warn)
    }

    /// Loading é skeleton, não spinner; vazio é uma frase e o CTA que resolve.
    private var emptyState: some View {
        VStack(spacing: Space.lg) {
            Spacer()
            if core.viewModel.isFetching {
                VStack(spacing: Space.md) {
                    ForEach(0..<3, id: \.self) { _ in
                        RoundedRectangle(cornerRadius: Radius.sm)
                            .fill(LognDark.surface)
                            .frame(height: 64)
                            .overlay(
                                RoundedRectangle(cornerRadius: Radius.sm)
                                    .stroke(LognDark.line, lineWidth: 1)
                            )
                    }
                }
                .padding(.horizontal, Space.screenMargin)
            } else {
                // O status do Core é diagnóstico interno e vem em inglês — "Session
                // refreshed!" aparecia aqui como se fosse a explicação do mapa vazio.
                Text("Não consegui carregar o mapa das trilhas.")
                    .font(LognFont.bodyMedium)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, Space.screenMargin)

                LognButton(title: "Tentar de novo", variant: .secondary) {
                    core.dispatch(event: .fetchNodes)
                }
                .padding(.horizontal, Space.screenMargin)
            }
            Spacer()
        }
    }
}

// MARK: - Arena

struct ArenaHostView: View {
    @EnvironmentObject var core: CoreWrapper
    @Binding var tab: LognTab

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()
            VStack(spacing: 0) {
                Spacer()
                VStack(alignment: .leading, spacing: Space.md) {
                    Text("ARENA")
                        .lognLabel()
                        .foregroundColor(LognDark.textMuted)

                    Text("Contests cronometrados entram aqui.")
                        .font(.plexSansSemiBold(21))
                        .tracking(-0.02 * 21)
                        .foregroundColor(LognDark.textPrimary)

                    Text("Por enquanto, as sessões de três minutos vivem nas trilhas.")
                        .font(LognFont.bodyMedium)
                        .foregroundColor(LognDark.textSecondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, Space.screenMargin)
                Spacer()

                LognBottomNav(selection: $tab)
            }
        }
        .navigationBarHidden(true)
    }
}

// MARK: - Placar

struct StandingsHostView: View {
    @EnvironmentObject var core: CoreWrapper
    @Binding var tab: LognTab

    /// Só em DEBUG: `-LogNStartScreen telao` abre o telão direto, para inspeção visual.
    @State private var showsScoreboard = LognTab.launchScreen == "telao"

    var body: some View {
        VStack(spacing: 0) {
            StandingsView().environmentObject(core)
            LognBottomNav(selection: $tab)
        }
        .navigationBarHidden(true)
        .navigationDestination(isPresented: $showsScoreboard) {
            ScoreboardView().environmentObject(core)
        }
    }
}
