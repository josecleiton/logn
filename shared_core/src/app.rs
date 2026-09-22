use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crux_http::protocol::{HttpRequest, HttpResult};
use crux_kv::{KeyValueOperation, KeyValueResult, KeyValueResponse};
use crate::domain::{GameEvent, SyncPayload, Challenge, TelemetryOperation};
use crate::match_engine;

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
    AccountEmailStored(KeyValueResult),
    AccountEmailRead(KeyValueResult),
    AttemptRefresh,
    TokenRead(KeyValueResult),
    TokenCleared(KeyValueResult),
    RefreshCompleted(HttpResult),
    Logout,
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
    SyncAndLogout,
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
    // Match Events
    StartMatch { node_id: String },
    MatchSelectLine { line: i32 },
    MatchSetAnswer { answer: String },
    MatchSetDropTime { value: String },
    MatchSetDropSpace { value: String },
    MatchToggleTag { tag: String },
    MatchSetOutput { value: String },
    MatchSubmit { timestamp: i64 },
    MatchDismissTrap,
    MatchTimerTick,
    // Logout
    UndoLogout,
    DismissLogoutNotice,
}

/// O que o desfazer devolve. Existe só entre a saída e o jogador deixar a tela.
#[derive(Clone, Default)]
pub struct LogoutSnapshot {
    pub access_token: Option<String>,
    pub was_guest: bool,
    pub email: String,
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
    /// E-mail em trânsito no fluxo de OTP. Vive só até o código ser verificado.
    pub otp_email: String,
    /// E-mail da sessão. Diferente do `otp_email`, sobrevive ao login e é reposto
    /// do armazenamento seguro quando o app abre direto pelo refresh token —
    /// sem ele o perfil de quem entrou por senha mostrava "?" como se fosse visitante.
    pub account_email: String,
    pub otp_verified: bool,
    pub global_xp: i32,
    pub bugs_found: i32,
    pub dry_runs_completed: i32,
    pub match_state: Option<match_engine::MatchState>,
    pub logout_undo: Option<LogoutSnapshot>,
    /// Marcado por `SyncAndLogout`: a saída espera a fila subir.
    pub logout_after_sync: bool,
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
    /// E-mail da conta em sessão. Vazio no visitante.
    pub account_email: String,
    pub otp_verified: bool,
    pub global_xp: i32,
    pub bugs_found: i32,
    pub dry_runs_completed: i32,
    /// Progressão. Calculada aqui, nunca no cliente.
    pub level: i32,
    pub xp_into_level: i32,
    pub xp_for_level: i32,
    pub xp_to_next_level: i32,
    pub challenges_completed: i32,
    pub balloons_up: i32,
    /// Acabou de sair e ainda dá para desfazer.
    pub just_logged_out: bool,
    /// Primeiro nome derivado do e-mail, para a despedida.
    pub display_name: String,
    pub match_view: match_engine::MatchViewModel,
    pub contest_name: String,
    pub standings_global: Vec<crate::domain::StandingRow>,
    pub standings_home: Vec<crate::domain::StandingRow>,
    pub user_standing: crate::domain::StandingRow,
    pub scoreboard: Vec<crate::domain::ScoreboardRow>,
}

#[effect(facet_typegen)]
#[facet(fg::namespace = "LogN")]
pub enum Effect {
    Render(RenderOperation),
    Http(HttpRequest),
    SecureStore(KeyValueOperation),
    Telemetry(crate::domain::TelemetryOperation),
    Monitoring(crate::domain::MonitoringOperation),
}

/// XP por nível. O DS fixa a fórmula: `nível = floor(xp / 200) + 1`.
pub const XP_PER_LEVEL: i32 = 200;

/// Nível a partir do XP acumulado. Mora aqui, não no cliente.
pub fn level_for_xp(xp: i32) -> i32 {
    xp.max(0) / XP_PER_LEVEL + 1
}

/// Tira o "A · " da frente do nome do problema.
///
/// A letra é posição na partida, não parte do nome: o mesmo desafio pode ser o A de um
/// nó e o C de outro. O seed guardava "A · Soma de Dois Números" e o core prefixava de novo,
/// então o enunciado abria como "A · A · SOMA DE DOIS NÚMEROS".
pub fn strip_problem_letter(title: &str) -> String {
    let mut chars = title.chars();
    match (chars.next(), chars.next()) {
        (Some(first), Some(' ')) if first.is_ascii_uppercase() => {
            let rest = chars.as_str();
            rest.strip_prefix("· ").unwrap_or(title).to_string()
        }
        _ => title.to_string(),
    }
}

