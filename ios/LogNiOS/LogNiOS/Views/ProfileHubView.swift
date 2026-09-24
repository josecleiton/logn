import SwiftUI
import LogNCoreFFI
import LogN
import App

/// Hub de perfil — tela 5 do DS.
///
/// Sobe como **sheet** sobre a árvore, que fica visível a 18% atrás: o jogador não perde
/// o lugar. Sem tab bar. Três estados: sincronizado, com fila offline, e visitante.
struct ProfileHubView: View {
    @EnvironmentObject var core: CoreWrapper
    @Environment(\.dismiss) private var dismiss

    /// Sheet crítico de saída, quando há evento na fila.
    @State private var showsCriticalLogout = false
    @State private var showsManageAccount = false

    /// Altura de partida do sheet, só até a primeira medição chegar.
    static let preferredHeight: CGFloat = 560

    /// Altura real do conteúdo.
    ///
    /// O hub muda de tamanho conforme o estado — o cartão da fila offline só existe
    /// quando há evento pendente, e o bloco do visitante é outro. Com a altura fixa de
    /// 560 o estado sincronizado sobrava mais de 100dp de vazio no topo do sheet.
    @State private var contentHeight: CGFloat = ProfileHubView.preferredHeight

    private struct ContentHeightKey: PreferenceKey {
        static var defaultValue: CGFloat = 0
        static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
            value = max(value, nextValue())
        }
    }

    private var vm: LogN.ViewModel { core.viewModel }
    private var isGuest: Bool { vm.isGuest }
    private var pending: Int { Int(vm.pendingSyncCount) }

    var body: some View {
        NavigationStack {
            ZStack(alignment: .bottom) {
                LognDark.surfaceRaised.ignoresSafeArea()

                VStack(alignment: .leading, spacing: 0) {
                    grabber
                    identity
                    if pending > 0 && !isGuest { queueCard }
                    levelBlock
                    statsGrid
                    summary
                    footer
                }
                .padding(.horizontal, 20)
                .padding(.top, 16)
                .padding(.bottom, 22)
                .background(
                    GeometryReader { proxy in
                        Color.clear.preference(key: ContentHeightKey.self, value: proxy.size.height)
                    }
                )
            }
            .navigationDestination(isPresented: $showsManageAccount) {
                ManageAccountView().environmentObject(core)
            }
        }
        .onPreferenceChange(ContentHeightKey.self) { height in
            if height > 0 { contentHeight = height }
        }
        // Gerenciar conta é uma tela empilhada: aí o sheet precisa da altura toda.
        .presentationDetents(showsManageAccount ? [.large] : [.height(contentHeight)])
        .presentationDragIndicator(.hidden)
        .modifier(SheetCorners())
        .sheet(isPresented: $showsCriticalLogout) {
            CriticalLogoutSheet(
                pendingCount: pending,
                xpAtRisk: Int(vm.xpIntoLevel),
                // Um evento só: quem decide sair é o core, depois da fila subir.
                onSyncAndLeave: { core.dispatch(event: .syncAndLogout) },
                onStay: { showsCriticalLogout = false },
                onDiscard: { core.dispatch(event: .logout) }
            )
            .presentationDetents([.height(CriticalLogoutSheet.preferredHeight)])
            .presentationDragIndicator(.hidden)
            .modifier(SheetCorners())
        }
    }

    // MARK: Grabber

    private var grabber: some View {
        Capsule()
            .fill(LognDark.lineStrong)
            .frame(width: 36, height: 3)
            .frame(maxWidth: .infinity)
            .padding(.bottom, 18)
    }

    // MARK: Identidade

    private var identity: some View {
        HStack(spacing: 14) {
            if isGuest {
                // Visitante não tem inicial: ícone genérico e borda tracejada.
                Circle()
                    .fill(LognDark.surface)
                    .frame(width: 52, height: 52)
                    .overlay(
                        Circle().strokeBorder(
                            LognDark.lineDim,
                            style: StrokeStyle(lineWidth: 1, dash: [4])
                        )
                    )
                    .overlay(
                        Image(systemName: "person")
                            .font(.system(size: 22, weight: .regular))
                            .foregroundColor(LognDark.textMuted)
                    )
            } else {
                ProfileAvatar(initial: vm.displayName.isEmpty ? vm.accountEmail : vm.displayName, size: 52)
            }

            VStack(alignment: .leading, spacing: 0) {
                if isGuest {
                    Text(Str.Profile.guest_mode)
                        .font(.plexMono(11))
                        .tracking(0.1 * 11)
                        .foregroundColor(LognDark.textSecondary)
                        .padding(.horizontal, 9)
                        .padding(.vertical, 4)
                        .overlay(RoundedRectangle(cornerRadius: Radius.xs).stroke(LognDark.lineStrong, lineWidth: 1))

                    Text(Str.Profile.guest_sub)
                        .font(.plexMono(10))
                        .tracking(0.12 * 10)
                        .foregroundColor(LognDark.textMuted)
                        .padding(.top, 6)
                } else {
                    Text(vm.accountEmail)
                        .font(.plexMono(13))
                        .foregroundColor(LognDark.textPrimary)
                        .lineLimit(1)
                        .truncationMode(.middle)

                    HStack(spacing: 6) {
                        Circle()
                            .fill(pending > 0 ? LognDark.warn : LognDark.correct)
                            .frame(width: 6, height: 6)
                        Text(pending > 0 ? Str.Profile.queued_events(pending) : Str.Profile.all_synced)
                            .font(.plexMono(10))
                            .tracking(0.12 * 10)
                            .foregroundColor(pending > 0 ? LognDark.warnInk : LognDark.textMuted)
                    }
                    .padding(.top, 6)
                }
            }
            Spacer(minLength: 0)
        }
    }

    // MARK: Card da fila offline

    private var queueCard: some View {
        HStack(spacing: 11) {
            VStack(alignment: .leading, spacing: 0) {
                Text(Str.Profile.local_xp_only(Int(vm.globalXp)))
                    .font(.plexSans(13.5, relativeTo: .footnote))
                    .lineSpacing(19 - 13.5)
                    .foregroundColor(LognDark.textPrimary)
                    .fixedSize(horizontal: false, vertical: true)

                Text(Str.Profile.pending_events(pending))
                    .font(.plexMono(10.5))
                    .foregroundColor(LognDark.textSecondary)
                    .padding(.top, 4)
            }

            Button {
                core.dispatch(event: .syncNow)
            } label: {
                Text(vm.isSyncing ? "…" : Str.Profile.retry)
                    .font(.plexMonoMedium(11.5))
                    .foregroundColor(LognDark.warnInk)
                    .frame(height: 34)
                    .padding(.horizontal, 13)
                    .overlay(RoundedRectangle(cornerRadius: 3).stroke(LognDark.warn, lineWidth: 1))
            }
            .buttonStyle(.plain)
        }
        .padding(.horizontal, 13)
        .padding(.vertical, 12)
        .background(LognDark.tintWarn)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.warn, lineWidth: 1))
        .padding(.top, 16)
    }

    // MARK: Nível — o herói da tela

    private var levelBlock: some View {
        HStack(alignment: .bottom, spacing: 14) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text("\(vm.level)")
                    .font(.plexSansSemiBold(44))
                    .tracking(-0.035 * 44)
                    .monospacedDigit()
                    .foregroundColor(LognDark.textPrimary)
                Text(Str.Profile.level)
                    .font(.plexMono(11))
                    .tracking(0.14 * 11)
                    .foregroundColor(LognDark.textMuted)
            }

            VStack(alignment: .leading, spacing: 0) {
                HStack {
                    Text(Str.Profile.xp(Int(vm.globalXp)))
                    Spacer(minLength: 0)
                    Text("\(Int(vm.level) * Int(vm.xpForLevel))")
                }
                .font(.plexMono(10.5))
                .monospacedDigit()
                .foregroundColor(LognDark.textMuted)

                GeometryReader { geo in
                    ZStack(alignment: .leading) {
                        Capsule().fill(LognDark.line)
                        Capsule()
                            .fill(LognDark.accent)
                            .frame(width: geo.size.width * progress)
                    }
                }
                .frame(height: 4)
                .padding(.top, 6)

                Text(Str.Profile.xp_to_next(Int(vm.xpToNextLevel), Int(vm.level + 1)))
                    .font(.plexMono(9.5))
                    .monospacedDigit()
                    .foregroundColor(LognDark.textMuted)
                    .padding(.top, 5)
            }
            .padding(.bottom, 3)
        }
        .padding(.top, 20)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(
            Str.Profile.level_accessibility(Int(vm.level), Int(vm.globalXp), Int(vm.xpToNextLevel), Int(vm.level + 1))
        )
    }

    private var progress: CGFloat {
        let total = max(Int(vm.xpForLevel), 1)
        return CGFloat(Int(vm.xpIntoLevel)) / CGFloat(total)
    }

    // MARK: Stats

    private var statsGrid: some View {
        HStack(spacing: 1) {
            statCell(Str.Profile.total_xp, "\(vm.globalXp)", nil)
            statCell(Str.Profile.bugs, "\(vm.bugsFound)", "spot the bug")
            statCell(Str.Profile.dry_runs, "\(vm.dryRunsCompleted)", "trace")
        }
        .background(LognDark.line)
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
        .padding(.top, 20)
    }

    private func statCell(_ label: String, _ value: String, _ sub: String?) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(label)
                .font(.plexMono(10))
                .tracking(0.12 * 10)
                .foregroundColor(LognDark.textMuted)
            Text(value)
                .font(.plexMonoSemiBold(17))
                .monospacedDigit()
                .foregroundColor(LognDark.textPrimary)
                .padding(.top, 4)
            if let sub {
                Text(sub)
                    .font(.plexMono(9.5))
                    .foregroundColor(LognDark.textMuted)
                    .padding(.top, 2)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 13)
        .padding(.vertical, 12)
        .background(LognDark.surface)
    }

    private var summary: some View {
        Text(Str.Profile.stats_summary(Str.Profile.challenges_completed(Int(vm.challengesCompleted)), Str.Dashboard.balloons_up(Int(vm.balloonsUp))))
            .font(.plexMono(10.5))
            .monospacedDigit()
            .foregroundColor(LognDark.textMuted)
            .padding(.top, 10)
    }

    // MARK: Rodapé — muda inteiro entre visitante e conta

    @ViewBuilder
    private var footer: some View {
        if isGuest {
            conversionCard.padding(.top, 22)

            Button {
                core.dispatch(event: .logout)
            } label: {
                Text(Str.Profile.has_account)
                    .font(.plexSans(14, relativeTo: .subheadline))
                    .foregroundColor(LognDark.textSecondary)
                    .frame(maxWidth: .infinity, minHeight: 42)
            }
            .buttonStyle(.plain)
            .padding(.top, 6)
        } else {
            Rectangle()
                .frame(height: 1)
                .foregroundColor(LognDark.line)
                .padding(.top, pending > 0 ? 20 : 22)
                .padding(.bottom, 16)

            Button {
                // Sem alerta quando não há o que perder: sair sincronizado é reversível,
                // e a tela de saída traz o desfazer.
                if pending > 0 { showsCriticalLogout = true } else { core.dispatch(event: .logout) }
            } label: {
                HStack(spacing: 9) {
                    if pending > 0 {
                        Circle().fill(LognDark.warn).frame(width: 7, height: 7)
                    }
                    Text(Str.Profile.sign_out)
                        .font(.plexSansMedium(15))
                        .foregroundColor(LognDark.textPrimary)
                }
                .frame(maxWidth: .infinity, minHeight: 50)
                .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
            }
            .buttonStyle(PressSinkStyle())

            Button {
                showsManageAccount = true
            } label: {
                HStack(spacing: 7) {
                    Image(systemName: "lock")
                        .font(.system(size: 12, weight: .medium))
                        .foregroundColor(LognDark.textMuted)
                    Text(Str.Profile.manage_account)
                        .font(.plexSans(13.5, relativeTo: .footnote))
                        .foregroundColor(LognDark.textSecondary)
                }
                .frame(maxWidth: .infinity, minHeight: 40)
            }
            .buttonStyle(.plain)
            .padding(.top, 6)
        }
    }

    /// Nomeia o risco e o ganho em números reais — nunca um "crie sua conta" genérico.
    private var conversionCard: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(Str.Profile.guest_risk)
                .font(.plexSansSemiBold(16, relativeTo: .headline))
                .lineSpacing(16 * 0.3)
                .foregroundColor(LognDark.textPrimary)
                .fixedSize(horizontal: false, vertical: true)

            Text(Str.Profile.guest_risk_desc(Int(vm.globalXp), Int(vm.balloonsUp), Int(vm.challengesCompleted)))
                .font(.plexSans(13.5, relativeTo: .footnote))
                .lineSpacing(20 - 13.5)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 8)

            Button {
                core.wantsRegistration = true
                core.dispatch(event: .logout)
            } label: {
                Text(Str.Profile.create_account)
                    .font(.plexSansSemiBold(15, relativeTo: .callout))
                    .foregroundColor(LognDark.onAccent)
                    .frame(maxWidth: .infinity, minHeight: 50)
                    .background(LognDark.accent)
                    .cornerRadius(Radius.sm)
            }
            .buttonStyle(PressSinkStyle())
            .padding(.top, 16)
        }
        .padding(.horizontal, 16)
        .padding(.top, 16)
        .padding(.bottom, 18)
        .background(LognDark.accent.opacity(0.12))
        .cornerRadius(Radius.sm)
        .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.accent, lineWidth: 1))
    }
}
