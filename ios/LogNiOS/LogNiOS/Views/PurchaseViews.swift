import SwiftUI
import LogNCoreFFI
import LogN

// Os pontos de compra da trilha paga (LogN Trilhas, seções B e C). Mesmo produto e mesmo
// preço nos três; o preço vem sempre da loja.

/// O nó fechado tocado na árvore (1e).
struct LockedNodeSheet: View {
    let node: SkillNode
    let track: TrackView

    @EnvironmentObject var storeKit: StoreKitManager
    @Environment(\.dismiss) private var dismiss

    private var nodeIndex: Int {
        for (i, row) in track.nodes.enumerated() where row.id == node.id {
            return i + 1
        }
        return 1
    }

    private var unlockTitle: String {
        if let price = storeKit.prices[track.productId] {
            return Str.Locked.unlock(track.name, price)
        }
        return Str.Locked.unlock_no_price(track.name)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(Str.Locked.eyebrow(nodeIndex, node.name.uppercased()))
                .font(.plexMono(10))
                .tracking(0.12 * 10)
                .foregroundColor(LognDark.textMuted)
            Text(Str.Locked.title)
                .font(.plexSansSemiBold(20))
                .tracking(-0.02 * 20)
                .foregroundColor(LognDark.textPrimary)
            // "Você terminou a amostra" só é dito a quem terminou. Cada frase é uma chave
            // inteira do catálogo: nada de montar frase no código.
            VStack(alignment: .leading, spacing: 2) {
                if track.sampleDone {
                    Text(Str.Locked.sample_done(Int(track.sampleBalloons)))
                }
                Text(Str.Locked.rest(Int(track.closedNodeCount)))
            }
            .font(.plexSans(13.5))
            .lineSpacing(20 - 13.5)
            .foregroundColor(LognDark.textSecondary)
            .fixedSize(horizontal: false, vertical: true)
            if let problem = storeKit.problem {
                Text(problem.copy)
                    .font(LognFont.bodyMedium)
                    .foregroundColor(LognDark.textSecondary)
            }
            // A folha fecha antes da compra: a tela do passo a passo e a do visitante
            // abrem da raiz, e uma raiz que já apresenta esta folha não apresenta outra.
            LognButton(title: unlockTitle, variant: .primary,
                       action: {
                           let product = track.productId
                           dismiss()
                           DispatchQueue.main.asyncAfter(deadline: .now() + 0.35) {
                               storeKit.buy(productID: product)
                           }
                       },
                       isDisabled: track.productId.isEmpty, isLoading: storeKit.isPurchasing)
            Button(Str.Locked.not_now) { dismiss() }
                .font(.plexSans(13.5))
                .foregroundColor(LognDark.textSecondary)
                .frame(maxWidth: .infinity, minHeight: 44)
        }
        .padding(.horizontal, 22)
        .padding(.top, 22)
        .padding(.bottom, 12)
        .background(LognDark.surface.ignoresSafeArea())
        .presentationDetents([.height(360)])
        .presentationDragIndicator(.visible)
        .task { await storeKit.loadPrices(for: [track.productId]) }
    }
}

/// A oferta do fim da amostra, no lugar do "Entendi" do relatório (1f). Pede a compra
/// com o XP ainda na tela.
struct SampleOfferCard: View {
    let offer: SampleOfferView
    let onDismiss: () -> Void

    @EnvironmentObject var core: CoreWrapper
    @EnvironmentObject var storeKit: StoreKitManager

    private var track: TrackView? {
        for t in core.viewModel.tracks where t.id == offer.trackId {
            return t
        }
        return nil
    }

    private var free: TrackView? {
        for t in core.viewModel.tracks where t.isFree {
            return t
        }
        return nil
    }

