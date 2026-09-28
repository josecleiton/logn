import SwiftUI
import LogNCoreFFI
import LogN

/// A chamada de compra do nó fechado de uma trilha paga.
///
/// O produto vem do catálogo do servidor (`viewModel.tracks`), nunca de um id montado
/// no app. Fecha sozinha quando a compra é confirmada e a trilha passa a ser da conta.
struct PaywallView: View {
    let trackId: String

    @EnvironmentObject var core: CoreWrapper
    @EnvironmentObject var storeKit: StoreKitManager
    @Environment(\.dismiss) private var dismiss

    private var track: TrackView? {
        core.viewModel.tracks.first { $0.id == trackId }
    }

    private var productId: String {
        track?.productId ?? ""
    }

    private var isBusy: Bool {
        storeKit.isPurchasing || core.viewModel.purchaseInFlight
    }

    private var buyTitle: String {
        if let price = storeKit.prices[productId] {
            return Str.Paywall.buy(price)
        }
        return Str.Paywall.buy_no_price
    }

    /// O que dizer embaixo do botão: primeiro o que a loja respondeu, depois o que o
    /// servidor respondeu sobre a compra.
    private var message: String? {
        if let problem = storeKit.problem {
            return problem.copy
        }
        switch core.viewModel.status {
        case .purchaseConfirming, .purchaseFailed, .purchaseOwnedByOtherAccount,
             .purchaseAccountMismatch, .purchaseRevoked, .purchaseNeedsAccount:
            return core.viewModel.status.copy
        default:
            return nil
        }
    }

    var body: some View {
        VStack(spacing: Space.lg) {
            Image(systemName: "lock.fill")
                .font(.system(size: 48))
                .foregroundColor(LognDark.accent)
                .padding(.top, Space.xxl)
                .accessibilityHidden(true)

            Text(track?.name ?? Str.Paywall.title)
                .font(LognFont.headlineMedium)
                .foregroundColor(LognDark.textPrimary)
                .multilineTextAlignment(.center)

            Text(Str.Paywall.body)
                .font(LognFont.bodyLarge)
                .foregroundColor(LognDark.textSecondary)
                .multilineTextAlignment(.center)

            Text(Str.Paywall.gates_note)
                .font(LognFont.bodyMedium)
                .foregroundColor(LognDark.textMuted)
                .multilineTextAlignment(.center)

            Spacer()

            if let message {
                Text(message)
                    .font(LognFont.bodyMedium)
                    .foregroundColor(core.viewModel.status.isInProgress ? LognDark.textSecondary : LognDark.wrongInk)
                    .multilineTextAlignment(.center)
            }

            LognButton(
                title: buyTitle,
                variant: .primary,
                action: buy,
                isDisabled: productId.isEmpty,
                isLoading: isBusy
            )
            LognButton(title: Str.Paywall.close, variant: .ghost, action: { dismiss() })
        }
        .padding(.horizontal, Space.xxl)
        .padding(.bottom, Space.xl)
        .background(LognDark.canvas.ignoresSafeArea())
        .task(id: productId) {
            await storeKit.loadPrice(for: productId)
        }
        .onChange(of: track?.owned ?? false) { owned in
            if owned {
                dismiss()
            }
        }
    }

    private func buy() {
        Task {
            await storeKit.purchase(productID: productId)
        }
    }
}
