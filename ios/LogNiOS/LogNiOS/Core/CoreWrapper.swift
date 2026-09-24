import Foundation
import Security

import LogNCoreFFI
import App
import LogN
import PostHog

public class CoreWrapper: ObservableObject {
    private let coreFFI = CoreFFI()
    
    @Published public var viewModel: ViewModel

    /// Roteamento de tela, não regra de negócio: o visitante que toca em
    /// "Criar conta · salvar progresso" precisa cair no cadastro, não no login.
    @Published public var wantsRegistration = false

    /// Incrementado a cada pedido de avaliação vindo do Core. Contador em vez de Bool
    /// para que dois pedidos consecutivos cada um acione o onChange no ContentView.
    /// O Core envia via notify_shell — não há resolve; nunca chame coreFFI.resolve aqui.
    @Published public private(set) var reviewRequestCount = 0

    public init() {
        self.viewModel = ViewModel(
            status: .silent,
            pendingSyncCount: 0,
            isSyncing: false,
            isFetching: false,
            isAuthenticating: false,
            hasAccessToken: false,
            hasSession: false,
            isOfflineSession: false,
            trailFromBundle: false,
            trailGeneratedAt: "",
            matchLeft: false,
            isGuest: false,
            locale: "",
            challenges: [],
            nodes: [],
            otpEmail: "",
            accountEmail: "",
            otpVerified: false,
            globalXp: 0,
            bugsFound: 0,
            dryRunsCompleted: 0,
            level: 1,
            xpIntoLevel: 0,
            xpForLevel: 200,
            xpToNextLevel: 200,
            challengesCompleted: 0,
            balloonsUp: 0,
            justLoggedOut: false,
            passwordResetDone: false,
            displayName: "",
            matchView: MatchViewModel(
                isActive: false, currentLetter: "", currentTitle: "", currentDescription: .findTheBug,
                currentTemplateType: "", currentCodeLines: [], currentOptions: [],
                maxSelections: 0, currentOrigin: "", originSheet: "", originSheetPaused: false,
                leavePending: false, solvedSoFar: 0,
                lives: 0, maxLives: 0, penaltyMinutes: 0,
                contestSeconds: 0, questionSeconds: 0, isFrozen: false,
                totalProblems: 0, solvedCount: 0, balloonStates: [], xpEarned: 0,
                selectedLine: -1, answerString: "", dropTime: "", dropSpace: "",
                selectedTags: [], predictedOutput: "", watchVariables: [], watchNote: "",
                lastVerdict: "", errors: [], hasTrap: false, trapCategory: "",
                trapTitle: "", trapExplanation: ""
            ),
            contestName: "",
            standingsGlobal: [],
            standingsHome: [],
            userStanding: StandingRow(
                rank: 0, handle: "", university: "", solved: 0, penalty: 0,
                isUser: true, note: ""
            ),
            scoreboard: [],
            standingsAreSample: true,
            authCooldownSeconds: 0,
            resendCooldownSeconds: 0
        )
        updateViewModel()
        prepareKeychain()
        // A trilha que viaja no bundle entra antes de tudo: instalação nova e sem rede
        // não tem retrato guardado nem resposta do servidor, e sem isto o app abria com
        // uma trilha de mock que não existe no banco. O Core só usa o que estiver vazio.
        loadBundledTrail()
        // O Core não tem relógio: quem dá a hora é o shell, e sem ela ele não consegue
        // decidir se a sessão guardada ainda vale quando não há rede.
        dispatch(event: .tick(now: Int64(Date().timeIntervalSince1970)))
        // Auto-login on init
        dispatch(event: .attemptRefresh)
        // E a fila que ficou no disco da sessão anterior volta junto: responder
        // offline e fechar o app não pode apagar o progresso.
        dispatch(event: .restoreOfflineQueue)
    }
    
    /// Lê `trail-seed.json` do bundle e entrega ao Core.
    ///
    /// Gerado por `just seed-bundle` a partir da própria API, então o formato é o mesmo
    /// que o Core já lê das respostas HTTP. Ausência do arquivo não é erro fatal: o app
    /// segue buscando pela rede, como fazia antes de a semente existir.
    private func loadBundledTrail() {
        guard let url = Bundle.main.url(forResource: "trail-seed", withExtension: "json"),
              let json = try? String(contentsOf: url, encoding: .utf8)
        else {
            print("trail-seed.json não veio no bundle; rode `just seed-bundle`")
            return
        }
        dispatch(event: .bundledTrailLoaded(json: json))
    }

    public func dispatch(event: Event) {
        do {
            let bytes = try event.bincodeSerialize()
            let effectsBytes = coreFFI.update(data: Data(bytes))
            
            try processEffects(effectsBytes)
            
        } catch {
            print("Failed to dispatch event: \(error)")
        }
    }
    
