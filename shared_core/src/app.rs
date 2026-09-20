use crux_core::render::Render;
use crux_core::App;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug)]
pub enum Event {
    Ping,
    Pong,
}

#[derive(Serialize, Deserialize, Default, Clone)]
pub struct Model {
    pub status: String,
}

#[derive(Serialize, Deserialize, Default, Clone)]
pub struct ViewModel {
    pub display_status: String,
}

pub struct LogNApp;

impl Default for LogNApp {
    fn default() -> Self {
        Self
    }
}

impl App for LogNApp {
    type Event = Event;
    type Model = Model;
    type ViewModel = ViewModel;
    type Capabilities = crux_core::render::Render<Event>;

    fn update(&self, event: Self::Event, model: &mut Self::Model, caps: &Self::Capabilities) {
        match event {
            Event::Ping => {
                model.status = "Pong received!".to_string();
                caps.render();
            }
            Event::Pong => {}
        }
    }

    fn view(&self, model: &Self::Model) -> Self::ViewModel {
        ViewModel {
            display_status: model.status.clone(),
        }
    }
}
