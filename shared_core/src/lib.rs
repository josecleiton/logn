pub mod app;

use app::LogNApp;
use crux_core::Core;

uniffi::setup_scaffolding!();

// Envoltório para exportar para o iOS via UniFFI
#[derive(uniffi::Object)]
pub struct LogNCore {
    core: Core<LogNApp>,
}

#[uniffi::export]
impl LogNCore {
    #[uniffi::constructor]
    pub fn new() -> Self {
        Self {
            core: Core::new(),
        }
    }

    pub fn process_event(&self, _event: String) -> Vec<u8> {
        // Lógica de FFI do Crux será expandida futuramente
        vec![]
    }
}
