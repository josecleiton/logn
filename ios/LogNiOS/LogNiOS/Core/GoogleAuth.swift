import AuthenticationServices
import CryptoKit
import Foundation
import Security
import UIKit

/// Login com o Google sem SDK (ADR 0016): OAuth 2.0 com PKCE numa
/// `ASWebAuthenticationSession`, e a troca do código pelo ID token feita aqui, porque o
/// client de iOS não tem secret.
///
/// O shell só obtém o token. Quem decide o que ele vale é o servidor, que confere
/// assinatura, audiência, emissor e o nonce; o Core recebe o token e o nonce cru e
/// segue com o login.
@MainActor
final class GoogleAuth: NSObject {
    static let shared = GoogleAuth()

    struct Credential {
        let idToken: String
        /// O nonce cru. O Google recebeu o SHA-256 dele; só o servidor vê este valor.
        let nonce: String
        /// Só para `revoke` depois de excluir a conta. Não sai do aparelho e não é
        /// guardado: vence em uma hora.
        let accessToken: String
    }

    enum Failure: Error {
        /// O jogador fechou a janela do Google. Não é erro para mostrar.
        case cancelled
        case notConfigured
        case failed
    }

    /// Endereços de reserva. Os de verdade vêm do discovery document a cada abertura:
    /// se o Google trocar um endereço, o app segue sem precisar de versão nova.
    private static let discoveryURL = URL(string: "https://accounts.google.com/.well-known/openid-configuration")!
    private static let fallbackAuthorization = URL(string: "https://accounts.google.com/o/oauth2/v2/auth")!
    private static let fallbackToken = URL(string: "https://oauth2.googleapis.com/token")!
    private static let revokeURL = URL(string: "https://oauth2.googleapis.com/revoke")!

    private var endpoints: (authorization: URL, token: URL)?
    private var session: ASWebAuthenticationSession?
    /// Um login por vez. O botão só trava quando o Core recebe o token, e até lá há o
    /// discovery, a janela do Google e a troca do código: um segundo toque abria outra
    /// sessão e soltava a primeira no meio.
    private var inFlight = false

    /// O Client ID vem do `Info.plist`, que o recebe do `Local.xcconfig`. Vazio ou sem
    /// substituir, o botão não aparece.
    var clientID: String? {
        guard let raw = Bundle.main.object(forInfoDictionaryKey: "LogNGoogleClientID") as? String else { return nil }
        let id = raw.trimmingCharacters(in: .whitespaces)
        guard id.hasSuffix(".apps.googleusercontent.com"), !id.contains("$") else { return nil }
        return id
    }

    var isConfigured: Bool { clientID != nil }

    /// Abre o login do Google e devolve o ID token com o nonce cru.
    func signIn() async throws -> Credential {
        guard let clientID else { throw Failure.notConfigured }
        // O toque repetido é ignorado como desistência: o login que já está aberto segue.
        guard !inFlight else { throw Failure.cancelled }
        inFlight = true
        defer { inFlight = false }
        // Exclusão que não deu certo deixou o token aqui; um login novo o descarta.
        pendingRevocation = ""

        // O esquema de retorno é o Client ID ao contrário, como o Google exige para
        // client de iOS. A sessão intercepta o retorno sozinha, sem entrada no Info.plist.
        let prefix = clientID.replacingOccurrences(of: ".apps.googleusercontent.com", with: "")
        let scheme = "com.googleusercontent.apps.\(prefix)"
        let redirectURI = "\(scheme):/oauthredirect"

        let verifier = Self.randomToken()
        let state = Self.randomToken()
        let nonce = Self.randomToken()
        let (authorizationEndpoint, tokenEndpoint) = await resolveEndpoints()

        var components = URLComponents(url: authorizationEndpoint, resolvingAgainstBaseURL: false)!
        components.queryItems = [
            URLQueryItem(name: "client_id", value: clientID),
            URLQueryItem(name: "redirect_uri", value: redirectURI),
            URLQueryItem(name: "response_type", value: "code"),
            // Só o que o servidor usa: `sub`, `email` e `email_verified`. Nome e foto não.
            URLQueryItem(name: "scope", value: "openid email"),
            URLQueryItem(name: "code_challenge", value: Self.base64URL(Data(SHA256.hash(data: Data(verifier.utf8))))),
            URLQueryItem(name: "code_challenge_method", value: "S256"),
            URLQueryItem(name: "state", value: state),
            URLQueryItem(name: "nonce", value: Self.sha256Hex(nonce)),
            // Quem tem mais de uma conta Google escolhe, em vez de entrar na última.
            URLQueryItem(name: "prompt", value: "select_account"),
        ]
        guard let authURL = components.url else { throw Failure.failed }

        let callback = try await authorize(url: authURL, scheme: scheme)
        let items = URLComponents(url: callback, resolvingAgainstBaseURL: false)?.queryItems ?? []
        let value = { (name: String) in items.first { $0.name == name }?.value }

        guard callback.path == "/oauthredirect" else { throw Failure.failed }
        // Recusa na tela do Google pode voltar sem `state`: é desistência, não falha.
        if value("error") == "access_denied" { throw Failure.cancelled }
        // `state` diferente é resposta de outro pedido: não se usa o código dela.
        guard value("state") == state else { throw Failure.failed }
        guard let code = value("code"), !code.isEmpty else { throw Failure.failed }

        let tokens = try await exchange(
            code: code, verifier: verifier, clientID: clientID,
            redirectURI: redirectURI, endpoint: tokenEndpoint
        )
        return Credential(idToken: tokens.idToken, nonce: nonce, accessToken: tokens.accessToken)
    }

