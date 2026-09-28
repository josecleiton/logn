import SwiftUI
import LogNCoreFFI
import LogN

/// O catálogo de trilhas em grade, um balão por trilha (LogN Trilhas, 1b; LogN Validade
/// Offline, V2). A principal no topo, as pagas em duas colunas.
struct CatalogView: View {
    /// Leva a árvore para a trilha e fecha o catálogo.
    let openTrack: (String) -> Void

    @EnvironmentObject var core: CoreWrapper
    @EnvironmentObject var storeKit: StoreKitManager
    @Environment(\.dismiss) private var dismiss

    private var free: TrackView? {
        for track in core.viewModel.tracks where track.isFree {
            return track
        }
        return nil
    }

    /// Resolvido fora do `body`: filtro dentro da árvore de views derruba o type-checker
    /// (AGENTS.md, regra 11).
    private var paid: [TrackView] {
        var out: [TrackView] = []
        for track in core.viewModel.tracks where !track.isFree {
            out.append(track)
        }
        return out
    }

    private var productIDs: [String] {
        var ids: [String] = []
        for track in paid where !track.productId.isEmpty {
            ids.append(track.productId)
        }
        return ids
    }

    private let columns = [GridItem(.flexible(), spacing: 10), GridItem(.flexible(), spacing: 10)]

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Space.lg) {
                header
                if let free {
                    principalCard(free)
                }
                LazyVGrid(columns: columns, spacing: 10) {
                    ForEach(paid, id: \.id) { track in
                        NavigationLink {
                            TrackDetailView(trackId: track.id, openTrack: openTrack)
                                .environmentObject(core)
                                .environmentObject(storeKit)
                        } label: {
                            card(track)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
            .padding(.horizontal, Space.lg)
            .padding(.bottom, Space.xl)
        }
        .background(LognDark.surface.ignoresSafeArea())
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            core.dispatch(event: .catalogOpened)
        }
        .task(id: productIDs) {
            await storeKit.loadPrices(for: productIDs)
        }
    }

    private var header: some View {
        HStack {
            Text(Str.Catalog.title)
                .font(LognFont.headlineMedium)
                .tracking(-0.02 * 24)
                .foregroundColor(LognDark.textPrimary)
            Spacer()
            if core.viewModel.isOfflineSession {
                Text(Str.Catalog.offline)
                    .font(.plexMono(10))
                    .tracking(0.1 * 10)
                    .foregroundColor(LognDark.textMuted)
            }
        }
        .padding(.top, Space.sm)
    }

    private func principalCard(_ track: TrackView) -> some View {
        Button {
            openTrack(track.id)
        } label: {
            HStack(spacing: 12) {
                TrackBalloon(track: track, width: 30)
                VStack(alignment: .leading, spacing: 3) {
                    Text(Str.Catalog.principal)
                        .font(.plexMono(10))
                        .tracking(0.12 * 10)
                        .foregroundColor(LognDark.accentInk)
                    Text(track.name)
                        .font(.plexSansSemiBold(16))
                        .foregroundColor(LognDark.textPrimary)
                    Text(Str.Catalog.progress(Int(track.nodesDone), Int(track.nodeCount), Int(track.trackXp)))
                        .font(.plexMono(11))
                        .foregroundColor(LognDark.textSecondary)
                }
                Spacer(minLength: 0)
            }
            .padding(14)
            .background(track.selected ? LognDark.accentTint : LognDark.canvas)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(track.selected ? LognDark.accent : LognDark.line, lineWidth: 1))
            .cornerRadius(Radius.sm)
        }
        .buttonStyle(.plain)
    }

    private func card(_ track: TrackView) -> some View {
        let chip = TrackChipStyle.of(track, price: storeKit.prices[track.productId])
        let warn = track.owned && (track.offline == .soon || track.offline == .today)
        return VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .top) {
                TrackBalloon(track: track, width: 22, dimmed: track.isExpired || track.revoked)
                Spacer()
                Text(track.owned
                     ? verbatimProgress(track)
                     : Str.Catalog.nodes(Int(track.nodeCount)))
                    .font(.plexMono(10))
                    .foregroundColor(LognDark.textMuted)
            }
            Text(track.name)
                .font(.plexSansSemiBold(14))
                .foregroundColor(LognDark.textPrimary)
                .opacity(track.isExpired || track.revoked ? 0.45 : 1)
                .frame(maxWidth: .infinity, alignment: .leading)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 0)
            TrackChip(style: chip)
        }
        .padding(12)
        .frame(minHeight: 132, alignment: .topLeading)
        .background(LognDark.canvas)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(warn ? LognDark.warn : LognDark.line, lineWidth: 1))
        .cornerRadius(Radius.sm)
        .accessibilityElement(children: .combine)
        .accessibilityLabel(Str.Catalog.open_accessibility(track.name, chip.text))
    }

    /// "5/9" na comprada: é progresso, não contagem.
    private func verbatimProgress(_ track: TrackView) -> String {
        "\(track.nodesDone)/\(track.nodeCount)"
    }
}
