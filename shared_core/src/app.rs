use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crux_http::protocol::{HttpRequest, HttpResult};
use crux_kv::{KeyValueOperation, KeyValueResult, KeyValueResponse};
use crate::domain::{GameEvent, SyncPayload, Challenge, TelemetryOperation};

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[repr(C)]
#[facet(fg::namespace = "LogN")]
pub enum Event {
    TelemetrySent,
    Ping,
    Pong,
    Login { email: String, password_hash: String },
    LoginCompleted(HttpResult),
    ContinueAsGuest,
    TokenStored(KeyValueResult),
    AttemptRefresh,
    TokenRead(KeyValueResult),
    RefreshCompleted(HttpResult),
    FetchChallenges,
    FetchNodes,
    NodesFetched(HttpResult),
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
    RequestOTP { email: String, purpose: String },
    OTPRequested(HttpResult),
    VerifyOTP { email: String, code: String, purpose: String },
    OTPVerified(HttpResult),
    Register { email: String, password: String, otp: String },
    RegisterCompleted(HttpResult),
    ResetPassword { email: String, new_password: String, otp: String },
    ResetPasswordCompleted(HttpResult),
    ChallengeAnswered { challenge_id: String, node_id: String, is_correct: bool, timestamp: i64 },
    QueueLoadedForHash {
        challenge_id: String,
        node_id: String,
        is_correct: bool,
        timestamp: i64,
        result: KeyValueResult,
    },
    QueueSavedForSync(KeyValueResult),
}

#[derive(Default, Clone)]
pub struct Model {
    pub status: String,
    pub pending_events: Vec<GameEvent>,
    pub challenges: Vec<Challenge>,
    pub nodes: Vec<crate::domain::SkillNode>,
    pub last_hash: String,
    pub user_id: String,
    pub access_token: Option<String>,
    pub is_syncing: bool,
    pub is_fetching: bool,
    pub is_authenticating: bool,
    pub is_guest: bool,
    pub pending_retry_event: Option<Event>, // Para o interceptor 401
    pub otp_email: String,
    pub otp_verified: bool,
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
    pub is_guest: bool,
    pub challenges: Vec<Challenge>,
    pub nodes: Vec<crate::domain::SkillNode>,
    pub otp_email: String,
    pub otp_verified: bool,
}

#[effect(facet_typegen)]
#[facet(fg::namespace = "LogN")]
pub enum Effect {
    Render(RenderOperation),
    Http(HttpRequest),
    SecureStore(KeyValueOperation),
    Telemetry(crate::domain::TelemetryOperation),
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

            Event::TelemetrySent => {
                render::render()
            }
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
                    url: "/api/v1/auth/login".to_string(),
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
                            model.is_guest = false;
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

            Event::ContinueAsGuest => {
                model.is_guest = true;
                render::render()
            }

            Event::TokenStored(_) => {
                model.is_authenticating = false;
                Command::request_from_shell(crate::domain::TelemetryOperation::Identify { user_id: model.user_id.clone() })
                    .then_send(|_| Event::TelemetrySent)
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
                            url: "/api/v1/auth/refresh".to_string(),
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
                model.is_authenticating = false;
                render::render()
            }

            Event::RefreshCompleted(result) => {
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct RefreshResp { access_token: String }
                        
                        if let Ok(data) = serde_json::from_slice::<RefreshResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            model.is_guest = false;
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
                model.is_authenticating = false;
                    }
                }
                render::render()
            }


            
            Event::FetchNodes => {
                model.is_fetching = true;
                model.status = "Fetching skill tree...".to_string();
                
                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: "/api/v1/nodes".to_string(),
                    headers: auth_headers(&model.access_token),
                    body: vec![],
                };
                
                
                Command::request_from_shell(request).then_send(Event::NodesFetched)
            }
            Event::NodesFetched(result) => {
                model.is_fetching = false;
                match result {
                    HttpResult::Ok(response) => {
                        if response.status == 200 {
                            if let Ok(nodes) = serde_json::from_slice::<Vec<crate::domain::SkillNode>>(&response.body) {
                                model.nodes = nodes;
                                model.status = "Skill tree loaded".to_string();
                            } else {
                                model.status = "Failed to parse nodes".to_string();
                            }
                            render::render()
                        } else if response.status == 401 {
                            model.pending_retry_event = Some(Event::FetchNodes);
                            Command::request_from_shell(KeyValueOperation::Get { key: "refresh_token".into() }).then_send(Event::TokenRead)
                        } else {
                            model.status = format!("Error: {}", response.status);
                            render::render()
                        }
                    }
                    HttpResult::Err(e) => {
                        model.status = "Network Error".to_string();
                        Command::request_from_shell(crate::domain::TelemetryOperation::LogError { 
                            message: "FetchNodes Failed".to_string(), 
                            details: format!("{:?}", e) 
                        }).then_send(|_| Event::TelemetrySent)
                    }
                }
            }
