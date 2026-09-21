use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crate::domain::GameEvent;

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[repr(C)]
#[facet(fg::namespace = "LogN")]
pub enum Event {
    Ping,
    Pong,
    RegisterAction {
        action_id: String,
        action_type: String,
        payload_json: String,
        timestamp: i64,
    },
}

#[derive(Facet, Serialize, Deserialize, Default, Clone)]
#[facet(fg::namespace = "LogN")]
pub struct Model {
    pub status: String,
    pub pending_events: Vec<GameEvent>,
    pub last_hash: String,
}

#[derive(Facet, Serialize, Deserialize, Default, Clone)]
#[facet(fg::namespace = "LogN")]
pub struct ViewModel {
    pub display_status: String,
    pub pending_sync_count: u32,
}

#[effect(facet_typegen)]
#[facet(fg::namespace = "LogN")]
pub enum Effect {
    Render(RenderOperation),
    // Http(crux_http::HttpOperation) will be added here
}

#[derive(Default)]
pub struct LogNApp;

impl App for LogNApp {
    type Event = Event;
    type Model = Model;
    type ViewModel = ViewModel;
    type Effect = Effect;

    fn update(&self, event: Self::Event, model: &mut Self::Model) -> Command<Self::Effect, Self::Event> {
        match event {
            Event::Ping => {
                model.status = "Pong received!".to_string();
                render::render()
            }
            Event::Pong => Command::done(),
            Event::RegisterAction { action_id, action_type, payload_json, timestamp } => {
                let previous_hash = if model.last_hash.is_empty() {
                    "0000000000000000000000000000000000000000000000000000000000000000".to_string()
                } else {
                    model.last_hash.clone()
                };

                let game_event = GameEvent::new(
                    action_id,
                    action_type,
                    payload_json,
                    timestamp,
                    previous_hash,
                );

                model.last_hash = game_event.current_hash.clone();
                model.pending_events.push(game_event);
                model.status = "Action Registered".to_string();

                render::render()
            }
        }
    }

    fn view(&self, model: &Self::Model) -> Self::ViewModel {
        ViewModel {
            display_status: model.status.clone(),
            pending_sync_count: model.pending_events.len() as u32,
        }
    }
}

#[cfg(test)]
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_register_action_chaining() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let _cmd1 = app.update(Event::RegisterAction {
            action_id: "evt1".to_string(),
            action_type: "SOLVE".to_string(),
            payload_json: "{}".to_string(),
            timestamp: 1600,
        }, &mut model);

        assert_eq!(model.pending_events.len(), 1);
        let first_hash = model.last_hash.clone();
        assert!(!first_hash.is_empty());

        let _cmd2 = app.update(Event::RegisterAction {
            action_id: "evt2".to_string(),
            action_type: "SKIP".to_string(),
            payload_json: "{}".to_string(),
            timestamp: 1605,
        }, &mut model);

        assert_eq!(model.pending_events.len(), 2);
        assert_eq!(model.pending_events[1].previous_hash, first_hash);
        assert_ne!(model.last_hash, first_hash);
    }
}