    /// O access token do login que confirmou uma exclusão, esperando ela dar certo.
    /// Só na memória; sai na revogação ou no próximo login.
    var pendingRevocation = ""

    /// Revoga o token pendente, se houver. Chamado quando a exclusão deu certo.
    func revokePending() async {
        let token = pendingRevocation
        pendingRevocation = ""
        await revoke(accessToken: token)
    }

    /// Tira o LogN dos apps com acesso à conta Google da pessoa. Só na exclusão da
    /// conta: no login comum, revogar apagaria o consentimento e o Google pediria de
    /// novo a cada entrada. Falha não importa; a conta já foi desativada.
    func revoke(accessToken: String) async {
        guard !accessToken.isEmpty else { return }
        var request = URLRequest(url: Self.revokeURL)
        request.httpMethod = "POST"
        request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        request.httpBody = "token=\(Self.formEncode(accessToken))".data(using: .utf8)
        _ = try? await URLSession.shared.data(for: request)
    }

    // MARK: - Etapas

    private func authorize(url: URL, scheme: String) async throws -> URL {
        try await withCheckedThrowingContinuation { continuation in
            let session = ASWebAuthenticationSession(url: url, callbackURLScheme: scheme) { [weak self] callback, error in
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
            self.session = session
            if !session.start() {
                continuation.resume(throwing: Failure.failed)
            }
        }
    }

    private func exchange(code: String, verifier: String, clientID: String, redirectURI: String, endpoint: URL) async throws -> (idToken: String, accessToken: String) {
        let fields = [
            ("code", code),
            ("client_id", clientID),
            ("redirect_uri", redirectURI),
            ("grant_type", "authorization_code"),
            ("code_verifier", verifier),
        ]
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        // Codificado à mão: `URLComponents` deixa `&`, `=` e `+` como estão, e um código
        // com um deles quebraria o formulário.
        request.httpBody = fields
            .map { "\(Self.formEncode($0.0))=\(Self.formEncode($0.1))" }
            .joined(separator: "&")
            .data(using: .utf8)

        let (data, response) = try await URLSession.shared.data(for: request)
        guard (response as? HTTPURLResponse)?.statusCode == 200 else { throw Failure.failed }
        struct TokenResponse: Decodable { let id_token: String; let access_token: String? }
        guard let token = try? JSONDecoder().decode(TokenResponse.self, from: data), !token.id_token.isEmpty else {
            throw Failure.failed
        }
        return (token.id_token, token.access_token ?? "")
    }

    /// Os endereços do discovery document, lidos uma vez por execução do app. Cada um só
    /// vale no host de hoje, em HTTPS: o que o discovery pode mudar sem versão nova é o
    /// caminho. Host diferente fica com o de reserva, para um documento adulterado não
    /// mandar o código e o verifier a outro servidor.
    private func resolveEndpoints() async -> (URL, URL) {
        if let endpoints { return endpoints }
        var resolved = (authorization: Self.fallbackAuthorization, token: Self.fallbackToken)
        struct Discovery: Decodable { let authorization_endpoint: String; let token_endpoint: String }
        var request = URLRequest(url: Self.discoveryURL)
        request.timeoutInterval = 5
        if let (data, _) = try? await URLSession.shared.data(for: request),
           let doc = try? JSONDecoder().decode(Discovery.self, from: data) {
            if let url = URL(string: doc.authorization_endpoint), Self.isHTTPS(url, host: Self.fallbackAuthorization.host) {
                resolved.authorization = url
            }
            if let url = URL(string: doc.token_endpoint), Self.isHTTPS(url, host: Self.fallbackToken.host) {
                resolved.token = url
            }
        }
        endpoints = resolved
        return resolved
    }

    // MARK: - Utilitários

    private static func isHTTPS(_ url: URL, host: String?) -> Bool {
        url.scheme == "https" && url.host != nil && url.host == host
    }

    /// Percent-encoding de formulário: só letras, dígitos e `-._~` passam como estão.
    private static func formEncode(_ value: String) -> String {
        var allowed = CharacterSet.alphanumerics.intersection(.init(charactersIn: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"))
        allowed.insert(charactersIn: "-._~")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? ""
    }

    /// 32 bytes aleatórios em base64url: 43 caracteres, que é o que o servidor espera
    /// do nonce e o que o PKCE aceita no verifier.
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

    private static func sha256Hex(_ value: String) -> String {
        SHA256.hash(data: Data(value.utf8)).map { String(format: "%02x", $0) }.joined()
    }
}

extension GoogleAuth: ASWebAuthenticationPresentationContextProviding {
    nonisolated func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        MainActor.assumeIsolated {
            UIApplication.shared.connectedScenes
                .compactMap { $0 as? UIWindowScene }
                .flatMap(\.windows)
                .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
        }
    }
}
