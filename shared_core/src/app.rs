use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crux_http::protocol::{HttpRequest, HttpResult};
use crate::domain::{GameEvent, SyncPayload, Challenge};

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[repr(C)]
#[facet(fg::namespace = "LogN")]
pub enum Event {
    Ping,
    Pong,
    FetchChallenges,
    ChallengesFetched(HttpResult),
    RegisterAction {
        action_id: String,
        action_type: String,
        payload_json: String,
        timestamp: i64,
    },
    SyncNow,
    SyncCompleted(HttpResult),
    SubmitChallengeAnswer { action_id: String, 
        challenge_id: String, 
        answer_json: String, 
        timestamp: i64, 
    },
}

#[derive(Default, Clone)]
pub struct Model {
    pub status: String,
    pub pending_events: Vec<GameEvent>,
    pub challenges: Vec<Challenge>,
    pub last_hash: String,
    pub user_id: String,
    pub is_syncing: bool,
    pub is_fetching: bool,
}

#[derive(Facet, Serialize, Deserialize, Default, Clone)]
#[facet(fg::namespace = "LogN")]
pub struct ViewModel {
    pub display_status: String,
    pub pending_sync_count: u32,
    pub is_syncing: bool,
    pub is_fetching: bool,
    pub challenges: Vec<Challenge>,
}

#[effect(facet_typegen)]
#[facet(fg::namespace = "LogN")]
pub enum Effect {
    Render(RenderOperation),
    Http(HttpRequest),
    SecureStore(crux_kv::KeyValueOperation),
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
            Event::SubmitChallengeAnswer { action_id, challenge_id, answer_json, timestamp } => {
                let challenge = model.challenges.iter().find(|c| c.id == challenge_id);
                if let Some(ch) = challenge {
                    let is_correct = if ch.template_type == "SPOT_THE_BUG" {
                        if let Some(ref correct_line) = ch.payload.validation.correct_line {
                            answer_json.contains(&format!("\"selected_line\": {}", correct_line)) ||
                            answer_json.contains(&format!("\"selected_line\":{}", correct_line))
                        } else {
                            false
                        }
                    } else if ch.template_type == "FILL_IN_THE_BLANK" {
                        if let Some(ref expected) = ch.payload.validation.expected_string {
                            answer_json.contains(&format!("\"answer_string\": \"{}\"", expected)) ||
                            answer_json.contains(&format!("\"answer_string\":\"{}\"", expected))
                        } else {
                            false
                        }
                    } else {
                        false
                    };

                    let status_str = if is_correct { "Correct!" } else { "Incorrect!" };
                    model.status = format!("Challenge {}: {}", challenge_id, status_str);

                    let mut enriched_payload = answer_json.clone();
                    enriched_payload.pop(); // Remove closing brace
                    enriched_payload.push_str(&format!(", \"is_correct\": {}}}", is_correct));

                    let previous_hash = if model.last_hash.is_empty() {
                        "0000000000000000000000000000000000000000000000000000000000000000".to_string()
                    } else {
                        model.last_hash.clone()
                    };

                    let game_event = GameEvent::new(
                        action_id,
                        "CHALLENGE_ANSWER".to_string(),
                        enriched_payload,
                        timestamp,
                        previous_hash,
                    );

                    model.last_hash = game_event.current_hash.clone();
                    model.pending_events.push(game_event);
                } else {
                    model.status = "Challenge not found!".to_string();
                }
                render::render()
            }
            Event::FetchChallenges => {
                if model.is_fetching {
                    return Command::done();
                }
                model.is_fetching = true;
                model.status = "Fetching challenges...".to_string();

                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: "http://localhost:8080/api/v1/challenges".to_string(),
                    headers: vec![],
                    body: vec![],
                };

                Command::request_from_shell(request)
                    .then_send(Event::ChallengesFetched)
            }
            Event::ChallengesFetched(result) => {
                model.is_fetching = false;
                match result {
                    HttpResult::Ok(response) => {
                        if response.status == 200 {
                            if let Ok(challenges) = serde_json::from_slice::<Vec<Challenge>>(&response.body) {
                                model.challenges = challenges;
                                model.status = "Challenges loaded".to_string();
                            } else {
                                model.status = "Failed to parse challenges".to_string();
                            }
                        } else {
                            model.status = format!("Failed to fetch: HTTP {}", response.status);
                        }
                    }
                    HttpResult::Err(_) => {
                        model.status = "Network Error loading challenges".to_string();
                    }
                }
                render::render()
            }
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
            Event::SyncNow => {
                if model.is_syncing || model.pending_events.is_empty() {
                    return Command::done();
                }
                
                model.is_syncing = true;
                model.status = "Syncing...".to_string();
                
                let user_id = if model.user_id.is_empty() { "user_1".to_string() } else { model.user_id.clone() };
                
                let payload = SyncPayload {
                    user_id,
                    events: model.pending_events.clone(),
                };
                
                let body_bytes = serde_json::to_vec(&payload).unwrap_or_default();
                
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "http://localhost:8080/api/v1/sync".to_string(),
                    headers: vec![
                        crux_http::protocol::HttpHeader {
                            name: "Content-Type".to_string(),
                            value: "application/json".to_string(),
                        }
                    ],
                    body: body_bytes,
                };
                
                Command::request_from_shell(request)
                    .then_send(Event::SyncCompleted)
            }
            Event::SyncCompleted(result) => {
                model.is_syncing = false;
                match result {
                    HttpResult::Ok(response) => {
                        if response.status == 200 {
                            model.status = "Sync Successful".to_string();
                            model.pending_events.clear();
                        } else if response.status == 409 {
                            model.status = "Sync Conflict - Rebase Required".to_string();
                        } else {
                            model.status = format!("Sync Failed: HTTP {}", response.status);
                        }
                    }
                    HttpResult::Err(_err) => {
                        model.status = "Sync Failed: Network Error".to_string();
                    }
                }
                render::render()
            }
        }
    }

    fn view(&self, model: &Self::Model) -> Self::ViewModel {
        ViewModel {
            display_status: model.status.clone(),
            pending_sync_count: model.pending_events.len() as u32,
            is_syncing: model.is_syncing,
            is_fetching: model.is_fetching,
            challenges: model.challenges.clone(),
        }
    }
}

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
