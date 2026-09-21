import Foundation
import LogNCoreFFI
import App

public class CoreWrapper: ObservableObject {
    private let coreFFI = CoreFFI()
    
    @Published public var viewModel: ViewModel
    
    public init() {
        self.viewModel = ViewModel(displayStatus: "Initializing...", pendingSyncCount: 0, isSyncing: false)
        updateViewModel()
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
            }
        }
    }
    
    private func handleHttp(id: UInt32, request: HttpRequest) {
        guard let url = URL(string: request.url) else {
            resolveEffect(id: id, result: .err(HttpError.io("Invalid URL")))
            return
        }
        
        var urlRequest = URLRequest(url: url)
        urlRequest.httpMethod = request.method
        urlRequest.httpBody = Data(request.body)
        
        for header in request.headers {
            urlRequest.setValue(header.value, forHTTPHeaderField: header.name)
        }
        
        URLSession.shared.dataTask(with: urlRequest) { data, response, error in
            let result: HttpResult
            
            if let error = error {
                result = .err(HttpError.io(error.localizedDescription))
            } else if let httpResponse = response as? HTTPURLResponse {
                let res = HttpResponse(
                    status: UInt16(httpResponse.statusCode),
                    headers: [], // Simplifying for now
                    body: data.map { [UInt8]($0) } ?? []
                )
                result = .ok(res)
            } else {
                result = .err(HttpError.io("Unknown response type"))
            }
            
            DispatchQueue.main.async {
                self.resolveEffect(id: id, result: result)
            }
        }.resume()
    }
    
    private func resolveEffect(id: UInt32, result: HttpResult) {
        do {
            let resultBytes = try result.bincodeSerialize()
            let nextEffectsBytes = coreFFI.resolve(id: id, data: Data(resultBytes))
            
            try processEffects(nextEffectsBytes)
        } catch {
            print("Failed to resolve HTTP effect: \(error)")
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
