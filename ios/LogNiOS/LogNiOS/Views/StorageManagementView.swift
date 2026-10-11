import SwiftUI
import LogNCoreFFI
import LogN

/// O que as trilhas pagas ocupam no aparelho, e o botão de apagar (spec, seção 9).
///
/// Apagar tira a chave e o pacote daqui; a compra continua na conta, e baixar de novo
/// pede a licença ao servidor.
struct StorageManagementView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) private var dismiss

    /// As trilhas da conta. Resolvido fora do `body`: filtro dentro da árvore de views
    /// derruba o type-checker (AGENTS.md, regra 11).
    private var ownedTracks: [TrackView] {
        var owned: [TrackView] = []
        for track in core.viewModel.tracks where track.owned || track.downloaded {
            owned.append(track)
        }
        return owned
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            // A linha das outras folhas. O `Divider()` do sistema saía no cinza dele, por
            // cima do `.background`, que não o pintava.
            Rectangle().fill(LognColors.line).frame(height: 1)
            ScrollView {
                VStack(spacing: Space.md) {
                    Text(Str.Storage.desc)
                        .font(LognFont.bodyMedium)
                        .foregroundColor(LognColors.textSecondary)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.top, Space.lg)

                    if ownedTracks.isEmpty {
                        Text(Str.Storage.empty)
                            .font(LognFont.bodyMedium)
                            .foregroundColor(LognColors.textSecondary)
                            .padding(.top, Space.xl)
                    } else {
                        ForEach(ownedTracks, id: \.id) { track in
                            row(for: track)
                        }
                    }
                }
                .padding(.horizontal, Space.xl)
            }
            Spacer()
        }
        .background(LognColors.surfaceRaised.ignoresSafeArea())
    }

    private var header: some View {
        HStack {
            Text(Str.Storage.title)
                .font(LognFont.titleMedium)
                .foregroundColor(LognColors.textPrimary)
            Spacer()
            Button(action: { dismiss() }) {
                Image(systemName: "xmark")
                    .foregroundColor(LognColors.textSecondary)
            }
            .accessibilityLabel(Str.Storage.close)
        }
        .padding(.top, Space.xl)
        .padding(.horizontal, Space.xl)
        .padding(.bottom, Space.lg)
    }

    private func detail(for track: TrackView) -> String {
        guard track.downloaded else { return Str.Storage.not_downloaded }
        let size = ByteCountFormatter.string(fromByteCount: Int64(track.downloadBytes), countStyle: .file)
        return Str.Storage.size(size)
    }

    @ViewBuilder
    private func row(for track: TrackView) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text(track.name)
                    .font(LognFont.bodyLarge)
                    .foregroundColor(LognColors.textPrimary)
                Text(detail(for: track))
                    .font(LognFont.label)
                    .foregroundColor(LognColors.textSecondary)
            }
            Spacer()
            if track.downloaded {
                Button(action: { core.dispatch(event: .deleteTrackDownload(trackId: track.id)) }) {
                    Image(systemName: "trash")
                        .foregroundColor(LognColors.wrongInk)
                }
                .accessibilityLabel(Str.Storage.delete_accessibility(track.name))
            } else {
                Button(Str.Storage.download) {
                    core.dispatch(event: .fetchLicense(trackId: track.id))
                }
                .font(LognFont.label)
                .foregroundColor(LognColors.accentInk)
            }
        }
        .padding(Space.lg)
        .background(LognColors.canvas)
        .cornerRadius(Radius.md)
        .overlay(RoundedRectangle(cornerRadius: Radius.md).stroke(LognColors.line, lineWidth: 1))
    }
}
