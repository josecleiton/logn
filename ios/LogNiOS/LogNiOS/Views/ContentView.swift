import SwiftUI
import LogNCoreFFI
import StoreKit
import LogN
import App

struct ContentView: View {
    @EnvironmentObject var core: CoreWrapper
    @EnvironmentObject var storeKit: StoreKitManager
    @State private var tab: LognTab = LognTab.launchOverride ?? .trilhas
    @State private var catalogRequested = false
    @Environment(\.requestReview) private var requestReview

    /// A tela do passo a passo da compra fica em cima de tudo enquanto houver compra.
    /// Não disputa com o onboarding: duas capas de tela cheia não abrem juntas.
    private var showsPurchase: Binding<Bool> {
        Binding(
            get: { core.viewModel.purchaseFlow.stage != .idle && !core.viewModel.showOnboarding },
            set: { shown in if !shown { core.dispatch(event: .closePurchaseFlow) } }
        )
    }

    var body: some View {
        // A navegação mora dentro de cada tela raiz, não em volta delas: quando a
        // partida é empurrada, ela toma a tela inteira e a barra sai junto — que é o
        // comportamento que o documento de gameplay mostra.
        //
        // O onboarding é a tela, não uma capa por cima dela. Como capa, ele abria no
        // mesmo instante em que o login saía, e a apresentação segurava o login na tela
        // por quase um segundo, já com o botão de volta a "Entrar".
        Group {
            if core.viewModel.showOnboarding {
                OnboardingView { wantsCatalog in
                    tab = .trilhas
                    catalogRequested = wantsCatalog
                }
                .environmentObject(core)
                .transition(.opacity)
            } else {
                tabRoot
            }
        }
        .animation(.easeOut(duration: 0.25), value: core.viewModel.showOnboarding)
        // Aqui, e não na árvore: com o onboarding na frente a árvore não existe, e o
        // cartão dele ficava sem o nome da trilha, que vem da mesma busca.
        .onAppear(perform: loadNodesIfNeeded)
        .onChange(of: core.viewModel.hasAccessToken) { _ in loadNodesIfNeeded() }
        .tint(LognDark.accent)
        // Dark-first e, por ora, dark-only: `LognLight` existe nos tokens mas nenhum
        // mock desenha o app em claro, então o app não oferece a escolha.
        .preferredColorScheme(.dark)
        // O Core emite StoreReview via notify_shell após dominar um nó de milestone.
        // O delay deixa a animação de pop do relatório terminar antes de a sheet aparecer.
        .onChange(of: core.reviewRequestCount) { _ in
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) {
                requestReview()
            }
        }
        .fullScreenCover(isPresented: showsPurchase) {
            PurchaseProgressView()
                .environmentObject(core)
        }
        .sheet(item: $storeKit.guestPrompt) { prompt in
            GuestPurchaseSheet(prompt: prompt)
                .environmentObject(core)
                .environmentObject(storeKit)
        }
    }

    /// Busca a árvore assim que a sessão existe.
    ///
    /// Nada disparava `fetchNodes` depois do login: entrar autenticado caía no estado
    /// vazio e só o "Tentar de novo" carregava o mapa. O visitante não entra aqui porque
    /// o Core já lhe entrega os nós locais.
    ///
    /// A árvore da semente também conta como "falta buscar": com ela na tela, a conta
    /// logada nunca falava com o servidor, e o catálogo e as licenças da trilha paga,
    /// que vêm atrás dos desafios, nunca chegavam.
    private func loadNodesIfNeeded() {
        guard core.viewModel.hasAccessToken,
              core.viewModel.nodes.isEmpty || core.viewModel.trailFromBundle,
              !core.viewModel.isFetching else { return }
        core.dispatch(event: .fetchNodes)
    }

    @ViewBuilder
    private var tabRoot: some View {
        Group {
            switch tab {
            case .trilhas:
                NavigationStack {
                    SkillTreeHostView(tab: $tab, catalogRequested: $catalogRequested)
                        .environmentObject(core)
                        .environmentObject(storeKit)
                }
            case .arena:
                NavigationStack { ArenaHostView(tab: $tab).environmentObject(core) }
            case .placar:
                NavigationStack { StandingsHostView(tab: $tab).environmentObject(core) }
            }
        }
    }
}

// MARK: - Barra de navegação

enum LognTab: CaseIterable {
    case trilhas, arena, placar