    var body: some View {
        let name = track?.name ?? ""
        let price = track.flatMap { storeKit.prices[$0.productId] }
        VStack(spacing: 10) {
            VStack(alignment: .leading, spacing: 10) {
                Text(Str.Offer.eyebrow(Int(offer.nextNodeIndex), Int(track?.nodeCount ?? 0)))
                    .font(.plexMono(10))
                    .tracking(0.12 * 10)
                    .foregroundColor(LognDark.textMuted)
                Text(offer.nextNodeName)
                    .font(.plexSansSemiBold(16))
                    .foregroundColor(LognDark.textPrimary)
                // Contagens em mono, separadas; a frase da compra inteira, abaixo.
                HStack(spacing: 8) {
                    Text(Str.Offer.nodes(Int(offer.remainingNodes)))
                    Text(Str.Offer.problems(Int(offer.remainingProblems)))
                }
                .font(.plexMono(12))
                .foregroundColor(LognDark.textSecondary)
                Text(Str.Offer.tail)
                    .font(.plexSans(13.5))
                    .foregroundColor(LognDark.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(16)
            .background(LognDark.canvas)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.lineStrong, lineWidth: 1))
            .cornerRadius(Radius.sm)

            LognButton(title: price.map { Str.Offer.continue_buy(name, $0) } ?? Str.Offer.continue_buy_no_price(name),
                       variant: .primary,
                       action: { storeKit.buy(productID: track?.productId ?? "") },
                       isDisabled: track == nil, isLoading: storeKit.isPurchasing)
            Button(Str.Offer.back(free?.name ?? "")) {
                core.dispatch(event: .dismissSampleOffer(trackId: offer.trackId))
                if let free {
                    core.dispatch(event: .selectTrack(trackId: free.id))
                }
                onDismiss()
            }
            .font(.plexSans(13.5))
            .foregroundColor(LognDark.textSecondary)
            .frame(maxWidth: .infinity, minHeight: 40)
        }
        .task { await storeKit.loadPrices(for: [track?.productId ?? ""]) }
    }
}

/// O passo a passo da compra (F3): o app só libera depois de o servidor confirmar, e o
/// log mostra cada passo como saída de juiz.
struct PurchaseProgressView: View {
    @EnvironmentObject var core: CoreWrapper

    private var flow: PurchaseSteps { PurchaseSteps(view: core.viewModel.purchaseFlow) }

    private var track: TrackView? {
        for t in core.viewModel.tracks where t.id == core.viewModel.purchaseFlow.trackId {
            return t
        }
        return nil
    }

    var body: some View {
        let name = track?.name ?? ""
        VStack(alignment: .leading, spacing: 14) {
            if let track {
                TrackBalloon(track: track, width: 34)
            }
            Text(flow.label)
                .font(.plexMono(11))
                .tracking(0.12 * 11)
                .foregroundColor(flow.isFailed ? LognDark.wrongInk : LognDark.textMuted)
            Text(flow.title(name))
                .font(.plexSansSemiBold(24))
                .tracking(-0.02 * 24)
                .foregroundColor(LognDark.textPrimary)
            Text(flow.body(core.viewModel.purchaseFlow.failure))
                .font(.plexSans(14))
                .lineSpacing(20 - 14)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)

            VStack(alignment: .leading, spacing: 0) {
                ForEach(flow.log, id: \.text) { line in
                    HStack(spacing: 8) {
                        Text(line.mark)
                            .foregroundColor(line.markInk)
                            .frame(width: 18, alignment: .leading)
                        Text(line.text)
                            .foregroundColor(line.done ? LognDark.textPrimary : LognDark.textMuted)
                        Spacer()
                        Text(line.meta)
                            .foregroundColor(LognDark.textMuted)
                    }
                    .font(.plexMono(12))
                    .frame(minHeight: 22)
                }
            }
            .padding(14)
            .background(LognDark.canvas)
            .overlay(RoundedRectangle(cornerRadius: Radius.sm).stroke(LognDark.line, lineWidth: 1))
            .cornerRadius(Radius.sm)
            .accessibilityElement(children: .combine)

            Spacer()

            LognButton(title: flow.button(name),
                       variant: flow.isFailed ? .secondary : .primary,
                       action: { core.dispatch(event: .closePurchaseFlow) },
                       isDisabled: flow.isWaiting)
            // A tela nunca prende: enquanto espera, dá para sair, e a compra termina
            // sozinha, em segundo plano.
            if flow.isWaiting {
                Button(Str.Purchase.background) { core.dispatch(event: .closePurchaseFlow) }
                    .font(.plexSans(13.5))
                    .foregroundColor(LognDark.textSecondary)
                    .frame(maxWidth: .infinity, minHeight: 44)
            }
        }
        .padding(.horizontal, 22)
        .padding(.top, 40)
        .padding(.bottom, 26)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(LognDark.surface.ignoresSafeArea())
    }
}

/// O que cada passo da compra mostra. Fora da view para o `body` ficar leve.
struct PurchaseSteps {
    let view: LogN.PurchaseFlowView

    struct Line {
        let mark: String
        let markInk: Color
        let text: String
        let meta: String
        let done: Bool
    }

