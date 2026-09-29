import Foundation
import PostHog

/// Liga o PostHog, com a escolha do interruptor "Análise de uso".
///
/// A abertura e o fechamento do app são capturados pelo próprio SDK, e o Core não os
/// enxerga — por isso o interruptor chega até aqui. Desligado, o SDK sobe sem essa
/// captura; o Core, do lado dele, para de mandar identificação e eventos de uso. Erros
/// e medições de desempenho seguem saindo, com identificador anônimo.
enum Telemetry {
    /// A mesma chave que o Core grava pelo `SecureStore` (que aqui é `UserDefaults`).
    /// O shell precisa lê-la antes de o Core existir, para iniciar o SDK certo.
    static let analyticsDisabledKey = "analytics_disabled"

    static var analyticsEnabled: Bool {
        UserDefaults.standard.string(forKey: analyticsDisabledKey) != "1"
    }

    /// Inicia o SDK. Sem chave de telemetria (o padrão em desenvolvimento), não inicia.
    static func start(analyticsEnabled: Bool = Telemetry.analyticsEnabled) {
        guard let key = Bundle.main.object(forInfoDictionaryKey: "LogNTelemetryKey") as? String, !key.isEmpty,
              let host = Bundle.main.object(forInfoDictionaryKey: "LogNPostHogHost") as? String, !host.isEmpty
        else {
            print("PostHog telemetry is disabled (no key provided)")
            return
        }
        let config = PostHogConfig(projectToken: key, host: host)
        config.captureApplicationLifecycleEvents = analyticsEnabled
        // Os logs do Core saem com o nome do serviço, para separar dos de outro cliente
        // (Android) no mesmo projeto. O resto (lote, buffer, teto por janela) fica no
        // padrão do SDK.
        config.logs.serviceName = "logn-ios"
        // Crash do app vira `$exception` no Error Tracking, mandado na abertura seguinte.
        // É o único jeito de ver erro crítico: o que derruba o app não passa pelo Core.
        // A política de privacidade cita (seção de dados técnicos).
        config.errorTrackingConfig.autoCapture = true
        // O IP do cliente não se desliga aqui: o SDK não tem essa opção. Descartar o
        // IP é o "Discard client IP data" nas configurações do projeto em PostHog, que
        // a política de privacidade promete.
        PostHogSDK.shared.setup(config)
    }

    /// O interruptor mudou: o SDK é encerrado e iniciado de novo com a captura
    /// automática ligada ou desligada. Vale na hora, não só na próxima abertura.
    static func apply(analyticsEnabled: Bool) {
        PostHogSDK.shared.close()
        start(analyticsEnabled: analyticsEnabled)
    }
}
