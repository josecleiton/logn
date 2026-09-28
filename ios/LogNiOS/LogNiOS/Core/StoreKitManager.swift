import Combine
import Foundation
import StoreKit
import LogNCoreFFI
import LogN

/// A ponte com a App Store: compra, escuta transações e finaliza.
///
/// Quem decide que comprou é o servidor, nunca o app. A transação verificada vai ao Core
/// (`submitPurchase`), o Core manda ao servidor, e só o que o servidor confirmou volta em
/// `purchasesToFinish` para ser finalizado aqui. A versão anterior finalizava antes de o
/// servidor saber da compra, e uma queda de rede no meio perdia a compra de vez.
@MainActor
final class StoreKitManager: ObservableObject {
    static let shared = StoreKitManager()

    /// O que a loja respondeu e o Core não tem como saber. A cópia sai do catálogo.
    enum Problem: Equatable {
        case cancelled, pending, storeUnavailable, productMissing, needsAccount

        var copy: String {
            switch self {
            case .cancelled:        return Str.Paywall.cancelled
            case .pending:          return Str.Paywall.pending
            case .storeUnavailable: return Str.Paywall.store_unavailable
            case .productMissing:   return Str.Paywall.product_missing
            case .needsAccount:     return Str.Paywall.needs_account
            }
        }
    }

    /// A compra que um visitante pediu (F2). A folha "a compra fica na sua conta" abre
    /// com ela, e ela segue sozinha depois do cadastro.
    struct GuestPrompt: Identifiable, Equatable {
        let productID: String
        var id: String { productID }
    }

    @Published private(set) var isPurchasing = false
    @Published private(set) var problem: Problem?
    /// Preço de cada produto na moeda da loja do jogador, por id de produto. Nunca um
    /// valor fixo no app: a loja localiza.
    @Published private(set) var prices: [String: String] = [:]
    @Published var guestPrompt: GuestPrompt?

    private weak var core: CoreWrapper?
    /// Transações verificadas que esperam o servidor, por id.
    private var unfinished: [String: Transaction] = [:]
    /// Transações que chegaram antes de haver sessão para mandá-las.
    private var queued: [(jws: String, id: String, productID: String, restore: Bool)] = []
    /// A compra que o visitante pediu, à espera da conta.
    private var pendingAfterSignUp: String?
    private var tasks: [Task<Void, Never>] = []
    private var subscriptions: Set<AnyCancellable> = []

    private init() {}

    func attach(core: CoreWrapper) {
        guard self.core == nil else { return }
        self.core = core

        core.$viewModel
            .map(\.purchasesToFinish)
            .removeDuplicates()
            .sink { [weak self] ids in
                Task { await self?.finish(ids) }
            }
            .store(in: &subscriptions)
        core.$viewModel
            .map { $0.hasAccessToken && !$0.isGuest }
            .removeDuplicates()
            .sink { [weak self] ready in
                guard ready, let self else { return }
                self.flush()
                // O visitante acabou de ganhar conta: a compra que ele pediu continua.
                if let product = self.pendingAfterSignUp {
                    self.pendingAfterSignUp = nil
                    Task { await self.purchase(productID: product) }
                }
            }
            .store(in: &subscriptions)

        tasks.append(Task { [weak self] in
            for await result in Transaction.updates {
                await self?.handle(result)
            }
        })
        // Pagas e nunca confirmadas: a rede caiu depois da cobrança, ou o app fechou.
        tasks.append(Task { [weak self] in
            for await result in Transaction.unfinished {
                await self?.handle(result)
            }
        })
    }

    func loadPrices(for productIDs: [String]) async {
        let missing = productIDs.filter { !$0.isEmpty && prices[$0] == nil }
        guard !missing.isEmpty, let products = try? await Product.products(for: missing) else { return }
        for product in products {
            prices[product.id] = product.displayPrice
        }
    }