    private func processEffects(_ effectsBytes: Data) throws {
        if effectsBytes.isEmpty { return }
        
        let requests = try Requests.bincodeDeserialize(input: [UInt8](effectsBytes)).value
        for request in requests {
            switch request.effect {
            case .render(_):
                updateViewModel()
            case .http(let httpRequest):
                handleHttp(id: request.id, request: httpRequest)
            case .secureStore(let operation):
                handleSecureStore(id: request.id, operation: operation)
            case .telemetry(let operation):
                handleTelemetry(id: request.id, operation: operation)
            case .monitoring(let operation):
                handleMonitoring(id: request.id, operation: operation)
            case .time(let operation):
                handleTime(id: request.id, operation: operation)
            case .storeReview(let operation):
                handleStoreReview(operation)
            }
        }
    }

    private func handleStoreReview(_ op: StoreReviewOperation) {
        switch op {
        case .requestReview:
            reviewRequestCount += 1
        }
        // notify_shell: o Core não espera resolve. Nunca chamar resolveUnitEffect aqui.
    }

    /// Temporizadores pedidos pelo Core e ainda não disparados, por id do `crux_time`.
    private var timers: [UInt64: DispatchWorkItem] = [:]

    /// A capability de tempo: a hora, e o aviso de que um prazo passou.
    ///
    /// O Core não tem relógio. A contagem de um 429 pede a hora aqui a cada segundo, em
    /// vez de descontar um por tique, porque o iOS congela o app em segundo plano: quem
    /// volta depois de um minuto tem de achar o botão liberado, não com 59 s pela frente.
    ///
    /// O pedido chega com os tipos do módulo `LogN`, mas a resposta só é gerada no
    /// módulo `App`, porque nenhum tipo de `LogN` a referencia. O bincode dos dois é o
    /// mesmo, então o id passa de um para o outro pelo valor.
    private func handleTime(id: UInt32, operation: LogN.TimeRequest) {
        switch operation {
        case .now:
            resolveTime(id: id, response: .now(instant: Self.instant(Date())))
        case .notifyAt(let timer, let instant):
            let at = Double(instant.seconds) + Double(instant.nanos) / 1_000_000_000
            schedule(id: id, timer: timer.value, after: max(0, at - Date().timeIntervalSince1970)) {
                .instantArrived(id: App.TimerId(value: timer.value))
            }
        case .notifyAfter(let timer, let duration):
            schedule(id: id, timer: timer.value, after: Double(duration.nanos) / 1_000_000_000) {
                .durationElapsed(id: App.TimerId(value: timer.value))
            }
        case .clear(let timer):
            timers.removeValue(forKey: timer.value)?.cancel()
            resolveTime(id: id, response: .cleared(id: App.TimerId(value: timer.value)))
        }
    }

    private func schedule(
        id: UInt32, timer: UInt64, after seconds: TimeInterval,
        response: @escaping () -> App.TimeResponse
    ) {
        let work = DispatchWorkItem { [weak self] in
            self?.timers.removeValue(forKey: timer)
            self?.resolveTime(id: id, response: response())
        }
        timers[timer] = work
        DispatchQueue.main.asyncAfter(deadline: .now() + seconds, execute: work)
    }

    private static func instant(_ date: Date) -> App.Instant {
        let t = date.timeIntervalSince1970
        let seconds = t.rounded(.down)
        return App.Instant(seconds: UInt64(seconds), nanos: UInt32((t - seconds) * 1_000_000_000))
    }

    private func resolveTime(id: UInt32, response: App.TimeResponse) {
        do {
            let bytes = try response.bincodeSerialize()
            let nextEffectsBytes = coreFFI.resolve(id: id, data: Data(bytes))
            try processEffects(nextEffectsBytes)
        } catch {
            print("Failed to resolve Time effect: \(error)")
        }
    }
    
    private func handleMonitoring(id: UInt32, operation: MonitoringOperation) {
        switch operation {
        case .logError(let message, let details):
            print("MONITORING ERROR: \(message) - \(details)")
            PostHogSDK.shared.capture("error", properties: [
                "message": message,
                "details": details
            ])
        case .startSpan(let name):
            print("MONITORING SPAN START: \(name)")
            PostHogSDK.shared.capture("span_started", properties: ["span_name": name])
        case .endSpan(let name):
            print("MONITORING SPAN END: \(name)")
            PostHogSDK.shared.capture("span_ended", properties: ["span_name": name])
        }

        resolveUnitEffect(id: id)
    }

