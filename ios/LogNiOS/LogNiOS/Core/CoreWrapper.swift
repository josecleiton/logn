import Foundation
import Security

import LogNCoreFFI
import App
import LogN
import PostHog

public class CoreWrapper: ObservableObject {
    private let coreFFI = CoreFFI()
    
    @Published public var viewModel: ViewModel
    
    public init() {
        self.viewModel = ViewModel(
            displayStatus: "Initializing...",
            pendingSyncCount: 0,
            isSyncing: false,
            isFetching: false,
            isAuthenticating: false,
            hasAccessToken: false,
            isGuest: false,
            challenges: [],
            nodes: [],
            otpEmail: "",
            otpVerified: false,
            globalXp: 0,
            bugsFound: 0,
            dryRunsCompleted: 0,
            matchView: MatchViewModel(
                isActive: false, currentLetter: "", currentTitle: "", currentDescription: "",
                currentTemplateType: "", currentCodeLines: [], currentOptions: [],
                maxSelections: 0, lives: 0, maxLives: 0, penaltyMinutes: 0,
                contestSeconds: 0, questionSeconds: 0, isFrozen: false,
                totalProblems: 0, solvedCount: 0, balloonStates: [],
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
            scoreboard: []
        )
        updateViewModel()
        // Auto-login on init
        dispatch(event: .attemptRefresh)
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
                handleTelemetry(operation: operation)
            case .monitoring(let operation):
                handleMonitoring(operation: operation)
            }
        }
    }
    
    private func handleMonitoring(operation: MonitoringOperation) {
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
    }

    private func handleSecureStore(id: UInt32, operation: LogN.KeyValueOperation) {
        let result: LogN.KeyValueResult
        
        // Simulating Secure Vault with UserDefaults for MVP
        switch operation {
        case .get(let key):
            if let strValue = UserDefaults.standard.string(forKey: key) {
                let bytes = Array(strValue.utf8)
                result = .ok(response: LogN.KeyValueResponse.get(value: LogN.Value.bytes(bytes)))
            } else {
                result = .ok(response: LogN.KeyValueResponse.get(value: LogN.Value.none))
            }
        case .set(let key, let valueBytes):
            let valueStr = String(bytes: valueBytes, encoding: .utf8) ?? ""
            let previousStr = UserDefaults.standard.string(forKey: key)
            
            UserDefaults.standard.set(valueStr, forKey: key)
            
            if let prev = previousStr {
                result = .ok(response: LogN.KeyValueResponse.set(previous: LogN.Value.bytes(Array(prev.utf8))))
            } else {
                result = .ok(response: LogN.KeyValueResponse.set(previous: LogN.Value.none))
            }
        case .delete(let key):
            let previousStr = UserDefaults.standard.string(forKey: key)
            UserDefaults.standard.removeObject(forKey: key)
            
            if let prev = previousStr {
                result = .ok(response: LogN.KeyValueResponse.delete(previous: LogN.Value.bytes(Array(prev.utf8))))
            } else {
                result = .ok(response: LogN.KeyValueResponse.delete(previous: LogN.Value.none))
            }
        case .listKeys(_, _):
            result = .err(error: LogN.KeyValueError.io(message: "listKeys unsupported"))
        case .exists(let key):
            let exists = UserDefaults.standard.object(forKey: key) != nil
            result = .ok(response: LogN.KeyValueResponse.exists(isPresent: exists))
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
                let res = LogN.HttpResponse(
                    status: UInt16(httpResponse.statusCode),
                    headers: [],
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
    
    private func handleTelemetry(operation: TelemetryOperation) {
        switch operation {
        case .identify(let userId):
            PostHogSDK.shared.identify(userId)
        case .track(let event, let properties):
            PostHogSDK.shared.capture(event, properties: properties)


        }
    }
