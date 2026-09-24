import SwiftUI
import UIKit
import WebKit

/// Termos de uso ou política de privacidade.
enum LegalKind: String, Identifiable {
    case terms, privacy

    var id: String { rawValue }

    var title: String {
        switch self {
        case .terms: return Str.Legal.terms_title
        case .privacy: return Str.Legal.privacy_title
        }
    }

    /// Só em DEBUG: `-LogNStartScreen termos` ou `privacidade` abre o documento direto,
    /// para o roteiro do simulador conferir a tela sem tocar em nada.
    static var launchOverride: LegalKind? {
        switch LognTab.launchScreen {
        case "termos": return .terms
        case "privacidade": return .privacy
        default: return nil
        }
    }
}

/// A língua do app na forma que o servidor e o banco usam: `pt-BR`, `en` ou `es`.
///
/// É a língua em que o `Str` está mostrando a interface, e não a do sistema: quem pôs o
/// iPhone em francês lê o app em inglês, e o documento tem de vir em inglês também.
enum AppLocale {
    static var current: String {
        let preferred = Bundle.main.preferredLocalizations.first ?? "pt-BR"
        switch preferred.split(whereSeparator: { $0 == "-" || $0 == "_" }).first.map(String.init) {
        case "en": return "en"
        case "es": return "es"
        default: return "pt-BR"
        }
    }
}

/// Versão, vigência e origem do documento na tela.
struct LegalMeta: Equatable {
    var version: Int
    /// `AAAA-MM-DD`, no fuso do Brasil, como o servidor manda.
    var effectiveAt: String
    var draft: Bool
    /// Veio da cópia do bundle, e não do servidor.
    var offline: Bool
}

enum LegalLoadPhase: Equatable {
    case loading, loaded, failed
}

/// O documento no modo do app, carregado do servidor com `?embed=1`.
///
/// Sem rede ou com o servidor fora, cai na cópia que `just legal-bundle` deixa no
/// bundle. Link para o outro documento troca o documento na mesma tela; `mailto:` e
/// links externos vão para o sistema; nada mais navega. JavaScript fica desligado: a
/// página não tem script nenhum, e não tem por que poder rodar um.
struct LegalWebView: UIViewRepresentable {
    let kind: LegalKind
    let locale: String
    var highlight: [String] = []
    /// Muda para pedir outra tentativa depois de uma falha.
    var reloadToken: Int = 0
    @Binding var meta: LegalMeta?
    @Binding var phase: LegalLoadPhase
    var onSwitch: (LegalKind) -> Void

    func makeCoordinator() -> Coordinator { Coordinator(self) }

    func makeUIView(context: Context) -> WKWebView {
        let config = WKWebViewConfiguration()
        config.defaultWebpagePreferences.allowsContentJavaScript = false
        config.dataDetectorTypes = []

        let webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = context.coordinator
        // Sem isto a página pisca branco antes do CSS escuro chegar.
        webView.isOpaque = false
        webView.backgroundColor = UIColor(LognDark.surfaceRaised)
        webView.scrollView.backgroundColor = UIColor(LognDark.surfaceRaised)
        webView.allowsLinkPreview = false
        webView.allowsBackForwardNavigationGestures = false
        webView.pageZoom = Self.textZoom
        context.coordinator.load(into: webView)
        return webView
    }

    func updateUIView(_ webView: WKWebView, context: Context) {
        context.coordinator.parent = self
        webView.pageZoom = Self.textZoom
        if context.coordinator.loadedKind != kind || context.coordinator.loadedToken != reloadToken {
            context.coordinator.load(into: webView)
        }
    }

    /// O tamanho de texto do sistema aplicado à página, até AX2 — o teto do design
    /// system para texto corrido.
    private static var textZoom: CGFloat {
        let category = UIApplication.shared.preferredContentSizeCategory
        let capped = min(category, .accessibilityExtraLarge)
        return UIFontMetrics(forTextStyle: .body)
            .scaledValue(for: 1, compatibleWith: UITraitCollection(preferredContentSizeCategory: capped))
    }

    final class Coordinator: NSObject, WKNavigationDelegate {
        var parent: LegalWebView
        var loadedKind: LegalKind?
        var loadedToken: Int?
        /// Já caiu na cópia do bundle nesta carga: uma segunda falha é falha de vez.
        private var usingBundle = false

        init(_ parent: LegalWebView) { self.parent = parent }