    private var rank: Int {
        switch view.stage {
        case .idle, .validating: return 1
        case .licensing: return 2
        case .downloading: return 3
        case .ready: return 4
        case .failed: return 0
        }
    }

    var isFailed: Bool { view.stage == .failed }
    var isWaiting: Bool { !isFailed && view.stage != .ready }

    var label: String {
        switch view.stage {
        case .ready: return Str.Purchase.ready_label
        case .downloading, .licensing: return Str.Purchase.downloading_label
        case .failed: return Str.Purchase.failed_label
        default: return Str.Purchase.validating_label
        }
    }

    func title(_ track: String) -> String {
        switch view.stage {
        case .ready: return Str.Purchase.ready_title
        case .downloading, .licensing: return Str.Purchase.downloading_title(track)
        case .failed: return Str.Purchase.failed_title
        default: return Str.Purchase.validating_title
        }
    }

    func body(_ failure: StatusKey) -> String {
        switch view.stage {
        case .ready: return Str.Purchase.ready_body
        case .downloading, .licensing: return Str.Purchase.downloading_body
        case .failed: return failure.copy ?? Str.Status.purchase_failed
        default: return Str.Purchase.validating_body
        }
    }

    func button(_ track: String) -> String {
        if isFailed { return Str.Purchase.close }
        return view.stage == .ready ? Str.Purchase.open_track(track) : Str.Purchase.wait
    }

    /// Os quatro passos: loja, transação, licença, pacote. Feito, em andamento, a vir.
    var log: [Line] {
        let steps: [(String, String)] = [
            (Str.Purchase.log_store, Str.Purchase.log_store_ok),
            (Str.Purchase.log_transaction, Str.Purchase.log_transaction_ok),
            (Str.Purchase.log_license, Str.Purchase.log_license_ok),
            (Str.Purchase.log_package, Str.Purchase.log_package_ok),
        ]
        var lines: [Line] = []
        for (i, step) in steps.enumerated() {
            // A loja já aprovou quando esta tela abre; cada passo seguinte depende do anterior.
            let done = i == 0 || i < rank
            let current = !isFailed && i == rank
            lines.append(Line(
                mark: done ? "✓" : (current ? "…" : "·"),
                markInk: done ? LognDark.correctInk : (current ? LognDark.warnInk : LognDark.textMuted),
                text: step.0,
                meta: done ? step.1 : "",
                done: done
            ))
        }
        return lines
    }
}

/// O visitante tocou em comprar (F2): a compra pede conta, e volta sozinha depois.
struct GuestPurchaseSheet: View {
    let prompt: StoreKitManager.GuestPrompt

    @EnvironmentObject var core: CoreWrapper
    @EnvironmentObject var storeKit: StoreKitManager

    private var track: TrackView? {
        for t in core.viewModel.tracks where t.productId == prompt.productID {
            return t
        }
        return nil
    }

    /// "GRAFOS · R$ 29,90", sem o preço quando a loja ainda não respondeu. Fora do
    /// `body` (AGENTS.md, regra 11).
    private func eyebrow(_ track: TrackView) -> String {
        let name = track.name.uppercased()
        guard let price = storeKit.prices[track.productId] else { return name }
        return name + " · " + price
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let track {
                HStack(spacing: 10) {
                    TrackBalloon(track: track, width: 16)
                    Text(eyebrow(track))
                        .font(.plexMono(10.5))
                        .tracking(0.12 * 10.5)
                        .foregroundColor(LognDark.textMuted)
                }
            }
            Text(Str.Guest_purchase.title)
                .font(.plexSansSemiBold(20))
                .tracking(-0.02 * 20)
                .foregroundColor(LognDark.textPrimary)
            Text(Str.Guest_purchase.body)
                .font(.plexSans(13.5))
                .lineSpacing(20 - 13.5)
                .foregroundColor(LognDark.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
            LognButton(title: Str.Guest_purchase.create, variant: .primary) {
                storeKit.continueAfterSignUp(prompt, register: true)
            }
            LognButton(title: Str.Guest_purchase.login, variant: .secondary) {
                storeKit.continueAfterSignUp(prompt, register: false)
            }
            Text(Str.Guest_purchase.note)
                .font(.plexMono(10))
                .foregroundColor(LognDark.textMuted)
                .frame(maxWidth: .infinity)
        }
        .padding(.horizontal, 22)
        .padding(.top, 22)
        .padding(.bottom, 12)
        .background(LognDark.surface.ignoresSafeArea())
        .presentationDetents([.height(420)])
        .presentationDragIndicator(.visible)
    }
}