    private func handleTelemetry(id: UInt32, operation: TelemetryOperation) {
        switch operation {
        case .identify(let userId):
            PostHogSDK.shared.identify(userId)
        case .track(let event, let properties):
            PostHogSDK.shared.capture(event, properties: properties)
        }

        resolveUnitEffect(id: id)
    }

    /// Devolve ao Core as capabilities cujo `Output` é `()` — telemetria e monitoramento.
    ///
    /// Sem isto o efeito fica pendurado para sempre. Custou o login inteiro: depois de
    /// `TokenStored` o Core encadeia `TelemetryOperation::Identify` e só renderiza quando
    /// recebe `TelemetrySent`, então o app autenticava, guardava o token e continuava
    /// desenhando a tela de login, sem erro nenhum. `()` serializa em zero bytes no bincode.
    private func resolveUnitEffect(id: UInt32) {
        DispatchQueue.main.async {
            do {
                let nextEffectsBytes = self.coreFFI.resolve(id: id, data: Data())
                try self.processEffects(nextEffectsBytes)
            } catch {
                print("Failed to resolve unit effect: \(error)")
            }
        }
    }

    /// Chaves que são credencial e por isso vão para o Keychain.
    ///
    /// O resto do que passa pelo `SecureStore` — fila offline, retrato da trilha, e-mail,
    /// prazo da sessão — continua em `UserDefaults`: são blobs que não autenticam ninguém,
    /// e a fila e o retrato crescem além do que o Keychain foi feito para guardar.
    private static let keychainKeys: Set<String> = ["refresh_token"]

    /// Marca, em `UserDefaults`, que esta instalação já passou por `prepareKeychain`.
    private static let keychainPreparedKey = "keychain_prepared"

    /// Acerta o Keychain antes de o Core pedir a primeira chave.
    ///
    /// Duas situações, e as duas só se resolvem na primeira abertura:
    /// - Instalação que veio da versão que guardava o token em `UserDefaults`: o token
    ///   muda para o Keychain e sai de lá, senão a atualização deslogava todo mundo.
    /// - Reinstalação: o Keychain sobrevive à desinstalação e o `UserDefaults` não.
    ///   Sem apagar, quem removeu o app para sair da conta voltava logado, com fila e
    ///   retrato zerados.
    private func prepareKeychain() {
        let defaults = UserDefaults.standard
        guard !defaults.bool(forKey: Self.keychainPreparedKey) else { return }

        for key in Self.keychainKeys {
            if let legacy = defaults.string(forKey: key) {
                Keychain.write(Array(legacy.utf8), forKey: key)
                defaults.removeObject(forKey: key)
            } else {
                Keychain.remove(forKey: key)
            }
        }
        defaults.set(true, forKey: Self.keychainPreparedKey)
    }

    private func storedBytes(forKey key: String) -> [UInt8]? {
        if Self.keychainKeys.contains(key) {
            return Keychain.read(forKey: key)
        }
        return UserDefaults.standard.string(forKey: key).map { Array($0.utf8) }
    }

    private func store(_ bytes: [UInt8], forKey key: String) {
        if Self.keychainKeys.contains(key) {
            Keychain.write(bytes, forKey: key)
        } else {
            UserDefaults.standard.set(String(bytes: bytes, encoding: .utf8) ?? "", forKey: key)
        }
    }

    private func removeStored(forKey key: String) {
        if Self.keychainKeys.contains(key) {
            Keychain.remove(forKey: key)
        } else {
            UserDefaults.standard.removeObject(forKey: key)
        }
    }

    private func handleSecureStore(id: UInt32, operation: LogN.KeyValueOperation) {
        let result: LogN.KeyValueResult

        func value(_ bytes: [UInt8]?) -> LogN.Value {
            bytes.map { LogN.Value.bytes($0) } ?? LogN.Value.none
        }

        switch operation {
        case .get(let key):
            result = .ok(response: LogN.KeyValueResponse.get(value: value(storedBytes(forKey: key))))
        case .set(let key, let valueBytes):
            let previous = storedBytes(forKey: key)
            store(valueBytes, forKey: key)
            result = .ok(response: LogN.KeyValueResponse.set(previous: value(previous)))
        case .delete(let key):
            let previous = storedBytes(forKey: key)
            removeStored(forKey: key)
            result = .ok(response: LogN.KeyValueResponse.delete(previous: value(previous)))
        case .listKeys(_, _):
            result = .err(error: LogN.KeyValueError.io(message: "listKeys unsupported"))
        case .exists(let key):
            result = .ok(response: LogN.KeyValueResponse.exists(isPresent: storedBytes(forKey: key) != nil))
        }

        DispatchQueue.main.async {
            self.resolveSecureStore(id: id, result: result)
        }
    }
    
