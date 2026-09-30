import AuthenticationServices
import CryptoKit
import Foundation
import Security
import UIKit

/// Login com o GitHub (ADR 0019): OAuth 2.0 com PKCE numa `ASWebAuthenticationSession`.
///
/// Ao contrário do Google, a troca do código não acontece aqui: o GitHub exige o
/// `client_secret` nela, mesmo com PKCE, e secret em app não é secret. O shell entrega ao
/// Core o código, o verifier e o nonce cru; o Core pede ao servidor a troca, e o servidor
/// devolve o bilhete que faz o papel do ID token.
@MainActor
final class GitHubAuth: NSObject {
    static let shared = GitHubAuth()

    struct Credential {
        let code: String
        let verifier: String
        /// O nonce cru. O servidor recebe o SHA-256 dele na troca, e o cru no login.
        let nonce: String
    }

    enum Failure: Error {
        /// O jogador fechou a janela do GitHub. Não é erro para mostrar.
        case cancelled
        case notConfigured
        case failed
    }

    private static let authorizationURL = URL(string: "https://github.com/login/oauth/authorize")!
    /// O retorno cadastrado como callback no OAuth App. O servidor manda o mesmo valor na
    /// troca, e o GitHub recusa se não bater.
    private static let callbackScheme = "logn"
    private static let redirectURI = "logn://oauth/github"

    private var session: ASWebAuthenticationSession?
    /// Um login por vez, como no Google: um segundo toque abria outra sessão e soltava a
    /// primeira no meio.
    private var inFlight = false

    /// O Client ID vem do `Info.plist`, que o recebe do `Local.xcconfig`. Vazio ou sem
    /// substituir, o botão não aparece.
    var clientID: String? {
        guard let raw = Bundle.main.object(forInfoDictionaryKey: "LogNGitHubClientID") as? String else { return nil }
        let id = raw.trimmingCharacters(in: .whitespaces)
        guard !id.isEmpty, !id.contains("$") else { return nil }
        return id
    }

    var isConfigured: Bool { clientID != nil }

    /// Abre o login do GitHub e devolve o código com o verifier e o nonce cru.
    func signIn() async throws -> Credential {
        guard let clientID else { throw Failure.notConfigured }
        // O toque repetido é ignorado como desistência: o login que já está aberto segue.
        guard !inFlight else { throw Failure.cancelled }
        inFlight = true
        defer { inFlight = false }

        let verifier = Self.randomToken()
        let state = Self.randomToken()
        let nonce = Self.randomToken()

        var components = URLComponents(url: Self.authorizationURL, resolvingAgainstBaseURL: false)!
        components.queryItems = [
            URLQueryItem(name: "client_id", value: clientID),
            URLQueryItem(name: "redirect_uri", value: Self.redirectURI),
            // Só o que o servidor usa: o id da conta, que não pede escopo, e a lista de
            // e-mails, que pede `user:email`. Nada de repositório.
            URLQueryItem(name: "scope", value: "user:email"),
            URLQueryItem(name: "code_challenge", value: Self.base64URL(Data(SHA256.hash(data: Data(verifier.utf8))))),
            URLQueryItem(name: "code_challenge_method", value: "S256"),
            URLQueryItem(name: "state", value: state),
            // Quem tem mais de uma conta GitHub escolhe, em vez de entrar na última.
            URLQueryItem(name: "prompt", value: "select_account"),
        ]
        guard let authURL = components.url else { throw Failure.failed }

        let callback = try await authorize(url: authURL)
        let items = URLComponents(url: callback, resolvingAgainstBaseURL: false)?.queryItems ?? []
        let value = { (name: String) in items.first { $0.name == name }?.value }

        guard callback.host == "oauth", callback.path == "/github" else { throw Failure.failed }
        // Recusa na tela do GitHub volta com `error=access_denied`: é desistência.
        if value("error") == "access_denied" { throw Failure.cancelled }
        // `state` diferente é resposta de outro pedido: não se usa o código dela.
        guard value("state") == state else { throw Failure.failed }
        guard let code = value("code"), !code.isEmpty else { throw Failure.failed }

        return Credential(code: code, verifier: verifier, nonce: nonce)
    }

    private func authorize(url: URL) async throws -> URL {
        try await withCheckedThrowingContinuation { continuation in
            let session = ASWebAuthenticationSession(url: url, callbackURLScheme: Self.callbackScheme) { [weak self] callback, error in
                Task { @MainActor in self?.session = nil }
                if let error = error as? ASWebAuthenticationSessionError, error.code == .canceledLogin {
                    continuation.resume(throwing: Failure.cancelled)
                } else if let callback {
                    continuation.resume(returning: callback)
                } else {
                    continuation.resume(throwing: Failure.failed)
                }
            }
            session.presentationContextProvider = self
            // Sem cookie do Safari: a conta do GitHub aberta no navegador não entra sozinha
            // aqui, e quem tem duas escolhe na hora.
            session.prefersEphemeralWebBrowserSession = true
            self.session = session
            if !session.start() {
                continuation.resume(throwing: Failure.failed)
            }
        }
    }

    /// 32 bytes aleatórios em base64url: 43 caracteres, o que o servidor espera do nonce
    /// e o mínimo que o PKCE aceita no verifier.
    private static func randomToken() -> String {
        var bytes = [UInt8](repeating: 0, count: 32)
        let status = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        precondition(status == errSecSuccess, "sem fonte de aleatoriedade")
        return base64URL(Data(bytes))
    }

    private static func base64URL(_ data: Data) -> String {
        data.base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}

extension GitHubAuth: ASWebAuthenticationPresentationContextProviding {
    nonisolated func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        MainActor.assumeIsolated {
            UIApplication.shared.connectedScenes
                .compactMap { $0 as? UIWindowScene }
                .flatMap(\.windows)
                .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
        }
    }
}
