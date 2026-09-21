use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crux_http::protocol::{HttpRequest, HttpResult};
use crux_kv::{KeyValueOperation, KeyValueResult, KeyValueResponse, KeyValueError};
use crate::domain::{GameEvent, SyncPayload, Challenge};

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[repr(C)]
#[facet(fg::namespace = "LogN")]
pub enum Event {
    Ping,
    Pong,
    Login { email: String, password_hash: String },
    LoginCompleted(HttpResult),
    TokenStored(KeyValueResult),
    AttemptRefresh,
    TokenRead(KeyValueResult),
    RefreshCompleted(HttpResult),
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
    SubmitChallengeAnswer { 
        action_id: String,
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
    pub access_token: Option<String>,
    pub is_syncing: bool,
    pub is_fetching: bool,
    pub is_authenticating: bool,
    pub pending_retry_event: Option<Event>, // Para o interceptor 401
}

#[derive(Facet, Serialize, Deserialize, Default, Clone)]
#[facet(fg::namespace = "LogN")]
pub struct ViewModel {
    pub display_status: String,
    pub pending_sync_count: u32,
    pub is_syncing: bool,
    pub is_fetching: bool,
    pub is_authenticating: bool,
    pub has_access_token: bool,
    pub challenges: Vec<Challenge>,
}

#[effect(facet_typegen)]
#[facet(fg::namespace = "LogN")]
pub enum Effect {
    Render(RenderOperation),
    Http(HttpRequest),
    SecureStore(KeyValueOperation),
}

#[derive(Default)]
pub struct LogNApp;

// Helper to inject token if available
fn auth_headers(token: &Option<String>) -> Vec<crux_http::protocol::HttpHeader> {
    let mut headers = vec![
        crux_http::protocol::HttpHeader {
            name: "Content-Type".to_string(),
            value: "application/json".to_string(),
        }
    ];
    if let Some(t) = token {
        headers.push(crux_http::protocol::HttpHeader {
            name: "Authorization".to_string(),
            value: format!("Bearer {}", t),
        });
    }
    headers
}

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

            Event::Login { email, password_hash } => {
                model.is_authenticating = true;
                model.status = "Logging in...".to_string();

                let body = serde_json::json!({
                    "email": email,
                    "password": password_hash // Em prod mandaríamos em plaintext sobre TLS pra o Go rodar o Argon2
                });

                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "http://localhost:8080/api/v1/auth/login".to_string(),
                    headers: vec![crux_http::protocol::HttpHeader {
                        name: "Content-Type".to_string(),
                        value: "application/json".to_string(),
                    }],
                    body: serde_json::to_vec(&body).unwrap_or_default(),
                };

                Command::request_from_shell(request).then_send(Event::LoginCompleted)
            }

            Event::LoginCompleted(result) => {
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct AuthResp { access_token: String, refresh_token: String }
                        
                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            model.status = "Login successful!".to_string();
                            
                            // Salva refresh token no Keychain
                            return Command::request_from_shell(KeyValueOperation::Set { 
                                key: "refresh_token".to_string(), 
                                value: data.refresh_token.into_bytes() 
                            }).then_send(Event::TokenStored);
                        } else {
                            model.status = "Failed to parse auth response".to_string();
                            model.is_authenticating = false;
                        }
                    }
                    HttpResult::Ok(response) => {
                        model.status = format!("Login failed: {}", response.status);
                        model.is_authenticating = false;
                    }
                    HttpResult::Err(_) => {
                        model.status = "Network error on login".to_string();
                        model.is_authenticating = false;
                    }
                }
                render::render()
            }

            Event::TokenStored(_) => {
                model.is_authenticating = false;
                render::render()
            }

            Event::AttemptRefresh => {
                model.status = "Refreshing session...".to_string();
                Command::request_from_shell(KeyValueOperation::Get { key: "refresh_token".to_string() })
                    .then_send(Event::TokenRead)
            }

            Event::TokenRead(result) => {
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value } } = result {
                    if let crux_kv::Value::Bytes(bytes) = value {
                        let rt = String::from_utf8(bytes).unwrap_or_default();
                        let body = serde_json::json!({ "refresh_token": rt });
                        
                        let request = HttpRequest {
                            method: "POST".to_string(),
                            url: "http://localhost:8080/api/v1/auth/refresh".to_string(),
                            headers: vec![crux_http::protocol::HttpHeader {
                                name: "Content-Type".to_string(),
                                value: "application/json".to_string(),
                            }],
                            body: serde_json::to_vec(&body).unwrap_or_default(),
                        };
                        return Command::request_from_shell(request).then_send(Event::RefreshCompleted);
                    }
                }
                model.status = "Session expired. Please login again.".to_string();
                model.access_token = None;
                render::render()
            }

            Event::RefreshCompleted(result) => {
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct RefreshResp { access_token: String }
                        
                        if let Ok(data) = serde_json::from_slice::<RefreshResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            model.status = "Session refreshed!".to_string();
                            
                            // Re-trigger pending event if any
                            if let Some(pending) = model.pending_retry_event.take() {
                                return self.update(pending, model);
                            }
                        } else {
                            model.status = "Failed to parse refresh response".to_string();
                        }
                    }
                    _ => {
                        model.status = "Session expired. Please login again.".to_string();
                        model.access_token = None;
                    }
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
                    headers: auth_headers(&model.access_token),
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
                        } else if response.status == 401 {
                            // Intercept 401 and attempt refresh
                            model.pending_retry_event = Some(Event::FetchChallenges);
                            return self.update(Event::AttemptRefresh, model);
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
                    headers: auth_headers(&model.access_token),
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
                        } else if response.status == 401 {
                            // Intercept 401 and attempt refresh
                            model.pending_retry_event = Some(Event::SyncNow);
                            return self.update(Event::AttemptRefresh, model);
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
            is_authenticating: model.is_authenticating,
            has_access_token: model.access_token.is_some(),
            challenges: model.challenges.clone(),
        }
    }
}