    /// Comprar a partir de qualquer tela: o visitante vê primeiro a folha da conta.
    func buy(productID: String) {
        guard let core else { return }
        if core.viewModel.isGuest || !core.viewModel.hasSession {
            guestPrompt = GuestPrompt(productID: productID)
            return
        }
        Task { await purchase(productID: productID) }
    }

    /// "Criar conta e continuar" (F2): guarda a compra e leva ao cadastro.
    func continueAfterSignUp(_ prompt: GuestPrompt, register: Bool) {
        guard let core else { return }
        pendingAfterSignUp = prompt.productID
        guestPrompt = nil
        core.wantsRegistration = register
        core.dispatch(event: .logout)
    }

    func purchase(productID: String) async {
        problem = nil
        // A compra nasce com o id da conta: o servidor recusa a transação mandada por
        // outra conta que não a que comprou.
        guard let core, !core.viewModel.isGuest, let account = UUID(uuidString: core.viewModel.accountUserId) else {
            problem = .needsAccount
            return
        }
        // Só a compra pedida agora abre o passo a passo; a que a loja reentrega na
        // abertura segue em silêncio.
        core.dispatch(event: .purchaseIntent(productId: productID))
        isPurchasing = true
        defer { isPurchasing = false }

        do {
            guard let product = try await Product.products(for: [productID]).first else {
                problem = .productMissing
                return
            }
            switch try await product.purchase(options: [.appAccountToken(account)]) {
            case .success(let verification):
                await handle(verification)
            case .userCancelled:
                problem = .cancelled
            case .pending:
                problem = .pending
            @unknown default:
                problem = .storeUnavailable
            }
        } catch {
            problem = .storeUnavailable
        }
    }

    /// "Restaurar compras": manda ao servidor cada compra do Apple ID, e o Core conta o
    /// resultado a partir de quantas foram.
    func restore() async {
        problem = nil
        guard let core, !core.viewModel.isGuest, core.viewModel.hasAccessToken else {
            problem = .needsAccount
            return
        }
        // Pedido explícito do jogador: é a hora em que a Apple pede para sincronizar.
        try? await AppStore.sync()
        var found: [(jws: String, id: String, productID: String)] = []
        for await result in Transaction.currentEntitlements {
            if case .verified(let transaction) = result {
                found.append((result.jwsRepresentation, String(transaction.id), transaction.productID))
            }
        }
        core.dispatch(event: .restoreStarted(count: UInt32(found.count)))
        for item in found {
            submit(jws: item.jws, id: item.id, productID: item.productID, restore: true)
        }
    }

    private func handle(_ result: VerificationResult<Transaction>) async {
        switch result {
        case .verified(let transaction):
            let id = String(transaction.id)
            unfinished[id] = transaction
            submit(jws: result.jwsRepresentation, id: id, productID: transaction.productID, restore: false)
        case .unverified:
            // O StoreKit não confirmou a assinatura; o servidor recusaria de qualquer jeito.
            // Fica sem finalizar, e o JWS não vai para log: ele é a prova da compra.
            break
        }
    }

    private func submit(jws: String, id: String, productID: String, restore: Bool) {
        queued.append((jws, id, productID, restore))
        flush()
    }

    private func flush() {
        guard let core, core.viewModel.hasAccessToken, !core.viewModel.isGuest else { return }
        let pending = queued
        queued.removeAll()
        for item in pending {
            core.dispatch(event: .submitPurchase(jws: item.jws, transactionId: item.id, productId: item.productID, restore: item.restore))
        }
    }

    private func finish(_ ids: [String]) async {
        for id in ids {
            if let transaction = unfinished.removeValue(forKey: id) {
                await transaction.finish()
            }
            // Transação que ainda não chegou nesta abertura volta pelo `unfinished`, e o
            // servidor, que já a tem, confirma de novo.
            core?.dispatch(event: .purchaseFinished(transactionId: id))
        }
    }
}
