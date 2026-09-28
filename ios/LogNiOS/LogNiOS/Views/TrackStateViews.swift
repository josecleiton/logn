import SwiftUI
import LogNCoreFFI
import LogN

/// "Por onde começar?" (LogN Trilhas, F1), uma tela só, na primeira abertura. Nenhum
/// preço aqui: quem é novo cai na grátis; quem já sabe vai para o catálogo.
struct OnboardingView: View {
    /// Chamado ao fechar: `true` quando o jogador escolheu ver o catálogo.
    let onFinish: (Bool) -> Void

    @EnvironmentObject var core: CoreWrapper
    @State private var wantsCatalog = false

    private var free: TrackView? {
        for t in core.viewModel.tracks where t.isFree {
            return t
        }
        return nil
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 10) {
                Text(Str.Onboarding.title)
                    .font(.plexSansSemiBold(26))
                    .tracking(-0.02 * 26)
                    .foregroundColor(LognDark.textPrimary)
                Text(Str.Onboarding.body)
                    .font(.plexSans(14.5))
                    .foregroundColor(LognDark.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .padding(.top, 40)

            VStack(spacing: 10) {
                option(selected: !wantsCatalog,
                       title: free?.name ?? "",
                       body: Str.Onboarding.free_body) { wantsCatalog = false }
                option(selected: wantsCatalog,
                       title: Str.Onboarding.know_title,
                       body: Str.Onboarding.know_body) { wantsCatalog = true }
            }
            .padding(.top, 22)

            Spacer()

            LognButton(title: Str.Onboarding.start, variant: .primary) {
                core.dispatch(event: .completeOnboarding)
                onFinish(wantsCatalog)
            }
        }
        .padding(.horizontal, 22)
        .padding(.bottom, 26)
        .background(LognDark.surface.ignoresSafeArea())
    }

    private func option(selected: Bool, title: String, body: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 12) {
                VStack(alignment: .leading, spacing: 3) {
                    Text(title)
                        .font(.plexSansSemiBold(15.5))
                        .foregroundColor(LognDark.textPrimary)
                    Text(body)
                        .font(.plexSans(13))
                        .foregroundColor(LognDark.textSecondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer()
                if selected {
                    Image(systemName: "checkmark")
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundColor(LognDark.accentInk)
                }
            }
            .padding(14)
            .background(selected ? LognDark.accentTint : Color.clear)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm)
                .stroke(selected ? LognDark.accent : LognDark.line, lineWidth: selected ? 1.5 : 1))
            .cornerRadius(Radius.sm)
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

/// A trilha comprada cuja licença offline venceu (LogN Validade Offline, V4). Estado
/// vazio dentro da trilha, não um modal: a saída secundária leva ao que ainda abre.
struct ExpiredTrackView: View {
    let track: TrackView

    @EnvironmentObject var core: CoreWrapper

    private var free: TrackView? {
        for t in core.viewModel.tracks where t.isFree {
            return t
        }
        return nil
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            TrackBalloon(track: track, width: 34, dimmed: true)
            Text(Str.Expired.eyebrow(Int(track.daysSinceContact)))
                .font(.plexMono(10.5))
                .tracking(0.12 * 10.5)
                .foregroundColor(LognDark.wrongInk)
            Text(Str.Expired.title(track.name))
                .font(.plexSansSemiBold(24))
                .tracking(-0.02 * 24)
                .foregroundColor(LognDark.textPrimary)
            Text(Str.Expired.body)
                .font(.plexSans(14))
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: 1) {
                stat(Str.Expired.progress, "\(track.nodesDone)/\(track.nodeCount)")
                stat(Str.Expired.xp, "\(track.trackXp)")
            }
            .background(LognDark.line)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
            .cornerRadius(Radius.sm)

            Spacer()

            LognButton(title: Str.Expired.retry, variant: .primary) {
                core.dispatch(event: .fetchLicense(trackId: track.id))
            }
            if let free {
                Button(Str.Expired.back(free.name)) {
                    core.dispatch(event: .selectTrack(trackId: free.id))
                }
                .font(.plexSans(13.5))
                .foregroundColor(LognDark.textSecondary)
                .frame(maxWidth: .infinity, minHeight: 44)
            }
        }
        .padding(.horizontal, 22)
        .padding(.top, 48)
        .padding(.bottom, 16)
    }

    private func stat(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(label)
                .font(.plexMono(10))
                .tracking(0.12 * 10)
                .foregroundColor(LognDark.textMuted)
            Text(value)
                .font(.plexMonoSemiBold(17))
                .foregroundColor(LognDark.textPrimary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
        .background(LognDark.surface)
    }
}

/// O resultado de "Restaurar compras".
struct RestoreResultSheet: View {
    @EnvironmentObject var core: CoreWrapper

    private var result: RestoreResultView { core.viewModel.restoreResult }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text(Str.Restore.title)
                    .font(.plexMono(10.5))
                    .tracking(0.12 * 10.5)
                    .foregroundColor(LognDark.textMuted)
                Spacer()
                if result.finished && result.total > 0 {
                    Text(Str.Restore.count(Int(result.restored), Int(result.total)))
                        .font(.plexMonoMedium(10.5))
                        .foregroundColor(result.restored > 0 ? LognDark.correctInk : LognDark.textSecondary)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 3)
                        .overlay(RoundedRectangle(cornerRadius: Radius.xs)
                            .stroke(result.restored > 0 ? LognDark.correct : LognDark.lineStrong, lineWidth: 1))
                }
            }
            Text(message)
                .font(.plexSans(14))
                .foregroundColor(LognDark.textPrimary)
                .fixedSize(horizontal: false, vertical: true)
            if result.otherAccount > 0 {
                Text(Str.Restore.other(Int(result.otherAccount)))
                    .font(.plexSans(13.5))
                    .foregroundColor(LognDark.warnInk)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: 0)
            LognButton(title: Str.Restore.ok, variant: .secondary, action: {
                core.dispatch(event: .dismissRestoreResult)
            }, isDisabled: !result.finished)
        }
        .padding(22)
        .background(LognDark.surface.ignoresSafeArea())
        .presentationDetents([.height(300)])
    }

    private var message: String {
        if !result.finished { return Str.Restore.running }
        if result.total == 0 { return Str.Restore.nothing }
        if result.restoredNames.isEmpty { return Str.Restore.nothing }
        return Str.Restore.names(ListFormatter.localizedString(byJoining: result.restoredNames))
    }
}
