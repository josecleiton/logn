import SwiftUI
import LogNCoreFFI
import LogN

/// A página de uma trilha: ementa, nós e preço antes de entrar (LogN Trilhas, 1d), e o
/// aviso de validade offline de quem comprou (LogN Validade Offline, V3).
struct TrackDetailView: View {
    let trackId: String
    let openTrack: (String) -> Void

    @EnvironmentObject var core: CoreWrapper
    @EnvironmentObject var storeKit: StoreKitManager

    private var track: TrackView? {
        for t in core.viewModel.tracks where t.id == trackId {
            return t
        }
        return nil
    }

    var body: some View {
        Group {
            if let track {
                content(track)
            } else {
                Color.clear
            }
        }
        .background(LognDark.surface.ignoresSafeArea())
        .navigationBarTitleDisplayMode(.inline)
    }

    private func content(_ track: TrackView) -> some View {
        VStack(spacing: 0) {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    heading(track)
                    if !track.owned && !track.description.isEmpty {
                        Text(track.description)
                            .font(.plexSans(14.5))
                            .lineSpacing(21 - 14.5)
                            .foregroundColor(LognDark.textSecondary)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                    notices(track)
                    nodeList(track)
                }
                .padding(.horizontal, 22)
                .padding(.bottom, Space.xl)
            }
            footer(track)
        }
    }

    private func heading(_ track: TrackView) -> some View {
        HStack(spacing: 14) {
            TrackBalloon(track: track, width: 34, dimmed: track.revoked)
            VStack(alignment: .leading, spacing: 3) {
                Text(track.name)
                    .font(.plexSansSemiBold(26))
                    .tracking(-0.02 * 26)
                    .foregroundColor(LognDark.textPrimary)
                Text(track.owned
                     ? Str.Track.owned_meta(Int(track.nodesDone), Int(track.nodeCount))
                     : Str.Track.meta(Int(track.nodeCount), track.author.uppercased(), track.languagesLabel))
                    .font(.plexMono(10.5))
                    .foregroundColor(LognDark.textMuted)
            }
        }
        .padding(.top, Space.sm)
    }

    @ViewBuilder
    private func notices(_ track: TrackView) -> some View {
        if track.revoked {
            note(text: track.revokedReason == "refund"
                 ? Str.Track.revoked_refund(track.name, Int(track.trackXp))
                 : Str.Track.revoked_other(track.name, Int(track.trackXp)),
                 background: LognDark.surfaceRaised, line: LognDark.lineStrong)
        } else if track.owned && (track.offline == .soon || track.offline == .today) {
            offlineBlock(track)
        } else if track.owned && track.discontinued {
            note(text: Str.Track.discontinued_note, background: LognDark.surfaceRaised, line: LognDark.lineStrong)
        } else if track.owned && track.validUntil > 0 {
            Text(Str.Track.valid_until(TrackDates.short(track.validUntil)))
                .font(.plexMono(10.5))
                .foregroundColor(LognDark.textMuted)
        }
    }

    private func note(text: String, background: Color, line: Color) -> some View {
        Text(text)
            .font(.plexSans(13.5))
            .lineSpacing(19 - 13.5)
            .foregroundColor(LognDark.textPrimary)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(14)
            .background(background)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(line, lineWidth: 1))
            .cornerRadius(Radius.sm)
    }

    /// O aviso se explica por inteiro: prazo, os dias desenhados, o que acontece se
    /// vencer. Não fecha: é informação, não interrupção (V3).
    private func offlineBlock(_ track: TrackView) -> some View {
        let lastDay = track.offline == .today
        return VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .firstTextBaseline) {
                Text(lastDay ? Str.Track.offline_last_day_tag : Str.Track.offline_days_tag(Int(track.daysSinceContact)))
                Spacer()
                Text(Str.Track.valid_until(TrackDates.short(track.validUntil)))
            }
            .font(.plexMono(10.5))
            .tracking(0.1 * 10.5)
            .foregroundColor(LognDark.warnInk)

            Text(lastDay
                 ? Str.Track.offline_today_body(track.name)
                 : Str.Track.offline_soon_body(TrackDates.weekday(track.validUntil), track.name))
                .font(.plexSans(14))
                .foregroundColor(LognDark.textPrimary)
                .fixedSize(horizontal: false, vertical: true)

            OfflineDaysBar(usedDays: min(30, Int(track.daysSinceContact)))

            Text(Str.Track.offline_rule)
                .font(.plexSans(12.5))
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(14)
        .background(LognDark.tintWarn)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.warn, lineWidth: 1))
        .cornerRadius(Radius.sm)
    }

    private func nodeList(_ track: TrackView) -> some View {
        VStack(spacing: 0) {
            ForEach(Array(track.nodes.enumerated()), id: \.element.id) { index, row in
                nodeRow(track, index: index, row: row)
            }
        }
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
        .cornerRadius(Radius.sm)
    }

    private func nodeRow(_ track: TrackView, index: Int, row: TrackNodeRow) -> some View {
        let sample = !track.owned && row.free
        return HStack(spacing: 10) {
            Text(String(format: "%02d", index + 1))
                .font(.plexMono(11))
                .foregroundColor(sample ? LognDark.correctInk : LognDark.textMuted)
                .frame(width: 28, alignment: .leading)
            Text(row.name)
                .font(.plexSans(13.5))
                .foregroundColor(sample || row.active ? LognDark.textPrimary : LognDark.textSecondary)
            Spacer()
            if sample {
                Text(Str.Track.free_tag)
                    .font(.plexMono(10))
                    .tracking(0.08 * 10)
                    .foregroundColor(LognDark.correctInk)
            } else if track.owned && row.done {
                Image(systemName: "checkmark")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundColor(LognDark.correctInk)
                    .accessibilityHidden(true)
            } else if track.owned && row.active {
                Image(systemName: "arrow.right")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundColor(LognDark.accentInk)
                    .accessibilityHidden(true)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 9)
        .background(sample ? LognDark.tintOk : Color.clear)
        .overlay(alignment: .top) {
            if index > 0 { Rectangle().frame(height: 1).foregroundColor(LognDark.line) }
        }
    }

    @ViewBuilder
    private func footer(_ track: TrackView) -> some View {
        let price = storeKit.prices[track.productId]
        VStack(spacing: 8) {
            if let problem = storeKit.problem {
                Text(problem.copy)
                    .font(LognFont.bodyMedium)
                    .foregroundColor(LognDark.textSecondary)
                    .multilineTextAlignment(.center)
            }
            if track.revoked {
                LognButton(title: price.map { Str.Track.buy_again($0) } ?? Str.Track.buy_again_no_price,
                           variant: .primary, action: { storeKit.buy(productID: track.productId) },
                           isLoading: storeKit.isPurchasing)
            } else if track.owned {
                LognButton(title: track.activeNodeIndex.map { Str.Track.continue_node($0) } ?? Str.Track.continue_track,
                           variant: .primary, action: { openTrack(track.id) })
            } else {
                HStack(spacing: 8) {
                    LognButton(title: Str.Track.play_sample, variant: .secondary, action: { openTrack(track.id) })
                    LognButton(title: price.map { Str.Track.buy($0) } ?? Str.Track.buy_no_price,
                               variant: .primary, action: { storeKit.buy(productID: track.productId) },
                               isDisabled: track.productId.isEmpty, isLoading: storeKit.isPurchasing)
                }
                Text(Str.Track.buy_note)
                    .font(.plexMono(10))
                    .foregroundColor(LognDark.textMuted)
            }
        }
        .padding(.horizontal, 22)
        .padding(.top, 12)
        .padding(.bottom, 22)
        .overlay(alignment: .top) { Rectangle().frame(height: 1).foregroundColor(LognDark.line) }
        .task(id: track.productId) {
            await storeKit.loadPrices(for: [track.productId])
        }
    }
}