    var title: String {
        switch self {
        case .trilhas: return Str.Tabs.trails
        case .arena:   return Str.Tabs.arena
        case .placar:  return Str.Tabs.standings
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
    @EnvironmentObject var storeKit: StoreKitManager
    @Binding var tab: LognTab
    /// O onboarding pediu o catálogo ("Já sei o básico").
    @Binding var catalogRequested: Bool
    @State private var showsProfile = LognTab.launchScreen == "perfil"
    @State private var showsCatalog = LognTab.launchScreen == "catalogo"

    /// "N balões no ar" — o contador é de nós conquistados, não de problemas aceitos.
    private var balloonsUp: Int {
        core.viewModel.nodes.filter { $0.status == .completed }.count
    }

    private var track: TrackView { core.viewModel.currentTrack }

    var body: some View {
        ZStack {
            LognDark.canvas.ignoresSafeArea()

            VStack(spacing: 0) {
                header

                if core.viewModel.isGuest {
                    guestWarning
                } else if core.viewModel.isOfflineSession {
                    offlineWarning
                }

                // A tarja acima diz que o progresso não sobe; esta diz que o conteúdo
                // pode estar velho. São coisas diferentes e as duas podem valer ao mesmo
                // tempo — visitante sem rede vê as duas. Esta é a mais quieta das duas
                // de propósito: não é aviso, é ressalva.
                if core.viewModel.trailFromBundle {
                    bundledTrailNote
                }

                // Só a trilha vencida fecha, numa tela própria; a grátis e as amostras
                // seguem abertas (V4).
                if track.isExpired && !track.isFree {
                    ExpiredTrackView(track: track)
                    Spacer(minLength: 0)
                } else if core.viewModel.nodes.isEmpty {
                    emptyState
                } else {
                    SkillTreeView(nodes: core.viewModel.nodes)
                }

                LognBottomNav(selection: $tab)
            }
        }
        .navigationBarHidden(true)
        .navigationDestination(isPresented: $showsCatalog) {
            CatalogView(openTrack: openTrack)
                .environmentObject(core)
                .environmentObject(storeKit)
        }
        // O onboarding é a tela antes desta: o pedido do catálogo chega antes de a
        // árvore existir, e o `onChange` abaixo não dispara com o valor de partida.
        .onAppear { openCatalogIfRequested(catalogRequested) }
        .onChange(of: catalogRequested, perform: openCatalogIfRequested)
        // Trocou de trilha (compra aberta, amostra, "Voltar para"): a árvore volta à frente.
        .onChange(of: track.id) { _ in showsCatalog = false }
        .sheet(isPresented: $showsProfile) {
            // Detent, grabber e cantos são do próprio hub: ele se mede.
            ProfileHubView()
                .environmentObject(core)
        }
    }

    private func openCatalogIfRequested(_ requested: Bool) {
        guard requested else { return }
        showsCatalog = true
        catalogRequested = false
    }

    private func openTrack(_ id: String) {
        core.dispatch(event: .selectTrack(trackId: id))
        showsCatalog = false
    }

    /// A trilha na tela e o botão que abre o catálogo, com o selo de validade (V1).
    /// Sem catálogo ainda (primeira abertura sem rede), fica o contador de balões.
    private var header: some View {
        HStack(spacing: 10) {
            if track.name.isEmpty {
                Text(core.viewModel.isFetching ? Str.Dashboard.map_updating : Str.Dashboard.balloons_up(balloonsUp))
                    .font(.plexSansSemiBold(20, relativeTo: .title3))
                    .tracking(-0.02 * 20)
                    .foregroundColor(LognDark.textPrimary)
            } else {
                trackSwitcher
            }

            Spacer(minLength: 0)

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
            .accessibilityLabel(Str.Profile.guest_mode)
        }
        .padding(.horizontal, 20)
        .padding(.top, 16)
        .padding(.bottom, 12)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    /// "4 de 7 nós · 620 XP", mais "SEM REDE" quando a sessão está sem servidor.
    private var headerMeta: String {
        let progress = Str.Catalog.progress(Int(track.nodesDone), Int(track.nodeCount), Int(track.trackXp))
        return core.viewModel.isOfflineSession ? progress + " · " + Str.Catalog.offline : progress
    }

    /// O selo é pequeno e não explica: só o bastante para dar vontade de tocar (V1).
    private var badgeText: String? {
        let badge = core.viewModel.catalogBadge
        switch badge.state {
        case .soon: return Str.Catalog.days(Int(badge.daysLeft))
        case .today: return Str.Catalog.today
        case .expired: return Str.Catalog.connect
        default: return nil
        }
    }

    /// O nome da trilha é o próprio seletor: nome e chevron abrem o catálogo. O selo de
    /// validade fica colado ao chevron, no mesmo lugar do olhar. Diverge do V1 de
    /// propósito, que tinha um botão "Trilhas" à parte: o nome cortava em tela pequena.
    private var trackSwitcher: some View {
        let expired = core.viewModel.catalogBadge.state == .expired
        let badge = badgeText
        return Button { showsCatalog = true } label: {
            HStack(spacing: 10) {
                TrackBalloon(track: track, width: 16)
                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: 6) {
                        Text(track.name)
                            .font(.plexSansSemiBold(17, relativeTo: .headline))
                            .foregroundColor(LognDark.textPrimary)
                            .lineLimit(1)
                        Image(systemName: "chevron.down")
                            .font(.system(size: 12, weight: .semibold))
                            .foregroundColor(LognDark.textSecondary)
                        if let badge {
                            Text(badge)
                                .font(.plexMonoMedium(9.5))
                                .tracking(0.06 * 9.5)
                                .foregroundColor(LognDark.canvas)
                                .padding(.horizontal, 5)
                                .padding(.vertical, 2)
                                .background(expired ? LognDark.wrong : LognDark.warn)
                                .cornerRadius(Radius.xs)
                                .fixedSize()
                        }
                    }
                    Text(headerMeta)
                        .font(.plexMono(10))
                        .monospacedDigit()
                        .foregroundColor(LognDark.textMuted)
                        .lineLimit(1)
                }
            }
            .frame(minHeight: 44)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel([Str.Catalog.button, track.name, badge].compactMap { $0 }.joined(separator: ", "))
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

    /// Sessão em pé, servidor fora de alcance.
    ///
    /// Mesma barra do visitante, mesmo `warn`: o jogo continua, o que não vai acontecer
    /// agora é a subida do progresso. Dizer isso é melhor do que deixar o jogador
    /// descobrir a fila crescendo no perfil.
    private var offlineWarning: some View {
        HStack(spacing: Space.sm) {
            Image(systemName: "wifi.slash")
                .font(.system(size: 11))
            Text(Str.Dashboard.offline_session)
                .lognLabel()
        }
        .foregroundColor(LognDark.onAccent)
        .frame(maxWidth: .infinity)
        .padding(.vertical, Space.sm)
        .background(LognDark.warn)
    }

    /// A trilha na tela é a que veio dentro do app, congelada no dia do build.
    ///
    /// A semente envelhece com o binário e não com o conteúdo: quem instala e fica
    /// offline joga a trilha daquele dia, e sem esta linha não há nada dizendo isso. O
    /// `generated_at` já estava gravado no asset desde que ele existe; ninguém lia.
    ///
    /// Sem fundo `warn`: nada está errado, e amarelo aqui competiria com as duas tarjas
    /// que de fato avisam de problema.
    private var bundledTrailNote: some View {
        // Alinhado ao topo, não ao centro: a frase com a data quebra em duas linhas em
        // tela estreita, e o ícone centralizado ficava flutuando entre elas.
        HStack(alignment: .top, spacing: Space.sm) {
            Image(systemName: "shippingbox")
                .font(.system(size: 11))
                .padding(.top, 1)
            Text(Str.Status.trail_from_bundle(bundledTrailDate))
                .lognLabel()
                .fixedSize(horizontal: false, vertical: true)
        }
        .foregroundColor(LognDark.textMuted)
        .frame(maxWidth: .infinity, alignment: .center)
        .padding(.horizontal, Space.lg)
        .padding(.vertical, Space.sm)
        .background(LognDark.surface)
        .overlay(alignment: .bottom) {
            Rectangle().frame(height: 1).foregroundColor(LognDark.line)
        }
    }

    /// A data como o leitor escreveria: "22 de setembro". O Core manda ISO 8601 porque
    /// não sabe em que idioma o app está — quem traduz é quem tem o locale.
    private var bundledTrailDate: String {
        let iso = ISO8601DateFormatter()
        iso.formatOptions = [.withInternetDateTime]
        guard let data = iso.date(from: core.viewModel.trailGeneratedAt) else {
            // Data ilegível não vira tela quebrada: cai no texto cru, que ainda diz algo.
            return core.viewModel.trailGeneratedAt
        }
        let f = DateFormatter()
        f.locale = Locale.current
        f.setLocalizedDateFormatFromTemplate("d MMMM")
        return f.string(from: data)
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
                Text(Str.Dashboard.map_failed)
                    .font(LognFont.bodyMedium)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, Space.screenMargin)

                LognButton(title: Str.Dashboard.try_again, variant: .secondary) {
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
                    Text(Str.Tabs.arena)
                        .lognLabel()
                        .foregroundColor(LognDark.textMuted)

                    Text(Str.Dashboard.arena_desc)
                        .font(.plexSansSemiBold(21))
                        .tracking(-0.02 * 21)
                        .foregroundColor(LognDark.textPrimary)

                    Text(Str.Dashboard.arena_sub)
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