        func load(into webView: WKWebView) {
            loadedKind = parent.kind
            loadedToken = parent.reloadToken
            usingBundle = false
            report(phase: .loading, meta: nil)

            guard let url = remoteURL() else {
                loadBundled(into: webView)
                return
            }
            webView.load(URLRequest(url: url, cachePolicy: .useProtocolCachePolicy, timeoutInterval: 10))
        }

        private func remoteURL() -> URL? {
            guard let base = Bundle.main.object(forInfoDictionaryKey: "LogNApiBaseURL") as? String,
                  !base.isEmpty,
                  var components = URLComponents(string: base)
            else { return nil }
            components.path = "/legal/\(parent.kind.rawValue)"
            var query = [
                URLQueryItem(name: "embed", value: "1"),
                URLQueryItem(name: "lang", value: parent.locale),
            ]
            if !parent.highlight.isEmpty {
                query.append(URLQueryItem(name: "highlight", value: parent.highlight.joined(separator: ",")))
            }
            components.queryItems = query
            return components.url
        }

        private func loadBundled(into webView: WKWebView) {
            usingBundle = true
            let name = "legal-\(parent.kind.rawValue).\(parent.locale)"
            guard let file = Bundle.main.url(forResource: name, withExtension: "html") else {
                report(phase: .failed, meta: nil)
                return
            }
            report(phase: .loading, meta: bundledMeta())
            webView.loadFileURL(file, allowingReadAccessTo: file.deletingLastPathComponent())
        }

        /// Versão e data da cópia do bundle, do manifesto que o `legal_bundle.py` grava.
        private func bundledMeta() -> LegalMeta? {
            guard let url = Bundle.main.url(forResource: "legal-manifest", withExtension: "json"),
                  let data = try? Data(contentsOf: url),
                  let manifest = try? JSONDecoder().decode([String: [String: ManifestEntry]].self, from: data),
                  let entry = manifest[parent.kind.rawValue]?[parent.locale]
            else { return nil }
            return LegalMeta(version: entry.version, effectiveAt: entry.effective_at, draft: entry.draft, offline: true)
        }

        private struct ManifestEntry: Decodable {
            let version: Int
            let effective_at: String
            let draft: Bool
        }

        private func report(phase: LegalLoadPhase, meta: LegalMeta?) {
            DispatchQueue.main.async {
                self.parent.phase = phase
                if phase == .loading || meta != nil { self.parent.meta = meta }
            }
        }

        // MARK: WKNavigationDelegate

        func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
                     decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
            // A carga que este coordenador pediu passa; o que o jogador tocou, decide-se.
            guard action.navigationType == .linkActivated, let url = action.request.url else {
                decisionHandler(.allow)
                return
            }
            switch url.path {
            case "/legal/terms", "/legal/privacy":
                decisionHandler(.cancel)
                if let other = LegalKind(rawValue: url.lastPathComponent) {
                    DispatchQueue.main.async { self.parent.onSwitch(other) }
                }
                return
            default:
                break
            }
            if url.scheme == "mailto" || url.scheme == "https" {
                UIApplication.shared.open(url)
            }
            decisionHandler(.cancel)
        }

        func webView(_ webView: WKWebView, decidePolicyFor response: WKNavigationResponse,
                     decisionHandler: @escaping (WKNavigationResponsePolicy) -> Void) {
            guard let http = response.response as? HTTPURLResponse else {
                decisionHandler(.allow)
                return
            }
            guard http.statusCode == 200 else {
                // 503 de rascunho, 404, 500: melhor a cópia do aparelho do que uma
                // página de erro crua.
                decisionHandler(.cancel)
                loadBundled(into: webView)
                return
            }
            let header = { (name: String) in http.value(forHTTPHeaderField: name) }
            if let version = header("X-LogN-Legal-Version").flatMap(Int.init),
               let effective = header("X-LogN-Legal-Effective") {
                report(phase: .loading, meta: LegalMeta(
                    version: version, effectiveAt: effective,
                    draft: header("X-LogN-Legal-Draft") == "1", offline: false))
            }
            decisionHandler(.allow)
        }

        func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
            report(phase: .loaded, meta: nil)
        }

        func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
            fail(webView, error)
        }

        func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
            fail(webView, error)
        }

        private func fail(_ webView: WKWebView, _ error: Error) {
            // Cancelar a resposta para cair no bundle também chega aqui, como "carga
            // interrompida": essa não é falha, é a troca que acabou de ser pedida.
            let nsError = error as NSError
            if nsError.domain == "WebKitErrorDomain" && nsError.code == 102 { return }
            if usingBundle {
                report(phase: .failed, meta: nil)
            } else {
                loadBundled(into: webView)
            }
        }
    }
}
