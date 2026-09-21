pub mod domain;
pub mod match_engine;
pub mod mock_data;

pub mod app;

use std::panic::{AssertUnwindSafe, catch_unwind};
use app::LogNApp;
use crux_core::{
    Core,
    bridge::{Bridge, EffectId},
};

pub struct CoreFFI {
    core: Bridge<LogNApp>,
}

impl Default for CoreFFI {
    fn default() -> Self {
        Self::new()
    }
}

fn guarded<F: FnOnce() -> Vec<u8>>(_operation: &str, body: F) -> Vec<u8> {
    catch_unwind(AssertUnwindSafe(body)).unwrap_or_else(|_| {
        Vec::new() 
    })
}

fn skew(_operation: &str, _error: &dyn std::fmt::Display) -> Vec<u8> {
    Vec::new() 
}

#[boltffi::export]
impl CoreFFI {
    #[must_use]
    pub fn new() -> Self {
        Self {
            core: Bridge::new(Core::new()),
        }
    }

    #[must_use]
    pub fn update(&self, data: &[u8]) -> Vec<u8> {
        guarded("update", || {
            let mut effects = Vec::new();
            match self.core.update(data, &mut effects) {
                Ok(()) => effects,
                Err(e) => skew("update", &e),
            }
        })
    }

    #[must_use]
    pub fn resolve(&self, id: u32, data: &[u8]) -> Vec<u8> {
        guarded("resolve", || {
            let mut effects = Vec::new();
            match self.core.resolve(EffectId(id), data, &mut effects) {
                Ok(()) => effects,
                Err(e) => skew("resolve", &e),
            }
        })
    }

    #[must_use]
    pub fn view(&self) -> Vec<u8> {
        guarded("view", || {
            let mut view_model = Vec::new();
            match self.core.view(&mut view_model) {
                Ok(()) => view_model,
                Err(e) => skew("view", &e),
            }
        })
    }
}