/// Primeiro nome a partir do e-mail, para a tela de saída ("Até a próxima, Rodrigo").
/// Sem e-mail, devolve vazio — e a tela cai numa despedida sem nome.
pub fn display_name_from_email(email: &str) -> String {
    let local = email.split('@').next().unwrap_or("");
    let first = local
        .split(|c: char| c == '.' || c == '_' || c == '-' || c == '+')
        .find(|p| !p.is_empty())
        .unwrap_or("");

    let mut chars = first.chars();
    match chars.next() {
        Some(c) => c.to_uppercase().collect::<String>() + &chars.as_str().to_lowercase(),
        None => String::new(),
    }
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
                model.account_email = email.clone();

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
                        struct AuthResp { access_token: String, refresh_token: String, #[serde(default)] user_id: String }
                        
                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
                            model.is_guest = false;
                            // A tela de login mostra o status como erro, em vermelho: um
                            // "deu certo" ali é ruído. A prova do sucesso é o app abrir.
                            model.status = String::new();

                            // Salva refresh token no Keychain
                            return Command::request_from_shell(KeyValueOperation::Set {
                                key: "refresh_token".to_string(),
                                value: data.refresh_token.into_bytes()
                            }).then_send(Event::TokenStored);
                        } else {
                            model.status = "Não consegui ler a resposta do servidor.".to_string();
                            model.is_authenticating = false;
                        }
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.status = "E-mail ou senha não conferem.".to_string();
                        model.is_authenticating = false;
                    }
                    HttpResult::Ok(_) => {
                        model.status = "Não consegui entrar agora. Tente de novo.".to_string();
                        model.is_authenticating = false;
                    }
                    HttpResult::Err(_) => {
                        model.status = "Sem conexão com o servidor.".to_string();
                        model.is_authenticating = false;
                    }
                }
                render::render()
            }

            Event::ContinueAsGuest => {
                model.is_guest = true;
                model.nodes = crate::mock_data::get_mock_nodes();
                model.challenges = crate::mock_data::get_mock_challenges();
                model.status = "Modo Visitante: Dados locais carregados".to_string();
                render::render()
            }

            Event::TokenStored(_) => {
                model.is_authenticating = false;
                // O e-mail vai para o mesmo cofre do refresh token: é ele que devolve
                // a identidade quando o app abre sem passar pela tela de login.
                Command::request_from_shell(KeyValueOperation::Set {
                    key: "account_email".to_string(),
                    value: model.account_email.clone().into_bytes(),
                })
                .then_send(Event::AccountEmailStored)
            }

            Event::AccountEmailStored(_) => {
                Command::request_from_shell(crate::domain::TelemetryOperation::Identify { user_id: model.user_id.clone() })
                    .then_send(|_| Event::TelemetrySent)
            }

            Event::Logout => {
                model.status = "Logging out...".to_string();
                Command::request_from_shell(KeyValueOperation::Delete { key: "refresh_token".into() }).then_send(Event::TokenCleared)
            }
            Event::TokenCleared(_) => {
                // Guarda o que dá para devolver. Sair estando sincronizado é
                // reversível, e o DS troca o alerta de confirmação por um desfazer:
                // alerta em toda saída treina o usuário a confirmar sem ler.
                model.logout_undo = Some(LogoutSnapshot {
                    access_token: model.access_token.clone(),
                    was_guest: model.is_guest,
                    email: model.account_email.clone(),
                });

                model.is_guest = false;
                model.access_token = None;
                model.account_email = String::new();
                model.user_id = String::new();
                model.nodes = vec![];
                model.challenges = vec![];
                model.pending_events = vec![];
                // Quem conta que a saída deu certo é a tela de despedida. Deixar texto
                // aqui fazia a tela de login abrir com "Logged out successfully" em
                // vermelho, como se sair fosse um erro.
                model.status = String::new();
                render::render()
            }

            Event::UndoLogout => {
                if let Some(snapshot) = model.logout_undo.take() {
                    model.access_token = snapshot.access_token;
                    model.is_guest = snapshot.was_guest;
                    model.account_email = snapshot.email;
                    model.status = "Sessão restaurada".to_string();
                }
                render::render()
            }

            Event::DismissLogoutNotice => {
                model.logout_undo = None;
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
                // Instead of showing an error on the login screen, we just remain silent
                model.status = "".to_string();
                model.access_token = None;
                model.is_authenticating = false;
                render::render()
            }

            Event::RefreshCompleted(result) => {
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct RefreshResp { access_token: String, #[serde(default)] user_id: String }
                        
                        if let Ok(data) = serde_json::from_slice::<RefreshResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
                            model.is_guest = false;
                            model.status = "Session refreshed!".to_string();

                            // O `/refresh` devolve só o token; quem a sessão é fica no cofre.
                            return Command::request_from_shell(KeyValueOperation::Get {
                                key: "account_email".to_string(),
                            })
                            .then_send(Event::AccountEmailRead);
                        } else {
                            model.status = "Failed to parse refresh response".to_string();
                        }
                    }
                    _ => {
                        model.status = "Sua sessão expirou. Entre de novo.".to_string();
                        model.access_token = None;
                model.is_authenticating = false;
                    }
                }
                render::render()
            }

            Event::AccountEmailRead(result) => {
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value } } = result {
                    if let crux_kv::Value::Bytes(bytes) = value {
                        model.account_email = String::from_utf8(bytes).unwrap_or_default();
                    }
                }

                // O 401 que disparou o refresh deixou um evento em espera.
                if let Some(pending) = model.pending_retry_event.take() {
                    return self.update(pending, model);
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
                            return self.update(Event::FetchChallenges, model);
                        } else if response.status == 401 {
                            model.pending_retry_event = Some(Event::FetchNodes);
                            Command::request_from_shell(KeyValueOperation::Get { key: "refresh_token".into() }).then_send(Event::TokenRead)
                        } else {
                            model.status = format!("Error: {}", response.status);
                            render::render()
                        }
                    }
                    HttpResult::Err(_) => {
                        model.status = "Offline Mode: Using Local Mock Data".to_string();
                        model.nodes = crate::mock_data::get_mock_nodes();
                        model.challenges = crate::mock_data::get_mock_challenges();
                        render::render()
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
                        model.status = "Offline Mode: Loaded Mock Challenges".to_string();
                        model.challenges = crate::mock_data::get_mock_challenges();
                    }
                }
                render::render()
            }
            Event::SubmitChallengeAnswer { action_id, challenge_id, answer_json, timestamp } => {
                let challenge = model.challenges.iter().find(|c| c.id == challenge_id);
                if let Some(ch) = challenge {
                    let is_correct = if ch.template_type == "SPOT_THE_BUG" {
                        if let Some(correct_line) = ch.payload.validation.correct_line {
                            // `selected_line` vem do cliente em índice; o desafio guarda
                            // a linha como ela aparece numerada na tela.
                            let index = correct_line - 1;
                            answer_json.contains(&format!("\"selected_line\": {}", index)) ||
                            answer_json.contains(&format!("\"selected_line\":{}", index))
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
                model.account_email = email.clone();

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
                        struct AuthResp { access_token: String, refresh_token: String, #[serde(default)] user_id: String }

                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
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
                model.account_email = email.clone();

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
                        struct AuthResp { access_token: String, refresh_token: String, #[serde(default)] user_id: String }

                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
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
            // Sincroniza e só então sai.
            //
            // O sheet crítico mandava `SyncNow` e `Logout` em sequência: a saída
            // esvaziava a fila antes de a resposta chegar, então um sync recusado
            // levava o progresso junto — e a tela de despedida ainda dizia que o XP
            // estava no servidor.
            Event::SyncAndLogout => {
                if model.pending_events.is_empty() {
                    return self.update(Event::Logout, model);
                }
                model.logout_after_sync = true;
                self.update(Event::SyncNow, model)
            }

            Event::SyncNow => {
                if model.is_guest && model.access_token.is_none() {
                    model.status = "Sign in to sync your progress!".to_string();
                    model.logout_after_sync = false;
                    return Command::done();
                }
                if model.is_syncing || model.pending_events.is_empty() {
                    return Command::done();
                }

                // Sem saber de quem é a sessão não há o que sincronizar. O código
                // mandava o literal "user_1", que o Postgres recusa como UUID: a fila
                // batia num 500 e o app dizia que o progresso estava salvo.
                if model.user_id.is_empty() {
                    model.status = "Entre de novo para sincronizar.".to_string();
                    model.logout_after_sync = false;
                    return render::render();
                }

                model.is_syncing = true;
                model.status = "Syncing...".to_string();

                let payload = SyncPayload {
                    user_id: model.user_id.clone(),
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
                            model.status = String::new();
                            model.pending_events.clear();

                            // Flush the empty queue to disk so it doesn't duplicate
                            let bytes = serde_json::to_vec(&model.pending_events).unwrap_or_default();
                            let flush = Command::request_from_shell(KeyValueOperation::Set {
                                key: "offline_events".to_string(),
                                value: bytes,
                            }).then_send(|_| Event::Ping); // Ping just as a dummy no-op event

                            if model.logout_after_sync {
                                model.logout_after_sync = false;
                                // A fila subiu: agora sair é seguro.
                                return flush.and(self.update(Event::Logout, model));
                            }
                            return flush;
                        } else if response.status == 409 {
                            model.status = "Seu progresso divergiu do servidor.".to_string();
                        } else if response.status == 401 {
                            // Intercept 401 and attempt refresh
                            model.pending_retry_event = Some(Event::SyncNow);
                            return self.update(Event::AttemptRefresh, model);
                        } else {
                            model.status = "Não consegui enviar seu progresso agora.".to_string();
                        }
                    }
                    HttpResult::Err(_err) => {
                        model.status = "Sem conexão para enviar seu progresso.".to_string();
                    }
                }
                // Falhou: fica. Sair aqui apagaria a fila que não subiu.
                model.logout_after_sync = false;
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

            // ── Match Events ──────────────────────────────────
            Event::StartMatch { node_id } => {
                let problems: Vec<match_engine::MatchProblem> = model.challenges.iter()
                    .filter(|c| c.node_id == node_id)
                    .enumerate()
                    .map(|(i, c)| {
                        let letter = (b'A' + i as u8) as char;
                        match_engine::MatchProblem {
                            letter: letter.to_string(),
                            challenge_id: c.id.clone(),
                            template_type: c.template_type.clone(),
                            title: format!("{} · {}", letter, strip_problem_letter(&c.payload.content.title)),
                            description: c.payload.content.description.clone(),
                            code_lines: c.payload.content.code_lines.clone(),
                            // O desafio guarda a linha como o jogador a lê (a partir de 1);
                            // o motor compara com o índice que o toque devolve.
                            correct_line: c.payload.validation.correct_line.map(|l| l - 1),
                            expected_string: c.payload.validation.expected_string.clone(),
                            options: c.payload.content.options.clone().unwrap_or_default(),
                            correct_options: c.payload.content.correct_options.clone().unwrap_or_default(),
                            max_selections: if c.template_type == "TAG_THE_PATTERN" { c.payload.content.correct_options.as_ref().map_or(1, |o| o.len() as i32) } else { 1 },
                            watch_variables: c.payload.content.watch_variables.clone().unwrap_or_default(),
                            watch_note: c.payload.content.watch_note.clone().unwrap_or_default(),
                        }
                    })
                    .collect();

                if problems.is_empty() {
                    model.match_state = None; // clear previous state
                    model.status = "No challenges for this node".to_string();
                    return render::render();
                }

                model.match_state = Some(match_engine::MatchState::new(problems));
                model.status = "Match started!".to_string();
                render::render()
            }

            Event::MatchSelectLine { line } => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.selected_line = Some(line);
                }
                render::render()
            }

            Event::MatchSetAnswer { answer } => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.answer_string = Some(answer);
                }
                render::render()
            }

            Event::MatchSetDropTime { value } => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.drop_time = Some(value);
                }
                render::render()
            }

            Event::MatchSetDropSpace { value } => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.drop_space = Some(value);
                }
                render::render()
            }

            Event::MatchToggleTag { tag } => {
                if let Some(ref mut ms) = model.match_state {
                    if let Some(pos) = ms.selection.selected_tags.iter().position(|t| *t == tag) {
                        ms.selection.selected_tags.remove(pos);
                    } else {
                        ms.selection.selected_tags.push(tag);
                    }
                }
                render::render()
            }

            Event::MatchSetOutput { value } => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.predicted_output = Some(value);
                }
                render::render()
            }

            Event::MatchSubmit { timestamp } => {
                if let Some(ref mut ms) = model.match_state {
                    // Template e letra são lidos ANTES do submit: ele avança de problema,
                    // e o evento registrado é o do problema que acabou de ser respondido.
                    let template = ms.current_template_type().to_string();
                    let letter = ms.current_letter().to_string();

                    let verdict = ms.submit();
                    let is_correct = verdict == match_engine::VerdictCode::Accepted;

                    if is_correct {
                        model.global_xp += 50;
                        match template.as_str() {
                            "SPOT_THE_BUG" => model.bugs_found += 1,
                            "DRY_RUN" => model.dry_runs_completed += 1,
                            _ => {}
                        }
                    }

                    let mut match_ended = false;
                    let mut solved = 0;
                    if !ms.is_active {
                        solved = ms.solved_count();
                        model.status = format!("Match over! {} solved", solved);
                        match_ended = true;
                    }

                    // Register game event for offline sync
                    let previous_hash = if model.last_hash.is_empty() {
                        "0000000000000000000000000000000000000000000000000000000000000000".to_string()
                    } else {
                        model.last_hash.clone()
                    };

                    let payload = format!(r#"{{"letter":"{}","is_correct":{},"template_type":"{}"}}"#, letter, is_correct, template);
                    let action_id = format!("match_{}", timestamp);
                    let game_event = GameEvent::new(action_id.clone(), "MATCH_ANSWER".into(), payload, timestamp, previous_hash.clone());
                    model.last_hash = game_event.current_hash.clone();
                    model.pending_events.push(game_event);

                    if match_ended {
                        let end_payload = format!(r#"{{"solved":{}}}"#, solved);
                        let end_event = GameEvent::new(format!("{}_end", action_id), "MATCH_END".into(), end_payload, timestamp, model.last_hash.clone());
                        model.last_hash = end_event.current_hash.clone();
                        model.pending_events.push(end_event);
                    }
                }
                render::render()
            }

            Event::MatchDismissTrap => {
                if let Some(ref mut ms) = model.match_state {
                    ms.trap = None;
                }
                render::render()
            }

            Event::MatchTimerTick => {
                if let Some(ref mut ms) = model.match_state {
                    if ms.is_active {
                        if ms.question_seconds_remaining > 0 {
                            ms.question_seconds_remaining -= 1;
                        }
                        if ms.contest_seconds_remaining > 0 {
                            ms.contest_seconds_remaining -= 1;
                        }
                        
                        // If question timer hits 0, it's a TLE (Time Limit Exceeded)
                        if ms.question_seconds_remaining == 0 {
                            ms.submit_tle();
                        }
                        
                        if ms.contest_seconds_remaining <= 0 {
                            ms.is_active = false;
                            model.status = "Time's up!".to_string();
                        }

                        // A última hora do contest: o placar congela.
                        ms.refresh_freeze();
                    }
                }
                render::render()
            }
        }
    }

        fn view(&self, model: &Self::Model) -> Self::ViewModel {
        let mut computed_nodes = model.nodes.clone();
        
        // Calculate DAG status
        for i in 0..computed_nodes.len() {
            let req_xp = computed_nodes[i].required_xp;
            let id = computed_nodes[i].id.clone();
            
            if model.global_xp < req_xp {
                computed_nodes[i].status = crate::domain::NodeStatus::Locked;
            } else {
                // It is at least Active. Is it Completed?
                // A node is completed if ANY of its children (nodes that have it as prerequisite)
                // are UNLOCKED (meaning user's global_xp >= child.required_xp).
                let mut has_unlocked_child = false;
                for child in &model.nodes {
                    if child.prerequisites.contains(&id) && model.global_xp >= child.required_xp {
                        has_unlocked_child = true;
                        break;
                    }
                }
                
                if has_unlocked_child {
                    computed_nodes[i].status = crate::domain::NodeStatus::Completed;
                } else {
                    computed_nodes[i].status = crate::domain::NodeStatus::Active;
                }
            }
        }

        // "N balões no ar" conta nós conquistados, não problemas aceitos.
        let balloons_up = computed_nodes
            .iter()
            .filter(|n| n.status == crate::domain::NodeStatus::Completed)
            .count() as i32;

        ViewModel {
            display_status: model.status.clone(),
            pending_sync_count: model.pending_events.len() as u32,
            is_syncing: model.is_syncing,
            is_fetching: model.is_fetching,
            is_authenticating: model.is_authenticating,
            has_access_token: model.access_token.is_some(),
            is_guest: model.is_guest,
            challenges: model.challenges.clone(),
            nodes: computed_nodes,
            otp_email: model.otp_email.clone(),
            account_email: model.account_email.clone(),
            otp_verified: model.otp_verified,
            global_xp: model.global_xp,
            bugs_found: model.bugs_found,
            dry_runs_completed: model.dry_runs_completed,
            level: level_for_xp(model.global_xp),
            xp_into_level: model.global_xp.rem_euclid(XP_PER_LEVEL),
            xp_for_level: XP_PER_LEVEL,
            xp_to_next_level: XP_PER_LEVEL - model.global_xp.rem_euclid(XP_PER_LEVEL),
            challenges_completed: model.bugs_found + model.dry_runs_completed,
            balloons_up: balloons_up,
            just_logged_out: model.logout_undo.is_some(),
            // Depois de sair, o nome vem do instantâneo — é ele que a despedida usa.
            display_name: display_name_from_email(
                model.logout_undo.as_ref().map(|s| s.email.as_str()).unwrap_or(&model.account_email),
            ),
            match_view: model.match_state.as_ref()
                .map(|ms| ms.to_view_model())
                .unwrap_or_default(),
            // Placar e ranking ainda não têm API; os dados vivem no core para o
            // cliente seguir sendo uma camada burra, como manda a arquitetura.
            contest_name: "REGIONAL SUL-AMERICANA".to_string(),
            standings_global: crate::mock_data::get_mock_standings_global(),
            standings_home: crate::mock_data::get_mock_standings_home(),
            user_standing: crate::mock_data::get_mock_user_standing(),
            scoreboard: crate::mock_data::get_mock_scoreboard(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;


    #[test]
    fn test_strips_only_a_single_letter_prefix_from_the_title() {
        assert_eq!(strip_problem_letter("A · Soma de Dois Números"), "Soma de Dois Números");
        assert_eq!(strip_problem_letter("Soma de Dois Números"), "Soma de Dois Números");
        // "DP" não é letra de problema; o nome fica inteiro.
        assert_eq!(strip_problem_letter("DP · Mochila"), "DP · Mochila");
        assert_eq!(strip_problem_letter("a · minúscula"), "a · minúscula");
        assert_eq!(strip_problem_letter(""), "");
    }

    /// Tocar na linha do bug tem de dar Accepted.
    ///
    /// O desafio guarda a linha como o jogador a lê (4), o toque devolve o índice (3),
    /// e o motor comparava um com o outro: acertar a linha certa dava Wrong Answer, e
    /// o relatório ainda dizia "sua resposta: linha 4" — a resposta certa, recusada.
    #[test]
    fn test_spot_the_bug_accepts_the_line_the_player_sees() {
        use crate::domain::{Challenge, ChallengeContent, ChallengePayload, ChallengeValidation};

        let app = LogNApp::default();
        let mut model = Model::default();
        model.challenges = vec![Challenge {
            id: "ch_001".into(),
            node_id: "node_1".into(),
            template_type: "SPOT_THE_BUG".into(),
            version: 1,
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: "Ache o laço infinito.".into(),
                    code_lines: vec![
                        "int l = 0, r = n - 1;".into(),
                        "while (a < b) {".into(),
                        "    int mid = l + (r - l) / 2;".into(),
                        "    if (a >= b) {".into(),
                    ],
                    options: None,
                    correct_options: None,
                    watch_variables: None,
                    watch_note: None,
                },
                validation: ChallengeValidation {
                    validation_type: "LINE_MATCH".into(),
                    correct_line: Some(4),
                    expected_string: None,
                },
            },
        }];

        let _ = app.update(Event::StartMatch { node_id: "node_1".into() }, &mut model);
        // A quarta linha é o índice 3, que é o que o toque manda.
        let _ = app.update(Event::MatchSelectLine { line: 3 }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);

        assert_eq!(app.view(&model).match_view.last_verdict, "AC");
    }

    /// Sair com fila pendente não pode perder a fila.
    ///
    /// O sheet crítico mandava `SyncNow` e `Logout` em seguida, e o `Logout` limpava
    /// `pending_events` antes da resposta chegar: sync recusado, progresso apagado, e
    /// a despedida dizendo que estava tudo no servidor.
    #[test]
    fn test_sync_and_logout_keeps_the_queue_when_the_sync_fails() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = "66b670a2-41d2-4ba2-b863-78735b69ec7c".into();
        model.pending_events = vec![GameEvent::new(
            "evt_1".into(), "MATCH_ANSWER".into(), "{}".into(), 1_700_000_000,
            "0000000000000000000000000000000000000000000000000000000000000000".into(),
        )];

        let mut cmd = app.update(Event::SyncAndLogout, &mut model);
        let req = cmd.expect_one_effect();
        assert!(matches!(req, Effect::Http(ref r) if r.operation.url == "/api/v1/sync"));

        let failure = HttpResult::Ok(crux_http::protocol::HttpResponse {
            status: 500, headers: vec![], body: vec![],
        });
        let _ = app.update(Event::SyncCompleted(failure), &mut model);

        assert_eq!(model.pending_events.len(), 1, "a fila que não subiu fica");
        assert!(model.access_token.is_some(), "e a sessão continua de pé");
        assert!(!model.logout_after_sync);
    }

    #[test]
    fn test_display_name_takes_the_first_name_from_the_email() {
        assert_eq!(display_name_from_email("jogador@example.com"), "Rodrigo");
        assert_eq!(display_name_from_email("jogador@example.com"), "Rodrigo");
        assert_eq!(display_name_from_email("JOSE_CLEITON@x.com"), "Jose");
        assert_eq!(display_name_from_email(""), "", "sem e-mail, sem nome inventado");
        assert_eq!(display_name_from_email("@x.com"), "");
    }

    #[test]
    fn test_level_starts_at_one_and_climbs_every_200_xp() {
        assert_eq!(level_for_xp(0), 1, "quem nunca jogou já está no nível 1");
        assert_eq!(level_for_xp(199), 1);
        assert_eq!(level_for_xp(200), 2);
        assert_eq!(level_for_xp(620), 4);
        // XP negativo não existe, mas não pode virar nível zero nem negativo.
        assert_eq!(level_for_xp(-50), 1);
    }

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
            "refresh_token": "ref_tok",
            "user_id": "66b670a2-41d2-4ba2-b863-78735b69ec7c"
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

        // Entrar por senha também identifica a sessão: o e-mail vai para o cofre
        // junto do refresh token, e é ele que o perfil mostra na volta.
        assert_eq!(model.account_email, "test@x.com");
        // E o id vem do servidor — sem ele o sync subia com "user_1" e o banco recusava.
        assert_eq!(model.user_id, "66b670a2-41d2-4ba2-b863-78735b69ec7c");

        let kv_result = KeyValueResult::Ok { response: KeyValueResponse::Set { previous: crux_kv::Value::None } };
        let mut cmd = app.update(Event::TokenStored(kv_result), &mut model);

        let kv_req = cmd.expect_one_effect();
        if let Effect::SecureStore(r) = kv_req {
            if let KeyValueOperation::Set { key, value } = r.operation {
                assert_eq!(key, "account_email");
                assert_eq!(String::from_utf8(value).unwrap(), "test@x.com");
            } else {
                panic!("Expected Set operation");
            }
        } else {
            panic!("Expected SecureStore effect storing the account e-mail");
        }

        assert_eq!(app.view(&model).display_name, "Test");
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

        // O refresh só devolve o token: antes de repetir o pedido o core repõe de quem
        // é a sessão, senão o perfil abre sem nome depois de um 401.
        let kv_req = cmd.expect_one_effect();
        if let Effect::SecureStore(r) = kv_req {
            assert!(matches!(r.operation, KeyValueOperation::Get { ref key } if key == "account_email"));
        } else {
            panic!("Expected SecureStore effect reading the account e-mail");
        }

        let kv_result = KeyValueResult::Ok {
            response: KeyValueResponse::Get { value: crux_kv::Value::Bytes("jogador@example.com".into()) },
        };
        let mut cmd = app.update(Event::AccountEmailRead(kv_result), &mut model);

        assert_eq!(model.account_email, "jogador@example.com");
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