    private func getBaseURL() -> URL? {
        guard let value = Bundle.main.object(forInfoDictionaryKey: "LogNApiBaseURL") as? String,
              !value.isEmpty
        else {
            return nil
        }
        return URL(string: value)
    }

    private func handleHttp(id: UInt32, request: LogN.HttpRequest) {
        guard let base = getBaseURL() else {
            print("HTTP Request blocked: No API_BASE_URL configured in environment")
            resolveHttpEffect(id: id, result: .err(HttpError.io("No API base URL configured")))
            return
        }
        
        guard let url = URL(string: request.url, relativeTo: base) else {
            resolveHttpEffect(id: id, result: .err(HttpError.url(request.url)))
            return
        }
        
        var urlRequest = URLRequest(url: url)
        urlRequest.httpMethod = request.method
        urlRequest.httpBody = Data(request.body)
        
        for header in request.headers {
            urlRequest.setValue(header.value, forHTTPHeaderField: header.name)
        }
        
        URLSession.shared.dataTask(with: urlRequest) { data, response, error in
            let result: LogN.HttpResult
            
            if let error = error {
                result = .err(HttpError.io(error.localizedDescription))
            } else if let httpResponse = response as? HTTPURLResponse {
                // Os cabeçalhos iam vazios, e o Core não tinha como ler o `Retry-After`
                // de um 429: a contagem do botão saía sempre do valor padrão.
                let headers = httpResponse.allHeaderFields.compactMap { key, value -> LogN.HttpHeader? in
                    guard let name = key as? String else { return nil }
                    return LogN.HttpHeader(name: name, value: "\(value)")
                }
                let res = LogN.HttpResponse(
                    status: UInt16(httpResponse.statusCode),
                    headers: headers,
                    body: data.map { [UInt8]($0) } ?? []
                )
                result = .ok(res)
            } else {
                result = .err(HttpError.io("Unknown response type"))
            }
            
            DispatchQueue.main.async {
                self.resolveHttpEffect(id: id, result: result)
            }
        }.resume()
    }
    
    private func resolveHttpEffect(id: UInt32, result: LogN.HttpResult) {
        do {
            let resultBytes = try result.bincodeSerialize()
            let nextEffectsBytes = coreFFI.resolve(id: id, data: Data(resultBytes))
            
            try processEffects(nextEffectsBytes)
        } catch {
            print("Failed to resolve HTTP effect: \(error)")
        }
    }
    
    private func resolveSecureStore(id: UInt32, result: LogN.KeyValueResult) {
        do {
            let resultBytes = try result.bincodeSerialize()
            let nextEffectsBytes = coreFFI.resolve(id: id, data: Data(resultBytes))
            
            try processEffects(nextEffectsBytes)
        } catch {
            print("Failed to resolve SecureStore effect: \(error)")
        }
    }
    
    private func updateViewModel() {
        do {
            let viewBytes = coreFFI.view()
            if !viewBytes.isEmpty {
                self.viewModel = try ViewModel.bincodeDeserialize(input: [UInt8](viewBytes))
            }
        } catch {
            print("Failed to deserialize ViewModel: \(error)")
        }
    }
}

/// Item genérico de senha no Keychain, um por chave do `SecureStore`.
///
/// `AfterFirstUnlockThisDeviceOnly`: o refresh pode rodar com a tela bloqueada depois do
/// primeiro desbloqueio, e o token não viaja em backup para outro aparelho — lá ele
/// seria uma sessão que ninguém abriu.
private enum Keychain {
    private static var service: String {
        Bundle.main.bundleIdentifier ?? "LogN"
    }

    private static func query(forKey key: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
        ]
    }

    static func read(forKey key: String) -> [UInt8]? {
        var query = query(forKey: key)
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        guard status == errSecSuccess, let data = item as? Data else {
            if status != errSecItemNotFound {
                print("Keychain read failed for \(key): \(status)")
            }
            return nil
        }
        return Array(data)
    }

    static func write(_ bytes: [UInt8], forKey key: String) {
        let attributes: [String: Any] = [
            kSecValueData as String: Data(bytes),
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
        ]
        var status = SecItemUpdate(query(forKey: key) as CFDictionary, attributes as CFDictionary)
        if status == errSecItemNotFound {
            let insert = query(forKey: key).merging(attributes) { _, new in new }
            status = SecItemAdd(insert as CFDictionary, nil)
        }
        if status != errSecSuccess {
            print("Keychain write failed for \(key): \(status)")
        }
    }

    static func remove(forKey key: String) {
        let status = SecItemDelete(query(forKey: key) as CFDictionary)
        if status != errSecSuccess && status != errSecItemNotFound {
            print("Keychain delete failed for \(key): \(status)")
        }
    }
}
