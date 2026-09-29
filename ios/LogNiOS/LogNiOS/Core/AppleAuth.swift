import AuthenticationServices
import CryptoKit
import Foundation
import Security
import UIKit

/// Sign in with Apple pelo `AuthenticationServices` (ADR 0017).
///
/// Como no Google, o shell só obtém o token. O pedido leva o SHA-256 do nonce, e o
/// Core recebe o nonce cru para o servidor conferir. Só o escopo de e-mail: o app não
/// guarda nome.
@MainActor
final class AppleAuth: NSObject {
    static let shared = AppleAuth()

    struct Credential {
        let idToken: String
        let nonce: String
        /// Uso único, válido por cinco minutos. Na exclusão da conta, o servidor o troca
        /// pelo refresh token e revoga o acesso na Apple.
        let authorizationCode: String
    }

    enum Failure: Error {
        case cancelled
        case failed
    }

    private var continuation: CheckedContinuation<Credential, Error>?
    private var nonce = ""
    private var controller: ASAuthorizationController?

    /// A capability só existe com o time pago. Sem o arquivo de entitlements no build,
    /// o pedido falharia, então o botão nem aparece.
    var isConfigured: Bool {
        let value = (Bundle.main.object(forInfoDictionaryKey: "LogNAppleSignIn") as? String) ?? ""
        return !value.trimmingCharacters(in: .whitespaces).isEmpty && !value.contains("$")
    }

    func signIn() async throws -> Credential {
        // Um pedido por vez: o toque repetido é ignorado, como no Google.
        guard continuation == nil else { throw Failure.cancelled }

        let raw = Self.randomToken()
        nonce = raw
        let request = ASAuthorizationAppleIDProvider().createRequest()
        request.requestedScopes = [.email]
        request.nonce = SHA256.hash(data: Data(raw.utf8)).map { String(format: "%02x", $0) }.joined()

        // A continuação não sente cancelamento sozinha. Sem isto, uma tarefa cancelada
        // cujo delegate nunca respondesse deixaria o pedido preso, e todo toque seguinte
        // cairia no guard acima até reiniciar o app.
        return try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                self.continuation = continuation
                let controller = ASAuthorizationController(authorizationRequests: [request])
                controller.delegate = self
                controller.presentationContextProvider = self
                self.controller = controller
                controller.performRequests()
            }
        } onCancel: {
            Task { @MainActor in AppleAuth.shared.finish(.failure(Failure.cancelled)) }
        }
    }

    private func finish(_ result: Result<Credential, Error>) {
        let continuation = continuation
        self.continuation = nil
        controller = nil
        continuation?.resume(with: result)
    }

    private static func randomToken() -> String {
        var bytes = [UInt8](repeating: 0, count: 32)
        let status = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        precondition(status == errSecSuccess, "sem fonte de aleatoriedade")
        return Data(bytes).base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}

extension AppleAuth: ASAuthorizationControllerDelegate {
    nonisolated func authorizationController(controller: ASAuthorizationController, didCompleteWithAuthorization authorization: ASAuthorization) {
        MainActor.assumeIsolated {
            guard let credential = authorization.credential as? ASAuthorizationAppleIDCredential,
                  let tokenData = credential.identityToken, let token = String(data: tokenData, encoding: .utf8),
                  let codeData = credential.authorizationCode, let code = String(data: codeData, encoding: .utf8)
            else {
                finish(.failure(Failure.failed))
                return
            }
            finish(.success(Credential(idToken: token, nonce: nonce, authorizationCode: code)))
        }
    }

    nonisolated func authorizationController(controller: ASAuthorizationController, didCompleteWithError error: Error) {
        MainActor.assumeIsolated {
            let cancelled = (error as? ASAuthorizationError)?.code == .canceled
            finish(.failure(cancelled ? Failure.cancelled : Failure.failed))
        }
    }
}

extension AppleAuth: ASAuthorizationControllerPresentationContextProviding {
    nonisolated func presentationAnchor(for controller: ASAuthorizationController) -> ASPresentationAnchor {
        MainActor.assumeIsolated {
            UIApplication.shared.connectedScenes
                .compactMap { $0 as? UIWindowScene }
                .flatMap(\.windows)
                .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
        }
    }
}
