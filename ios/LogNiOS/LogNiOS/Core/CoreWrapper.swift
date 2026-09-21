import Foundation
import LogNCoreFFI
import App

public class CoreWrapper: ObservableObject {
    private let coreFFI = CoreFFI()
    
    @Published public var viewModel: ViewModel
    
    public init() {
        self.viewModel = ViewModel(displayStatus: "Initializing...", pendingSyncCount: 0)
        updateViewModel()
    }
    
    public func dispatch(event: Event) {
        do {
            let bytes = try event.bincodeSerialize()
            let effectBytes = coreFFI.update(data: Data(bytes))
            
            // Aqui iteraríamos e processaríamos as Effects se houvesse lógicas nativas (ex: HTTP, DB nativo).
            // Para o LogN, o View(Model) é renderizado passivamente:
            updateViewModel()
        } catch {
            print("Failed to dispatch event: \(error)")
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