Event::FetchChallenges => {
                if model.is_fetching {
                    return Command::done();
                }
                model.is_fetching = true;
                model.status = "Fetching challenges...".to_string();

                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: "/api/v1/challenges".to_string(),
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
                    enriched_payload.push_str(&format!(", \"is_correct\": {}, \"challenge_id\": \"{}\", \"node_id\": \"{}\"}}", is_correct, challenge_id, ch.node_id));

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
                    
                    let mut props = std::collections::HashMap::new();
                    props.insert("challenge_id".to_string(), challenge_id.clone());
                    props.insert("is_correct".to_string(), is_correct.to_string());
                    
                    return Command::request_from_shell(crate::domain::TelemetryOperation::Track { 
                        event: "Challenge Answered".to_string(), 
                        properties: props 
                    }).then_send(|_| Event::TelemetrySent);
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
            Event::RequestOTP { email, purpose } => {
                model.is_authenticating = true;
                model.otp_email = email.clone();
                model.status = "Sending verification code...".to_string();

                let body = serde_json::json!({ "email": email, "purpose": purpose });
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/auth/request-otp".to_string(),
                    headers: vec![crux_http::protocol::HttpHeader {
                        name: "Content-Type".to_string(),
                        value: "application/json".to_string(),
                    }],
                    body: body.to_string().into_bytes(),
                };
                Command::request_from_shell(request).then_send(Event::OTPRequested)
            }
            Event::OTPRequested(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        model.status = "Code sent! Check your e-mail.".to_string();
                    }
                    _ => {
                        model.status = "Failed to send code.".to_string();
                    }
                }
                render::render()
            }
            Event::VerifyOTP { email, code, purpose } => {
                model.is_authenticating = true;
                model.status = "Verifying code...".to_string();

                let body = serde_json::json!({ "email": email, "code": code, "purpose": purpose });
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/auth/verify-otp".to_string(),
                    headers: vec![crux_http::protocol::HttpHeader {
                        name: "Content-Type".to_string(),
                        value: "application/json".to_string(),
                    }],
                    body: body.to_string().into_bytes(),
                };
                Command::request_from_shell(request).then_send(Event::OTPVerified)
            }
            Event::OTPVerified(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        model.otp_verified = true;
                        model.status = "E-mail verified!".to_string();
                    }
                    _ => {
                        model.otp_verified = false;
                        model.status = "Invalid or expired code.".to_string();
                    }
                }
                render::render()
            }
            Event::Register { email, password, otp } => {
                model.is_authenticating = true;
                model.status = "Creating account...".to_string();

                let body = serde_json::json!({ "email": email, "password": password, "otp": otp });
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/auth/register".to_string(),
                    headers: vec![crux_http::protocol::HttpHeader {
                        name: "Content-Type".to_string(),
                        value: "application/json".to_string(),
                    }],
                    body: body.to_string().into_bytes(),
                };
                Command::request_from_shell(request).then_send(Event::RegisterCompleted)
            }
            Event::RegisterCompleted(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct AuthResp { access_token: String, refresh_token: String }

                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            model.is_guest = false;
                            model.otp_verified = false;
                            model.otp_email = String::new();
                            model.status = "Account created!".to_string();

                            return Command::request_from_shell(KeyValueOperation::Set {
                                key: "refresh_token".to_string(),
                                value: data.refresh_token.into_bytes(),
                            }).then_send(Event::TokenStored);
                        }
                        model.status = "Failed to parse response".to_string();
                    }
                    _ => {
                        model.status = "Registration failed.".to_string();
                    }
                }
                render::render()
            }
            Event::ResetPassword { email, new_password, otp } => {
                model.is_authenticating = true;
                model.status = "Resetting password...".to_string();

                let body = serde_json::json!({ "email": email, "password": new_password, "otp": otp });
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/auth/reset-password".to_string(),
                    headers: vec![crux_http::protocol::HttpHeader {
                        name: "Content-Type".to_string(),
                        value: "application/json".to_string(),
                    }],
                    body: body.to_string().into_bytes(),
                };
                Command::request_from_shell(request).then_send(Event::ResetPasswordCompleted)
            }
            Event::ResetPasswordCompleted(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct AuthResp { access_token: String, refresh_token: String }

                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            model.is_guest = false;
                            model.otp_verified = false;
                            model.otp_email = String::new();
                            model.status = "Password updated!".to_string();

                            return Command::request_from_shell(KeyValueOperation::Set {
                                key: "refresh_token".to_string(),
                                value: data.refresh_token.into_bytes(),
                            }).then_send(Event::TokenStored);
                        }
                        model.status = "Failed to parse response".to_string();
                    }
                    _ => {
                        model.status = "Failed to reset password.".to_string();
                    }
                }
                render::render()
            }
            Event::SyncNow => {
                if model.is_guest && model.access_token.is_none() {
                    model.status = "Sign in to sync your progress!".to_string();
                    return Command::done();
                }
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
                    url: "/api/v1/sync".to_string(),
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
                            
                            // Flush the empty queue to disk so it doesn't duplicate
                            let bytes = serde_json::to_vec(&model.pending_events).unwrap_or_default();
                            return Command::request_from_shell(KeyValueOperation::Set {
                                key: "offline_events".to_string(),
                                value: bytes,
                            }).then_send(|_| Event::Ping); // Ping just as a dummy no-op event
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
            Event::ChallengeAnswered { challenge_id, node_id, is_correct, timestamp } => {
                Command::request_from_shell(KeyValueOperation::Get {
                    key: "offline_events".to_string(),
                }).then_send(move |result| Event::QueueLoadedForHash {
                    challenge_id: challenge_id.clone(),
                    node_id: node_id.clone(),
                    is_correct,
                    timestamp,
                    result,
                })
            }
            Event::QueueLoadedForHash { challenge_id, node_id, is_correct, timestamp, result } => {
                let mut queue: Vec<GameEvent> = Vec::new();
                
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result {
                    if let Ok(parsed) = serde_json::from_slice(&bytes) {
                        queue = parsed;
                    }
                }
                
                let previous_hash = queue.last().map(|e| e.current_hash.clone()).unwrap_or_else(|| "0000000000000000000000000000000000000000000000000000000000000000".to_string());
                
                let action_id = format!("evt_{}", timestamp);
                let payload_json = format!(r#"{{"challenge_id":"{}","node_id":"{}","is_correct":{}}}"#, challenge_id, node_id, is_correct);
                
                let event = GameEvent::new(
                    action_id,
                    "CHALLENGE_ANSWERED".to_string(),
                    payload_json,
                    timestamp,
                    previous_hash
                );
                
                queue.push(event);
                
                model.pending_events = queue.clone();
                
                let bytes = serde_json::to_vec(&queue).unwrap_or_default();
                Command::request_from_shell(KeyValueOperation::Set {
                    key: "offline_events".to_string(),
                    value: bytes,
                }).then_send(Event::QueueSavedForSync)
            }
            Event::QueueSavedForSync(_) => {
                if let Some(event) = model.pending_events.last() {
                    let is_correct = event.payload_json.contains("\"is_correct\":true");
                    let telemetry_event = if is_correct { "challenge_correct" } else { "challenge_incorrect" };
                    
                    return Command::request_from_shell(TelemetryOperation::Track {
                        event: telemetry_event.to_string(),
                        properties: std::collections::HashMap::from([
                            ("action_id".to_string(), event.id.clone()),
                        ])
                    }).then_send(|_| Event::SyncNow);
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
            is_guest: model.is_guest,
            challenges: model.challenges.clone(),
            nodes: model.nodes.clone(),
            otp_email: model.otp_email.clone(),
            otp_verified: model.otp_verified,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;


    #[test]
    fn test_otp_flow() {
        let app = LogNApp::default();
        let mut model = Model::default();

        // 1. Request OTP
        let mut cmd = app.update(Event::RequestOTP { 
            email: "test@example.com".into(), 
            purpose: "verify_email".into() 
        }, &mut model);
        
        assert_eq!(model.otp_email, "test@example.com");
        assert!(model.is_authenticating);
        
        let http_req = cmd.expect_one_effect();
        if let Effect::Http(r) = http_req {
            assert_eq!(r.operation.url, "/api/v1/auth/request-otp");
        } else {
            panic!("Expected Http effect");
        }

        // 2. OTP Requested Success
        let resp = crux_http::protocol::HttpResponse {
            status: 200,
            body: b"{}".to_vec(),
            headers: vec![],
        };
        let _ = app.update(Event::OTPRequested(HttpResult::Ok(resp)), &mut model);
        assert!(!model.is_authenticating);
        assert_eq!(model.status, "Code sent! Check your e-mail.");

        // 3. Verify OTP
        let mut cmd = app.update(Event::VerifyOTP { 
            email: "test@example.com".into(), 
            code: "123456".into(), 
            purpose: "verify_email".into() 
        }, &mut model);
        
        assert!(model.is_authenticating);
        
        let http_req = cmd.expect_one_effect();
        if let Effect::Http(r) = http_req {
            assert_eq!(r.operation.url, "/api/v1/auth/verify-otp");
            assert!(String::from_utf8_lossy(&r.operation.body).contains("123456"));
        } else {
            panic!("Expected Http effect");
        }

        // 4. OTP Verified Success
        let resp = crux_http::protocol::HttpResponse {
            status: 200,
            body: b"{}".to_vec(),
            headers: vec![],
        };
        let _ = app.update(Event::OTPVerified(HttpResult::Ok(resp)), &mut model);
        assert!(!model.is_authenticating);
        assert!(model.otp_verified);
        assert_eq!(model.status, "E-mail verified!");
    }

    #[test]
    fn test_login_flow() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut cmd = app.update(Event::Login { email: "test@x.com".into(), password_hash: "hash".into() }, &mut model);
        assert!(model.is_authenticating);
        
        let req = cmd.expect_one_effect();
        if let Effect::Http(http_req) = req {
            assert_eq!(http_req.operation.url, "/api/v1/auth/login");
        } else {
            panic!("Expected Http effect");
        }

        let body = serde_json::to_vec(&serde_json::json!({
            "access_token": "acc_tok",
            "refresh_token": "ref_tok"
        })).unwrap();
        
        let result = HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body });
        let mut cmd = app.update(Event::LoginCompleted(result), &mut model);
        
        assert_eq!(model.access_token, Some("acc_tok".into()));
        
        let kv_req = cmd.expect_one_effect();
        if let Effect::SecureStore(r) = kv_req {
            if let KeyValueOperation::Set { key, value } = r.operation {
                assert_eq!(key, "refresh_token");
                assert_eq!(String::from_utf8(value).unwrap(), "ref_tok");
            } else {
                panic!("Expected Set operation");
            }
        } else {
            panic!("Expected SecureStore effect");
        }
    }

    #[test]
    fn test_401_interceptor_flow() {
        let app = LogNApp::default();
        let mut model = Model::default();
        
        model.access_token = Some("old_tok".into());
        
        let result = HttpResult::Ok(crux_http::protocol::HttpResponse { status: 401, headers: vec![], body: vec![] });
        let mut cmd = app.update(Event::ChallengesFetched(result), &mut model);
        
        assert!(matches!(model.pending_retry_event, Some(Event::FetchChallenges)));
        
        let kv_req = cmd.expect_one_effect();
        if let Effect::SecureStore(r) = kv_req {
            if let KeyValueOperation::Get { key } = r.operation {
                assert_eq!(key, "refresh_token");
            } else {
                panic!("Expected Get operation");
            }
        } else {
            panic!("Expected SecureStore effect");
        }
        
        let kv_result = KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes("ref_tok".into()) } };
        let mut cmd = app.update(Event::TokenRead(kv_result), &mut model);
        
        let http_req = cmd.expect_one_effect();
        if let Effect::Http(r) = http_req {
            assert_eq!(r.operation.url, "/api/v1/auth/refresh");
        } else {
            panic!("Expected Http effect");
        }
        
        let body = serde_json::to_vec(&serde_json::json!({
            "access_token": "new_tok"
        })).unwrap();
        let result = HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body });
        let mut cmd = app.update(Event::RefreshCompleted(result), &mut model);
        
        assert_eq!(model.access_token, Some("new_tok".into()));
        assert!(model.pending_retry_event.is_none());
        
        let http_req = cmd.expect_one_effect();
        if let Effect::Http(r) = http_req {
            assert_eq!(r.operation.url, "/api/v1/challenges");
            assert_eq!(r.operation.headers[1].value, "Bearer new_tok");
        } else {
            panic!("Expected Http effect re-emitting original request");
        }
    }
}
