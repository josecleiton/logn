use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crux_http::protocol::{HttpRequest, HttpResult};
use crux_kv::{KeyValueOperation, KeyValueResult, KeyValueResponse};
use crux_time::{Time, TimeRequest};
use crate::domain::{GameEvent, SyncPayload, Challenge, StatusKey, TelemetryOperation};
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
    FetchProgress,
    ProgressFetched(HttpResult),
    NodesFetched(HttpResult),
    ChallengesFetched(HttpResult),

    SyncNow,
    SyncAndLogout,
    RestoreOfflineQueue,
    OfflineQueueRestored(KeyValueResult),
    /// A trilha que viaja no bundle do app, entregue pelo shell na abertura.
    ///
    /// O Core não lê arquivo; quem lê é o shell, e aqui só se decide se a semente serve.
    BundledTrailLoaded { json: String },
    /// Tocou no X. Pede confirmação quando há partida a perder; sai direto quando não há.
    LeaveMatch,
    /// Confirmou a saída no cartão.
    ConfirmLeaveMatch,
    /// Desistiu de sair e voltou para a partida.
    CancelLeaveMatch,
    /// Tocou no selo de origem. Na primeira vez de cada origem, para o relógio.
    OpenOriginSheet,
    CloseOriginSheet,
    OriginsSeenRestored(KeyValueResult),
    OriginsSeenStored(KeyValueResult),
    DismissPasswordReset,
    /// O shell dá a hora ao Core: na abertura e ao voltar para o primeiro plano.
    Tick { now: i64 },
    /// Hora pedida pelo `crux_time` enquanto há bloqueio por 429, em segundos desde a
    /// época. Ancora o bloqueio que acabou de começar e recalcula a contagem.
    CooldownClock { now: i64 },
    /// Passou um segundo de bloqueio: hora de pedir a hora de novo.
    CooldownElapsed,
    SessionExpiryStored(KeyValueResult),
    OfflineSessionChecked(KeyValueResult),
    SnapshotSaved(KeyValueResult),
    SnapshotRestored(KeyValueResult),
    RotatedTokenStored(KeyValueResult),
    AttemptRefreshDone,
    SyncCompleted(HttpResult),
    RequestOTP { email: String, purpose: String },
    OTPRequested(HttpResult),
    VerifyOTP { email: String, code: String, purpose: String },
    OTPVerified(HttpResult),
    Register { email: String, password: String, otp: String },
    RegisterCompleted(HttpResult),
    ResetPassword { email: String, new_password: String, otp: String },
    ResetPasswordCompleted(HttpResult),
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
    /// O refresh token voltou ao cofre depois de um `UndoLogout`.
    LogoutUndone(KeyValueResult),
    /// Hora pedida ao `crux_time` para fechar a partida abandonada. `model.now` pode
    /// ter horas — é a da abertura —, e o fim da partida vai para o histórico.
    MatchAbandonedAt { solved: i32, now: i64 },
    /// O jogador fechou o relatório pós-partida e volta para a trilha.
    MatchReportClosed,
    ReviewMilestonesFired(KeyValueResult),
    ReviewMilestonesRestored(KeyValueResult),
}

/// O que o desfazer devolve. Existe só entre a saída e o jogador deixar a tela.
///
/// É tudo o que `TokenCleared` apaga, disco incluído: devolver só a memória deixava
/// a sessão viva até o app fechar e mais nada.
#[derive(Clone, Default)]
pub struct LogoutSnapshot {
    pub access_token: Option<String>,
    /// O que o cofre devolveu ao apagar. `None` para visitante, que não tem.
    pub refresh_token: Option<Vec<u8>>,
    pub was_guest: bool,
    pub email: String,
    pub user_id: String,
    pub session_expires_at: i64,
    pub nodes: Vec<crate::domain::SkillNode>,
    pub challenges: Vec<Challenge>,
    pub pending_events: Vec<GameEvent>,
}

#[derive(Default, Clone)]
pub struct Model {
    /// Diagnóstico interno, em inglês, para log e teste. **Não vai para a tela.**
    pub status: String,
    /// O que a tela mostra, como chave. A cópia vive em `i18n/locales/`.
    pub status_key: crate::domain::StatusKey,
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
    /// Desafios que já renderam XP. Rejogar um deles não paga de novo.
    ///
    /// Sem isto, repetir o nó 1 abria o nó 7: os portões da trilha eram de moagem, não
    /// de conhecimento. Quem manda é o servidor — o login devolve a lista —, e o Core
    /// guarda a cópia para decidir sozinho enquanto está offline.
    pub paid_challenges: Vec<String>,
    pub match_state: Option<match_engine::MatchState>,
    /// Nó de onde a partida saiu. Vai no evento de sync para o servidor saber a que
    /// trilha creditar o XP — sem ele `user_progress` nunca ganhava uma linha.
    pub match_node_id: String,
    pub logout_undo: Option<LogoutSnapshot>,
    /// Marcado por `SyncAndLogout`: a saída espera a fila subir.
    pub logout_after_sync: bool,
    /// Já reencadeou a fila nesta tentativa. Impede laço de rebase com o servidor.
    pub rebase_attempted: bool,
    /// Acabou de trocar a senha. Fecha a tela de redefinição e volta a false.
    ///
    /// Quem fechava era a chegada do token — e quem redefine já estando logado não vê
    /// token nenhum chegar, então a tela ficava aberta depois de salvar.
    pub password_reset_done: bool,
    /// Agora, em segundos desde a época. Vem do shell — o Core não tem relógio.
    pub now: i64,
    /// Até quando a sessão vale, em segundos desde a época. 0 = desconhecido.
    pub session_expires_at: i64,
    /// Sessão em pé sem ter falado com o servidor nesta abertura.
    ///
    /// Abrir o app sem rede jogava para a tela de login mesmo com sessão guardada e
    /// dentro do prazo — num app que promete offline-first, era pedir para o jogador
    /// digitar a senha para ver o que já estava no aparelho.
    pub session_offline: bool,
    /// Origens cujo cartão de homenagem o jogador já leu, por chave (`FARIAS`).
    ///
    /// A primeira leitura de cada origem pausa o relógio da questão; as seguintes não.
    /// Sobrevive ao fechamento do app pelo `crux_kv`, senão a cortesia viraria uma
    /// forma de parar o cronômetro quantas vezes se quisesse.
    pub origins_seen: Vec<String>,
    /// Origem cujo cartão está aberto agora. Vazia quando não há cartão na tela.
    pub origin_sheet: String,
    /// O jogador saiu da partida por vontade própria, e não porque ela acabou.
    ///
    /// A diferença importa para a navegação: quem perdeu as três vidas tem relatório
    /// para ler, quem desistiu não tem o que revisar e volta direto para a trilha.
    /// Volta a falso quando uma partida começa.
    pub match_left: bool,
    /// Pedido de avaliação na loja marcado para o fechamento do relatório.
    ///
    /// Setado quando o jogador domina um nó de milestone, consumido em `MatchReportClosed`.
    /// Em memória: matar o app na tela do relatório perde esse pedido, o que é aceitável.
    pub review_prompt_pending: bool,
    /// Milestones de domínio que já geraram um pedido de avaliação, persistidos no disco.
    ///
    /// Impede que adicionar um desafio a um nó já dominado dispare o mesmo milestone de novo.
    /// Persistido via `crux_kv` para sobreviver ao fechamento do app.
    pub review_milestones_fired: Vec<usize>,
    /// A trilha em uso veio da semente do bundle, e ninguém falou com o servidor ainda.
    ///
    /// A semente envelhece com o binário, não com o conteúdo: quem instalar hoje e
    /// passar um mês offline joga a trilha de um mês atrás, e sem isto não há nada na
    /// tela dizendo isso. Vira falso assim que o retrato ou o servidor preenche.
    pub trail_from_bundle: bool,
    /// Quando a semente foi gerada, em ISO 8601, como veio do asset. O cliente formata
    /// na língua dele — o Core não sabe em que idioma o app está.
    pub trail_generated_at: String,
    /// Bloqueio do envio de código, depois de um 429 no `request-otp`.
    ///
    /// Só trava enviar e reenviar: quem pediu código de novo cedo demais ainda pode
    /// digitar o que já recebeu. Travar o "Verificar" junto era punir o jogador pelo
    /// toque a mais.
    pub resend_cooldown: Cooldown,
    /// Bloqueio de todas as ações de conta, depois de um 429 em qualquer outra rota de
    /// autenticação. Esse vem do limite por IP do servidor, que vale para todas elas.
    pub auth_cooldown: Cooldown,
    /// Há um `notify_after` de bloqueio pendente no shell. Impede que dois 429 seguidos
    /// ponham dois relógios para bater ao mesmo tempo.
    pub cooldown_timer_running: bool,
}

/// Um bloqueio por 429: quantos segundos o servidor pediu e até quando isso vai.
///
/// `until` fica zerado entre o 429 chegar e a primeira hora do `crux_time` voltar. O
/// Core não tem relógio, e `now` só é confiável depois de pedido: a hora da abertura
/// pode ter horas.
#[derive(Default, Clone, Copy, Debug, PartialEq, Eq)]
pub struct Cooldown {
    pub secs: i64,
    pub until: i64,
}

impl Cooldown {
    fn start(secs: i64) -> Self {
        Cooldown { secs, until: 0 }
    }

    fn is_active(&self) -> bool {
        self.secs > 0
    }

    /// Fixa o fim do bloqueio na primeira hora que chegar, e o encerra quando passa.
    fn advance(&mut self, now: i64) {
        if !self.is_active() {
            return;
        }
        if self.until == 0 {
            self.until = now + self.secs;
        }
        if now >= self.until {
            *self = Cooldown::default();
        }
    }

    /// Segundos que faltam, para a tela. Antes da âncora é o que o servidor pediu.
    fn remaining(&self, now: i64) -> u32 {
        if !self.is_active() {
            0
        } else if self.until == 0 {
            self.secs as u32
        } else {
            (self.until - now).max(0) as u32
        }
    }
}

/// Quanto o servidor mandou esperar. Sem `Retry-After` legível, um minuto, que é o que
/// o backend usa; acima de uma hora, uma hora, para um cabeçalho torto não travar o
/// botão pelo resto do dia.
fn retry_after_secs(response: &crux_http::protocol::HttpResponse) -> i64 {
    response
        .headers
        .iter()
        .find(|h| h.name.eq_ignore_ascii_case("retry-after"))
        .and_then(|h| h.value.trim().parse::<i64>().ok())
        .filter(|s| *s > 0)
        .map(|s| s.min(3600))
        .unwrap_or(60)
}

/// Pede a hora ao shell para ancorar e contar um bloqueio.
fn ask_cooldown_clock() -> Command<Effect, Event> {
    Time::now().then_send(|t| Event::CooldownClock { now: unix_seconds(t) })
}

fn unix_seconds(t: std::time::SystemTime) -> i64 {
    t.duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0)
}

/// Um 429 numa ação de conta: trava os botões e começa a contagem.
fn rate_limited(model: &mut Model, response: &crux_http::protocol::HttpResponse, resend_only: bool) -> Command<Effect, Event> {
    let cooldown = Cooldown::start(retry_after_secs(response));
    if resend_only {
        model.resend_cooldown = cooldown;
    } else {
        model.auth_cooldown = cooldown;
    }
    model.is_authenticating = false;
    model.status = "Rate limited".to_string();
    model.status_key = StatusKey::RateLimited;
    ask_cooldown_clock().and(render::render())
}

/// A ação foi disparada com o bloqueio ainda correndo — toque que escapou antes da tela
/// redesenhar. Não sai pedido nenhum: o servidor ia só devolver outro 429.
fn still_rate_limited(model: &mut Model) -> Command<Effect, Event> {
    model.status_key = StatusKey::RateLimited;
    render::render()
}

#[derive(Facet, Serialize, Deserialize, Default, Clone)]
#[facet(fg::namespace = "LogN")]
pub struct ViewModel {
    /// O que dizer ao jogador, como chave — o cliente traduz. Nunca frase pronta:
    /// o Core não sabe em que idioma o app está.
    pub status: crate::domain::StatusKey,
    pub pending_sync_count: u32,
    pub is_syncing: bool,
    pub is_fetching: bool,
    pub is_authenticating: bool,
    /// Tem credencial para falar com o servidor agora.
    pub has_access_token: bool,
    /// Tem sessão — com ou sem rede. É isto que decide se o app abre no jogo ou no login.
    pub has_session: bool,
    /// A sessão está em pé sem ter falado com o servidor nesta abertura.
    pub is_offline_session: bool,
    /// A trilha na tela é a que viajou no bundle, congelada quando o build saiu.
    /// O cliente avisa: pode haver desafio que este app ainda não conhece.
    pub trail_from_bundle: bool,
    /// Quando essa semente foi gerada, em ISO 8601. O cliente formata na língua dele.
    pub trail_generated_at: String,
    /// A última partida acabou porque o jogador saiu, não porque ela terminou. A tela
    /// volta direto para a trilha: quem desistiu não tem relatório para ler.
    pub match_left: bool,
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
    /// Acabou de trocar a senha: a tela de redefinição pode fechar.
    pub password_reset_done: bool,
    /// Primeiro nome derivado do e-mail, para a despedida.
    pub display_name: String,
    pub match_view: match_engine::MatchViewModel,
    pub contest_name: String,
    pub standings_global: Vec<crate::domain::StandingRow>,
    pub standings_home: Vec<crate::domain::StandingRow>,
    pub user_standing: crate::domain::StandingRow,
    pub scoreboard: Vec<crate::domain::ScoreboardRow>,
    /// Ranking e telão são dados de exemplo, não de jogadores de verdade.
    ///
    /// Sem este aviso o jogador lia "você está em 42º" como fato. Quem sabe de onde os
    /// dados vêm é o Core, então é ele quem desliga o aviso quando o placar tiver API.
    pub standings_are_sample: bool,
    /// Segundos até login, verificação, cadastro e troca de senha voltarem a valer.
    /// Zero quando não há bloqueio.
    pub auth_cooldown_seconds: u32,
    /// Segundos até enviar ou reenviar código voltar a valer. Inclui o bloqueio geral:
    /// o limite por IP também barra o envio.
    pub resend_cooldown_seconds: u32,
}

#[effect(facet_typegen)]
#[facet(fg::namespace = "LogN")]
pub enum Effect {
    Render(RenderOperation),
    Http(HttpRequest),
    SecureStore(KeyValueOperation),
    Telemetry(crate::domain::TelemetryOperation),
    Monitoring(crate::domain::MonitoringOperation),
    Time(TimeRequest),
    StoreReview(crate::domain::StoreReviewOperation),
}

/// XP por nível. O DS fixa a fórmula: `nível = floor(xp / 200) + 1`.
pub const XP_PER_LEVEL: i32 = 200;

/// Nível a partir do XP acumulado. Mora aqui, não no cliente.
pub fn level_for_xp(xp: i32) -> i32 {
    xp.max(0) / XP_PER_LEVEL + 1
}

/// Nós dominados em que o Core dispara o pedido de avaliação na loja.
///
/// [2, 4, 7]: passado o nó de entrada, no meio da trilha e ao concluir tudo.
/// São três pedidos — o limite anual da Apple, usados nos momentos mais significativos.
pub const REVIEW_PROMPT_MILESTONES: &[usize] = &[2, 4, 7];

/// Todos os desafios do nó já pagaram XP. Nó sem desafio nunca está dominado
/// (evita verdade vazia em nós que ainda não têm conteúdo).
pub fn node_mastered(challenges: &[Challenge], paid: &[String], node_id: &str) -> bool {
    let node_challenges: Vec<_> = challenges.iter().filter(|c| c.node_id == node_id).collect();
    !node_challenges.is_empty() && node_challenges.iter().all(|c| paid.contains(&c.id))
}

/// Quantos nós estão dominados (todos os desafios pagos).
pub fn mastered_node_count(nodes: &[crate::domain::SkillNode], challenges: &[Challenge], paid: &[String]) -> usize {
    nodes.iter().filter(|n| node_mastered(challenges, paid, &n.id)).count()
}

/// Verdadeiro quando `mastered_count` é um milestone que ainda não foi disparado.
pub fn should_request_review(mastered_count: usize, fired: &[usize]) -> bool {
    REVIEW_PROMPT_MILESTONES.contains(&mastered_count) && !fired.contains(&mastered_count)
}

/// Guarda o retrato local e renderiza.
fn save_offline_snapshot(model: &Model) -> Command<Effect, Event> {
    let snapshot = OfflineSnapshot {
        global_xp: model.global_xp,
        bugs_found: model.bugs_found,
        dry_runs_completed: model.dry_runs_completed,
        paid_challenge_ids: model.paid_challenges.clone(),
        nodes: model.nodes.clone(),
        challenges: model.challenges.clone(),
    };
    Command::request_from_shell(KeyValueOperation::Set {
        key: "offline_snapshot".to_string(),
        value: serde_json::to_vec(&snapshot).unwrap_or_default(),
    })
    .then_send(Event::SnapshotSaved)
}

/// Credita no modelo os aceitos que ainda estão na fila de sync e que a lista de pagos
/// não conhece: XP, contadores e o próprio desafio.
///
/// Chamado logo depois de o XP e a lista virem de quem não viu a fila — o servidor ou
/// o retrato. Sem isto, reabrir o app com respostas na fila mostrava o desafio como
/// resolvido e o XP dele sumido, e como rejogar não paga mais, ele não voltava até o
/// sync. Só conta o que não está na lista, então não paga duas vezes.
fn credit_queued_answers(model: &mut Model) {
    let queued: Vec<(String, String)> = model
        .pending_events
        .iter()
        .filter(|e| e.event_type == "MATCH_ANSWER")
        .filter_map(|e| serde_json::from_str::<serde_json::Value>(&e.payload_json).ok())
        .filter(|p| p["is_correct"] == serde_json::Value::Bool(true))
        .filter_map(|p| {
            let id = p["challenge_id"].as_str()?.to_string();
            let template = p["template_type"].as_str().unwrap_or_default().to_string();
            (!id.is_empty()).then_some((id, template))
        })
        .collect();

    for (id, template) in queued {
        if model.paid_challenges.contains(&id) {
            continue;
        }
        model.paid_challenges.push(id);
        model.global_xp += match_engine::XP_PER_ACCEPTED;
        match template.as_str() {
            "SPOT_THE_BUG" => model.bugs_found += 1,
            "DRY_RUN" => model.dry_runs_completed += 1,
            _ => {}
        }
    }
}

/// O retrato local da conta: o que a tela precisa quando não há rede.
///
/// Sem ele, abrir o app offline mostrava 0 XP e a árvore de exemplo — a trilha de
/// outra pessoa, na prática. Um app offline-first devolve o que era seu.
#[derive(Serialize, Deserialize, Default)]
struct OfflineSnapshot {
    global_xp: i32,
    bugs_found: i32,
    dry_runs_completed: i32,
    /// `default`: o retrato gravado antes desta regra não tem o campo.
    #[serde(default)]
    paid_challenge_ids: Vec<String>,
    nodes: Vec<crate::domain::SkillNode>,
    challenges: Vec<Challenge>,
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
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status = "Logging in".to_string();
                model.status_key = StatusKey::SigningIn;
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

                // O render vai junto com o request: sem ele o shell só recebe o
                // `is_authenticating` quando a resposta já chegou e a flag já desligou,
                // e o botão nunca mostra que está esperando a rede.
                Command::request_from_shell(request)
                    .then_send(Event::LoginCompleted)
                    .and(render::render())
            }

            Event::LoginCompleted(result) => {
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct AuthResp { access_token: String, refresh_token: String, #[serde(default)] user_id: String, #[serde(default)] refresh_expires_at: i64 }
                        
                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
                            if data.refresh_expires_at > 0 {
                                model.session_expires_at = data.refresh_expires_at;
                            }
                            model.session_offline = false;
                            model.is_guest = false;
                            // A tela de login mostra o status como erro, em vermelho: um
                            // "deu certo" ali é ruído. A prova do sucesso é o app abrir.
                            model.status_key = StatusKey::Silent;

                            // Salva refresh token no Keychain
                            return Command::request_from_shell(KeyValueOperation::Set {
                                key: "refresh_token".to_string(),
                                value: data.refresh_token.into_bytes()
                            }).then_send(Event::TokenStored);
                        } else {
                            model.status_key = StatusKey::ServerUnreadable;
                            model.is_authenticating = false;
                        }
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.status_key = StatusKey::WrongCredentials;
                        model.is_authenticating = false;
                    }
                    HttpResult::Ok(_) => {
                        model.status_key = StatusKey::SignInFailed;
                        model.is_authenticating = false;
                    }
                    HttpResult::Err(_) => {
                        model.status_key = StatusKey::NoConnection;
                        model.is_authenticating = false;
                    }
                }
                render::render()
            }

            // Visitante joga o jogo de verdade, não uma trilha inventada: os endpoints
            // de conteúdo são públicos, e só o progresso fica sem sincronizar. Se não
            // houver rede, a semente do bundle já encheu o modelo na abertura.
            Event::ContinueAsGuest => {
                model.is_guest = true;
                model.status = "Modo Visitante".to_string();
                self.update(Event::FetchNodes, model)
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
                // O prazo da sessão desce para o cofre com o resto: é ele que permite
                // abrir o app sem rede sem precisar perguntar nada ao servidor.
                Command::request_from_shell(KeyValueOperation::Set {
                    key: "session_expires_at".to_string(),
                    value: model.session_expires_at.to_string().into_bytes(),
                })
                .then_send(Event::SessionExpiryStored)
            }

            // Token rotacionado no disco: renova o prazo guardado e segue para saber
            // de quem é a sessão. O e-mail fica onde está — quem o escreve é o login.
            Event::RotatedTokenStored(_) => {
                Command::request_from_shell(KeyValueOperation::Set {
                    key: "session_expires_at".to_string(),
                    value: model.session_expires_at.to_string().into_bytes(),
                })
                .then_send(|_| Event::AttemptRefreshDone)
            }

            Event::AttemptRefreshDone => {
                Command::request_from_shell(KeyValueOperation::Get {
                    key: "account_email".to_string(),
                })
                .then_send(Event::AccountEmailRead)
            }

            Event::SessionExpiryStored(_) => {
                // Entrou agora: identifica na telemetria e busca o que já está no servidor.
                Command::request_from_shell(crate::domain::TelemetryOperation::Identify { user_id: model.user_id.clone() })
                    .then_send(|_| Event::TelemetrySent)
                    .and(self.update(Event::FetchProgress, model))
            }

            Event::Logout => {
                model.status = "Logging out".to_string();
                model.status_key = StatusKey::SigningOut;
                Command::request_from_shell(KeyValueOperation::Delete { key: "refresh_token".into() }).then_send(Event::TokenCleared)
            }
            Event::TokenCleared(result) => {
                // Guarda o que dá para devolver. Sair estando sincronizado é
                // reversível, e o DS troca o alerta de confirmação por um desfazer:
                // alerta em toda saída treina o usuário a confirmar sem ler.
                let refresh_token = match result {
                    KeyValueResult::Ok {
                        response: KeyValueResponse::Delete { previous: crux_kv::Value::Bytes(bytes) },
                    } => Some(bytes),
                    _ => None,
                };
                model.logout_undo = Some(LogoutSnapshot {
                    access_token: model.access_token.take(),
                    refresh_token,
                    was_guest: model.is_guest,
                    email: std::mem::take(&mut model.account_email),
                    user_id: std::mem::take(&mut model.user_id),
                    session_expires_at: model.session_expires_at,
                    nodes: std::mem::take(&mut model.nodes),
                    challenges: std::mem::take(&mut model.challenges),
                    pending_events: std::mem::take(&mut model.pending_events),
                });

                model.is_guest = false;
                model.session_offline = false;
                model.session_expires_at = 0;
                model.review_prompt_pending = false;
                // Quem conta que a saída deu certo é a tela de despedida. Deixar texto
                // aqui fazia a tela de login abrir com "Logged out successfully" em
                // vermelho, como se sair fosse um erro.
                model.status_key = StatusKey::Silent;
                render::render()
            }

            Event::UndoLogout => {
                let Some(snapshot) = model.logout_undo.take() else {
                    return render::render();
                };
                model.access_token = snapshot.access_token;
                model.is_guest = snapshot.was_guest;
                model.account_email = snapshot.email;
                model.user_id = snapshot.user_id;
                model.session_expires_at = snapshot.session_expires_at;
                model.nodes = snapshot.nodes;
                model.challenges = snapshot.challenges;
                model.pending_events = snapshot.pending_events;
                model.status_key = StatusKey::Silent;

                // A saída apagou o refresh token do cofre. Sem regravar, o desfazer
                // durava até o app fechar: a abertura seguinte caía no login.
                match snapshot.refresh_token {
                    Some(token) => Command::request_from_shell(KeyValueOperation::Set {
                        key: "refresh_token".to_string(),
                        value: token,
                    })
                    .then_send(Event::LogoutUndone)
                    .and(render::render()),
                    None => render::render(),
                }
            }

            Event::LogoutUndone(_) => render::render(),

            Event::Tick { now } => {
                model.now = now;
                Command::done()
            }

            Event::CooldownClock { now } => {
                model.now = now;
                model.resend_cooldown.advance(now);
                model.auth_cooldown.advance(now);

                let active = model.resend_cooldown.is_active() || model.auth_cooldown.is_active();
                if !active {
                    // Acabou a espera: o aviso sai junto com a trava.
                    if model.status_key == StatusKey::RateLimited {
                        model.status_key = StatusKey::Silent;
                    }
                    return render::render();
                }

                // Um relógio só por vez, por mais 429 que cheguem enquanto ele corre.
                if model.cooldown_timer_running {
                    return render::render();
                }
                model.cooldown_timer_running = true;
                let (tick, _handle) = Time::notify_after(std::time::Duration::from_secs(1));
                tick.then_send(|_| Event::CooldownElapsed).and(render::render())
            }

            Event::CooldownElapsed => {
                model.cooldown_timer_running = false;
                ask_cooldown_clock()
            }

            // Grava o retrato local. Chamado depois de cada coisa que o servidor confirma.
            Event::SnapshotSaved(_) => render::render(),

            Event::SnapshotRestored(result) => {
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result {
                    if let Ok(snap) = serde_json::from_slice::<OfflineSnapshot>(&bytes) {
                        model.global_xp = snap.global_xp;
                        model.bugs_found = snap.bugs_found;
                        model.dry_runs_completed = snap.dry_runs_completed;
                        model.paid_challenges = snap.paid_challenge_ids;
                        credit_queued_answers(model);
                        if !snap.nodes.is_empty() {
                            model.nodes = snap.nodes;
                            model.challenges = snap.challenges;
                            // O retrato é o que o servidor respondeu por esta conta, e
                            // é mais novo que a semente por definição.
                            model.trail_from_bundle = false;
                        }
                    }
                }

                // Sem retrato guardado, quem enche o modelo é a semente do bundle, que
                // o shell entrega na abertura. Antes entrava a trilha de mock aqui, e o
                // jogador via Arrays & Strings e Sliding Window sem saber que aquilo não
                // existia — pior, respondia desafios cujo node_id não está no banco.
                Command::request_from_shell(KeyValueOperation::Get {
                    key: "account_email".to_string(),
                })
                .then_send(Event::AccountEmailRead)
            }

            Event::DismissPasswordReset => {
                model.password_reset_done = false;
                render::render()
            }

            Event::DismissLogoutNotice => {
                model.logout_undo = None;
                render::render()
            }

            Event::AttemptRefresh => {
                model.status = "Refreshing session".to_string();
                model.status_key = StatusKey::ResumingSession;
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
                model.status_key = StatusKey::Silent;
                model.access_token = None;
                model.is_authenticating = false;
                render::render()
            }

            Event::RefreshCompleted(result) => {
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct RefreshResp {
                            access_token: String,
                            #[serde(default)]
                            refresh_token: String,
                            #[serde(default)]
                            user_id: String,
                            #[serde(default)]
                            refresh_expires_at: i64,
                        }

                        if let Ok(data) = serde_json::from_slice::<RefreshResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
                            if data.refresh_expires_at > 0 {
                                model.session_expires_at = data.refresh_expires_at;
                            }
                            model.is_guest = false;
                            model.session_offline = false;
                            // Retomar a sessão é invisível por definição: nada a dizer.
                            model.status_key = StatusKey::Silent;

                            // O servidor rotaciona o refresh token a cada uso. Guardar o
                            // novo não é opcional: na abertura seguinte o antigo já está
                            // revogado, e apresentá-lo desloga quem não fez nada errado.
                            if !data.refresh_token.is_empty() {
                                // Evento próprio, não `TokenStored`: aquele grava o
                                // `account_email` do modelo, que no refresh ainda está
                                // vazio — passar por ele apagava do cofre quem é a sessão.
                                return Command::request_from_shell(KeyValueOperation::Set {
                                    key: "refresh_token".to_string(),
                                    value: data.refresh_token.into_bytes(),
                                })
                                .then_send(Event::RotatedTokenStored);
                            }

                            // O `/refresh` devolve só o token; quem a sessão é fica no cofre.
                            return Command::request_from_shell(KeyValueOperation::Get {
                                key: "account_email".to_string(),
                            })
                            .then_send(Event::AccountEmailRead);
                        } else {
                            model.status = "Failed to parse refresh response".to_string();
                        }
                    }
                    // Sem rede, ou o servidor pedindo para esperar (429). Nos dois casos a
                    // sessão guardada pode estar perfeitamente válida — quem decide é o
                    // prazo. O 429 caía no braço de baixo e deslogava quem não tinha
                    // feito nada: bastava o limite por IP estourar num Wi-Fi cheio.
                    HttpResult::Ok(response) if response.status == 429 => {
                        return Command::request_from_shell(KeyValueOperation::Get {
                            key: "session_expires_at".to_string(),
                        })
                        .then_send(Event::OfflineSessionChecked);
                    }
                    HttpResult::Err(_) => {
                        return Command::request_from_shell(KeyValueOperation::Get {
                            key: "session_expires_at".to_string(),
                        })
                        .then_send(Event::OfflineSessionChecked);
                    }
                    // O servidor respondeu e recusou: aí a sessão acabou mesmo.
                    _ => {
                        model.status_key = StatusKey::SessionExpired;
                        model.access_token = None;
                        model.session_offline = false;
                        model.session_expires_at = 0;
                        model.is_authenticating = false;
                    }
                }
                render::render()
            }

            Event::OfflineSessionChecked(result) => {
                model.is_authenticating = false;

                let expires_at = match result {
                    KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } => {
                        String::from_utf8(bytes).ok().and_then(|s| s.parse::<i64>().ok()).unwrap_or(0)
                    }
                    _ => 0,
                };

                if expires_at > model.now && model.now > 0 {
                    model.session_expires_at = expires_at;
                    model.session_offline = true;
                    model.is_guest = false;
                    model.status_key = StatusKey::Silent;

                    // Offline não há o que buscar: devolve o retrato da última vez que
                    // o servidor respondeu — o XP e a trilha que são desta conta.
                    return Command::request_from_shell(KeyValueOperation::Get {
                        key: "offline_snapshot".to_string(),
                    })
                    .then_send(Event::SnapshotRestored);
                }

                // Sem prazo guardado, ou prazo vencido: não dá para afirmar que há sessão.
                model.status_key = StatusKey::Silent;
                model.access_token = None;
                model.session_offline = false;
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
                self.update(Event::FetchProgress, model)
            }

            // Traz de volta o que já está no servidor.
            //
            // O XP vivia só na memória do app: o sync subia os eventos e o login
            // seguinte abria com zero, com o progresso inteiro guardado do outro lado.
            Event::FetchProgress => {
                if model.access_token.is_none() {
                    // Renderiza mesmo sem ter o que buscar: este evento é o último elo
                    // da cadeia de abertura, e sair daqui em silêncio deixava a tela
                    // parada no login enquanto o modelo já estava com a sessão de pé.
                    return render::render();
                }

                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: "/api/v1/progress".to_string(),
                    headers: auth_headers(&model.access_token),
                    body: vec![],
                };

                Command::request_from_shell(request).then_send(Event::ProgressFetched)
            }

            Event::ProgressFetched(result) => {
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct Stats {
                            global_xp: i32,
                            bugs_found: i32,
                            dry_runs_completed: i32,
                            #[serde(default)]
                            paid_challenge_ids: Vec<String>,
                        }

                        if let Ok(stats) = serde_json::from_slice::<Stats>(&response.body) {
                            model.global_xp = stats.global_xp;
                            model.bugs_found = stats.bugs_found;
                            model.dry_runs_completed = stats.dry_runs_completed;
                            model.paid_challenges = stats.paid_challenge_ids;
                            credit_queued_answers(model);
                            return save_offline_snapshot(model);
                        }
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.pending_retry_event = Some(Event::FetchProgress);
                        return self.update(Event::AttemptRefresh, model);
                    }
                    // Offline ou erro: o progresso local segue valendo e sobe na fila.
                    _ => {}
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
                                // Substitui a lista inteira, e não completa a que
                                // estava: servidor que devolve menos nós — um nó
                                // removido — tem de encolher a trilha, não conviver
                                // com sobra da semente.
                                model.nodes = nodes;
                                model.trail_from_bundle = false;
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
                    // Sem rede o app fica com o que já tem — semente do bundle, retrato
                    // guardado, ou nada. Trocar por uma trilha fictícia era mentir para
                    // o jogador e gerar evento de sync com node_id que não existe.
                    HttpResult::Err(_) => {
                        model.status = "Offline: mantendo a trilha que já estava".to_string();
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
                                model.trail_from_bundle = false;
                                model.status = "Challenges loaded".to_string();
                                // Nós e desafios confirmados pelo servidor: é o momento
                                // de guardar o retrato que vai servir sem rede.
                                return save_offline_snapshot(model);
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
                        model.status = "Offline: mantendo os desafios que já estavam".to_string();
                    }
                }
                render::render()
            }

            Event::RequestOTP { email, purpose } => {
                if model.resend_cooldown.is_active() || model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.otp_email = email.clone();
                model.status = "Sending verification code".to_string();
                model.status_key = StatusKey::SendingCode;

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
                Command::request_from_shell(request)
                    .then_send(Event::OTPRequested)
                    .and(render::render())
            }
            Event::OTPRequested(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        // A tela já diz para onde o código foi; repetir aqui só enche
                        // o rodapé de status. Sucesso é o campo de código aparecer.
                        model.status_key = StatusKey::Silent;
                    }
                    // Pediu código de novo antes do intervalo do servidor, ou estourou o
                    // limite por IP. Os dois voltam 429 com o mesmo corpo; trava só o
                    // envio, porque o código que já chegou continua valendo.
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, true);
                    }
                    _ => {
                        model.status_key = StatusKey::CodeSentFailed;
                    }
                }
                render::render()
            }
            Event::VerifyOTP { email, code, purpose } => {
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status = "Verifying code".to_string();
                model.status_key = StatusKey::CheckingCode;

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
                Command::request_from_shell(request)
                    .then_send(Event::OTPVerified)
                    .and(render::render())
            }
            Event::OTPVerified(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        model.otp_verified = true;
                        // A tela avança para a senha; dizer "verificado" é redundante.
                        model.status_key = StatusKey::Silent;
                    }
                    // Não é código errado: o servidor nem conferiu. Dizer "código
                    // inválido" aqui fazia o jogador apagar um código bom.
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    _ => {
                        model.otp_verified = false;
                        model.status_key = StatusKey::CodeInvalid;
                    }
                }
                render::render()
            }
            Event::Register { email, password, otp } => {
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status = "Creating account".to_string();
                model.status_key = StatusKey::CreatingAccount;
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
                Command::request_from_shell(request)
                    .then_send(Event::RegisterCompleted)
                    .and(render::render())
            }
            Event::RegisterCompleted(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct AuthResp { access_token: String, refresh_token: String, #[serde(default)] user_id: String, #[serde(default)] refresh_expires_at: i64 }

                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
                            if data.refresh_expires_at > 0 {
                                model.session_expires_at = data.refresh_expires_at;
                            }
                            model.session_offline = false;
                            model.is_guest = false;
                            model.otp_verified = false;
                            model.otp_email = String::new();
                            // Conta criada: a prova é o app abrir. Status aqui vira
                            // ruído em vermelho na tela seguinte.
                            model.status_key = StatusKey::Silent;

                            return Command::request_from_shell(KeyValueOperation::Set {
                                key: "refresh_token".to_string(),
                                value: data.refresh_token.into_bytes(),
                            }).then_send(Event::TokenStored);
                        }
                        model.status_key = StatusKey::ServerUnreadable;
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    _ => {
                        model.status_key = StatusKey::AccountFailed;
                    }
                }
                render::render()
            }
            Event::ResetPassword { email, new_password, otp } => {
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status = "Resetting password".to_string();
                model.status_key = StatusKey::ResettingPassword;
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
                Command::request_from_shell(request)
                    .then_send(Event::ResetPasswordCompleted)
                    .and(render::render())
            }
            Event::ResetPasswordCompleted(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct AuthResp { access_token: String, refresh_token: String, #[serde(default)] user_id: String, #[serde(default)] refresh_expires_at: i64 }

                        if let Ok(data) = serde_json::from_slice::<AuthResp>(&response.body) {
                            model.access_token = Some(data.access_token);
                            if !data.user_id.is_empty() {
                                model.user_id = data.user_id;
                            }
                            if data.refresh_expires_at > 0 {
                                model.session_expires_at = data.refresh_expires_at;
                            }
                            model.session_offline = false;
                            model.is_guest = false;
                            model.otp_verified = false;
                            model.otp_email = String::new();
                            model.status_key = StatusKey::Silent;
                            model.password_reset_done = true;

                            return Command::request_from_shell(KeyValueOperation::Set {
                                key: "refresh_token".to_string(),
                                value: data.refresh_token.into_bytes(),
                            }).then_send(Event::TokenStored);
                        }
                        model.status_key = StatusKey::ServerUnreadable;
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    _ => {
                        model.status_key = StatusKey::ResetFailed;
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
                    model.status_key = StatusKey::SignInToSync;
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
                    model.status_key = StatusKey::SessionExpired;
                    model.logout_after_sync = false;
                    return render::render();
                }

                model.is_syncing = true;
                model.status = "Syncing".to_string();
                model.status_key = StatusKey::Syncing;

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
                            model.status_key = StatusKey::Silent;
                            model.pending_events.clear();
                            model.rebase_attempted = false;

                            // O topo que o servidor confirmou vira o ponto de partida do
                            // próximo evento, e desce para o disco junto com a fila vazia.
                            #[derive(Deserialize)]
                            struct SyncOk { new_top: String }
                            if let Ok(ok) = serde_json::from_slice::<SyncOk>(&response.body) {
                                if !ok.new_top.is_empty() {
                                    model.last_hash = ok.new_top;
                                }
                            }

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
                            // Rebase: o conteúdo da fila continua bom, só o
                            // encadeamento é que partiu do lugar errado. Reencadeia a
                            // partir do topo do servidor e tenta de novo, uma vez — sem
                            // isso a fila ficava presa para sempre e o jogador via
                            // "divergiu do servidor" sem nada que pudesse fazer.
                            #[derive(Deserialize)]
                            struct Rebase { server_top: String }

                            let top = serde_json::from_slice::<Rebase>(&response.body)
                                .map(|r| r.server_top)
                                .unwrap_or_default();

                            if !model.rebase_attempted && !top.is_empty() && !model.pending_events.is_empty() {
                                model.rebase_attempted = true;
                                model.pending_events = GameEvent::rebase(&model.pending_events, &top);
                                model.last_hash = model.pending_events[model.pending_events.len() - 1]
                                    .current_hash
                                    .clone();

                                let bytes = serde_json::to_vec(&model.pending_events).unwrap_or_default();
                                return Command::request_from_shell(KeyValueOperation::Set {
                                    key: "offline_events".to_string(),
                                    value: bytes,
                                })
                                .then_send(|_| Event::SyncNow);
                            }

                            model.status_key = StatusKey::SyncDiverged;
                        } else if response.status == 401 {
                            // Intercept 401 and attempt refresh
                            model.pending_retry_event = Some(Event::SyncNow);
                            return self.update(Event::AttemptRefresh, model);
                        } else {
                            model.status_key = StatusKey::SyncFailed;
                        }
                    }
                    HttpResult::Err(_err) => {
                        model.status_key = StatusKey::SyncOffline;
                    }
                }
                // Falhou: fica. Sair aqui apagaria a fila que não subiu.
                model.logout_after_sync = false;
                render::render()
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
                // Abrir uma partida apaga o registro da saída anterior, senão a tela
                // nova nasce achando que já a abandonaram.
                model.match_left = false;
                model.review_prompt_pending = false;
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
                            explanation: c.payload.validation.explanation.clone().unwrap_or_default(),
                            options: c.payload.content.options.clone().unwrap_or_default(),
                            correct_options: c.payload.content.correct_options.clone().unwrap_or_default(),
                            max_selections: if c.template_type == "TAG_THE_PATTERN" { c.payload.content.correct_options.as_ref().map_or(1, |o| o.len() as i32) } else { 1 },
                            origin: c.origin.clone(),
                            // A régua é do template; o desafio só entra se for atípico.
                            seconds: c.payload.content.seconds.unwrap_or_else(|| {
                                match_engine::seconds_for_template(&c.template_type)
                            }),
                            watch_variables: c.payload.content.watch_variables.clone().unwrap_or_default(),
                            watch_note: c.payload.content.watch_note.clone().unwrap_or_default(),
                            already_paid: model.paid_challenges.contains(&c.id),
                        }
                    })
                    .collect();

                if problems.is_empty() {
                    model.match_state = None; // clear previous state
                    model.status = "No challenges for this node".to_string();
                    return render::render();
                }

                model.match_state = Some(match_engine::MatchState::new(problems));
                model.match_node_id = node_id.clone();
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
                    let challenge_id = ms.current_problem().map(|p| p.challenge_id.clone()).unwrap_or_default();

                    let verdict = ms.submit();
                    let is_correct = verdict == match_engine::VerdictCode::Accepted;

                    // Paga uma vez por desafio; os contadores seguem a mesma regra. O
                    // servidor decide de novo no sync, pela tabela dele.
                    if is_correct && !challenge_id.is_empty() && !model.paid_challenges.contains(&challenge_id) {
                        let node_id = model.match_node_id.clone();
                        let was_mastered = node_mastered(&model.challenges, &model.paid_challenges, &node_id);

                        model.paid_challenges.push(challenge_id.clone());
                        model.global_xp += match_engine::XP_PER_ACCEPTED;
                        match template.as_str() {
                            "SPOT_THE_BUG" => model.bugs_found += 1,
                            "DRY_RUN" => model.dry_runs_completed += 1,
                            _ => {}
                        }

                        if !was_mastered && node_mastered(&model.challenges, &model.paid_challenges, &node_id) {
                            let count = mastered_node_count(&model.nodes, &model.challenges, &model.paid_challenges);
                            if should_request_review(count, &model.review_milestones_fired) {
                                model.review_prompt_pending = true;
                            }
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

                    let payload = format!(
                        r#"{{"letter":"{}","is_correct":{},"template_type":"{}","node_id":"{}","challenge_id":"{}"}}"#,
                        letter, is_correct, template, model.match_node_id, challenge_id
                    );
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

                // A fila vai para o disco a cada resposta.
                //
                // Só empilhar na memória não basta: fechar o app entre a resposta e o
                // sync apagava tudo — o oposto de offline-first.
                let bytes = serde_json::to_vec(&model.pending_events).unwrap_or_default();
                Command::request_from_shell(KeyValueOperation::Set {
                    key: "offline_events".to_string(),
                    value: bytes,
                })
                .then_send(Event::QueueSavedForSync)
                .and(render::render())
            }

            // Recupera a fila que ficou no disco de uma sessão anterior, e junto com ela
            // as origens já lidas — as duas são estado de disco e chegam pelo mesmo
            // gatilho de abertura, então não vale um evento a mais no shell.
            Event::RestoreOfflineQueue => {
                Command::request_from_shell(KeyValueOperation::Get {
                    key: "offline_events".to_string(),
                })
                .then_send(Event::OfflineQueueRestored)
                .and(
                    Command::request_from_shell(KeyValueOperation::Get {
                        key: "origins_seen".to_string(),
                    })
                    .then_send(Event::OriginsSeenRestored),
                )
                .and(
                    Command::request_from_shell(KeyValueOperation::Get {
                        key: "review_milestones_fired".to_string(),
                    })
                    .then_send(Event::ReviewMilestonesRestored),
                )
            }

            Event::OfflineQueueRestored(result) => {
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result {
                    if let Ok(queue) = serde_json::from_slice::<Vec<GameEvent>>(&bytes) {
                        if !queue.is_empty() {
                            // O topo da cadeia também volta: sem ele o próximo evento
                            // encadeia a partir do gênesis e o servidor pede rebase.
                            model.last_hash = queue[queue.len() - 1].current_hash.clone();
                            model.pending_events = queue;
                            credit_queued_answers(model);
                        }
                    }
                }
                render::render()
            }

            // A semente só preenche o que está vazio. Retrato guardado e resposta do
            // servidor são mais novos, e sobrescrevê-los faria o app regredir de
            // conteúdo a cada abertura.
            Event::BundledTrailLoaded { json } => {
                match serde_json::from_str::<crate::domain::TrailSeed>(&json) {
                    Ok(seed) if seed.version == crate::domain::TRAIL_SEED_VERSION => {
                        let encheu = model.nodes.is_empty() || model.challenges.is_empty();
                        if model.nodes.is_empty() {
                            model.nodes = seed.nodes;
                        }
                        if model.challenges.is_empty() {
                            model.challenges = seed.challenges;
                        }
                        // Só marca quando a semente de fato entrou. Se o retrato já
                        // tinha enchido tudo, o jogador não está vendo conteúdo velho.
                        if encheu {
                            model.trail_from_bundle = true;
                            model.trail_generated_at = seed.generated_at;
                        }
                    }
                    // Semente de outra versão é ignorada, não é erro: o app segue
                    // buscando pela rede como sempre buscou.
                    _ => {}
                }
                render::render()
            }

            // Sair da partida. Antes disto não havia saída: quem abrisse um desafio sem
            // saber a resposta ficava preso até perder as três vidas.
            //
            // Confirma só quando há partida a perder — nenhum balão no ar e as três
            // vidas de pé quer dizer que não há nada a confirmar. Pergunta que aparece
            // quando não há o que perguntar ensina a dispensá-la sem ler, e aí ela
            // também não funciona quando importa.
            Event::LeaveMatch => {
                let tem_o_que_perder = match model.match_state.as_ref() {
                    Some(ms) => ms.solved_count() > 0 || ms.lives < ms.max_lives,
                    None => false,
                };

                if !tem_o_que_perder {
                    return self.update(Event::ConfirmLeaveMatch, model);
                }

                if let Some(ref mut ms) = model.match_state {
                    // O relógio para enquanto a pessoa decide: perguntar e continuar
                    // contando é cobrar pela pergunta.
                    ms.leave_pending = true;
                    ms.is_paused = true;
                }
                render::render()
            }

            Event::CancelLeaveMatch => {
                if let Some(ref mut ms) = model.match_state {
                    ms.leave_pending = false;
                    // Só devolve o relógio se não for o cartão de origem que o segura.
                    ms.is_paused = !model.origin_sheet.is_empty();
                }
                render::render()
            }

            // Sair não custa XP: ele entra por resposta aceita e já foi para a fila.
            // O que acaba é a partida.
            //
            // A partida abandonada fecha com `MATCH_END` e `abandoned`, para quem ler o
            // histórico saber que ela existiu e como terminou — sem isso, só a partida
            // jogada até o fim tinha fim. Não é derrota nem vitória; o que ela vale para
            // quem ler, o flag deixa decidir depois. Quem entrou e saiu sem responder
            // nada não gera evento: é o nó aberto por engano, não uma partida.
            Event::ConfirmLeaveMatch => {
                let answered = model.match_state.as_ref().map_or(0, |ms| ms.answered_count());
                let solved = model.match_state.as_ref().map_or(0, |ms| ms.solved_count());

                model.match_state = None;
                model.match_node_id.clear();
                model.origin_sheet.clear();
                model.match_left = true;
                model.review_prompt_pending = false;

                if answered == 0 {
                    return render::render();
                }

                Time::now()
                    .then_send(move |t| Event::MatchAbandonedAt { solved, now: unix_seconds(t) })
                    .and(render::render())
            }

            Event::MatchAbandonedAt { solved, now } => {
                let previous_hash = if model.last_hash.is_empty() {
                    "0000000000000000000000000000000000000000000000000000000000000000".to_string()
                } else {
                    model.last_hash.clone()
                };
                let end_event = GameEvent::new(
                    format!("match_{}_left", now),
                    "MATCH_END".into(),
                    format!(r#"{{"solved":{},"abandoned":true}}"#, solved),
                    now,
                    previous_hash,
                );
                model.last_hash = end_event.current_hash.clone();
                model.pending_events.push(end_event);

                // Não passa por `QueueSavedForSync`: ele manda telemetria de resposta
                // lendo o último evento da fila, e a saída não é resposta.
                let bytes = serde_json::to_vec(&model.pending_events).unwrap_or_default();
                Command::request_from_shell(KeyValueOperation::Set {
                    key: "offline_events".to_string(),
                    value: bytes,
                })
                .then_send(|_| Event::SyncNow)
            }

            // O selo de origem abre o cartão de homenagem. A primeira leitura de cada
            // origem para o relógio: ler de onde o problema veio não pode custar a
            // questão. Da segunda em diante o relógio segue, senão o cartão vira um
            // botão de pausa.
            Event::OpenOriginSheet => {
                let origin = model
                    .match_state
                    .as_ref()
                    .and_then(|ms| ms.current_problem())
                    .map(|p| p.origin.clone())
                    .unwrap_or_default();

                if origin.is_empty() {
                    return render::render();
                }

                let primeira_vez = !model.origins_seen.iter().any(|o| o == &origin);
                model.origin_sheet = origin.clone();

                if !primeira_vez {
                    return render::render();
                }

                model.origins_seen.push(origin);
                if let Some(ref mut ms) = model.match_state {
                    ms.is_paused = true;
                }

                match serde_json::to_vec(&model.origins_seen) {
                    Ok(bytes) => Command::request_from_shell(KeyValueOperation::Set {
                        key: "origins_seen".to_string(),
                        value: bytes,
                    })
                    .then_send(Event::OriginsSeenStored)
                    .and(render::render()),
                    Err(_) => render::render(),
                }
            }

            Event::CloseOriginSheet => {
                model.origin_sheet.clear();
                if let Some(ref mut ms) = model.match_state {
                    ms.is_paused = false;
                }
                render::render()
            }

            Event::OriginsSeenStored(_) => render::render(),

            Event::OriginsSeenRestored(result) => {
                if let KeyValueResult::Ok {
                    response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) },
                } = result
                {
                    if let Ok(seen) = serde_json::from_slice::<Vec<String>>(&bytes) {
                        model.origins_seen = seen;
                    }
                }
                render::render()
            }

            // O jogador fechou o relatório pós-partida e volta para a trilha.
            // Se havia pedido de avaliação pendente, dispara e persiste o milestone.
            Event::MatchReportClosed => {
                if std::mem::take(&mut model.review_prompt_pending) {
                    let count = mastered_node_count(&model.nodes, &model.challenges, &model.paid_challenges);
                    model.review_milestones_fired.push(count);
                    let bytes = serde_json::to_vec(&model.review_milestones_fired).unwrap_or_default();
                    Command::notify_shell(crate::domain::StoreReviewOperation::RequestReview)
                        .build()
                        .and(
                            Command::request_from_shell(KeyValueOperation::Set {
                                key: "review_milestones_fired".to_string(),
                                value: bytes,
                            })
                            .then_send(Event::ReviewMilestonesFired),
                        )
                } else {
                    Command::done()
                }
            }

            Event::ReviewMilestonesFired(_) => render::render(),

            Event::ReviewMilestonesRestored(result) => {
                if let KeyValueResult::Ok {
                    response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) },
                } = result
                {
                    if let Ok(fired) = serde_json::from_slice::<Vec<usize>>(&bytes) {
                        model.review_milestones_fired = fired;
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
                    // Parado é parado: nem a questão nem a sessão andam enquanto o
                    // cartão de origem está aberto pela primeira vez.
                    if ms.is_active && !ms.is_paused {
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

        // Problema a problema, na ordem das letras da partida — a mesma ordem em que
        // `StartMatch` as distribui.
        for node in computed_nodes.iter_mut() {
            node.problems_solved = model.challenges.iter()
                .filter(|c| c.node_id == node.id)
                .map(|c| model.paid_challenges.contains(&c.id))
                .collect();
        }

        // "N balões no ar" conta nós conquistados, não problemas aceitos.
        let balloons_up = computed_nodes
            .iter()
            .filter(|n| n.status == crate::domain::NodeStatus::Completed)
            .count() as i32;

        ViewModel {
            status: model.status_key.clone(),
            pending_sync_count: model.pending_events.len() as u32,
            is_syncing: model.is_syncing,
            is_fetching: model.is_fetching,
            is_authenticating: model.is_authenticating,
            has_access_token: model.access_token.is_some(),
            has_session: model.access_token.is_some() || model.session_offline,
            is_offline_session: model.session_offline,
            trail_from_bundle: model.trail_from_bundle,
            trail_generated_at: model.trail_generated_at.clone(),
            match_left: model.match_left,
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
            password_reset_done: model.password_reset_done,
            // Depois de sair, o nome vem do instantâneo — é ele que a despedida usa.
            display_name: display_name_from_email(
                model.logout_undo.as_ref().map(|s| s.email.as_str()).unwrap_or(&model.account_email),
            ),
            match_view: model.match_state.as_ref()
                .map(|ms| ms.to_view_model(&model.origin_sheet))
                .unwrap_or_default(),
            // Placar e ranking ainda não têm API; os dados vivem no core para o
            // cliente seguir sendo uma camada burra, como manda a arquitetura.
            contest_name: "REGIONAL SUL-AMERICANA".to_string(),
            standings_global: crate::mock_data::get_mock_standings_global(),
            standings_home: crate::mock_data::get_mock_standings_home(),
            user_standing: crate::mock_data::get_mock_user_standing(),
            scoreboard: crate::mock_data::get_mock_scoreboard(),
            standings_are_sample: true,
            auth_cooldown_seconds: model.auth_cooldown.remaining(model.now),
            resend_cooldown_seconds: model
                .resend_cooldown
                .remaining(model.now)
                .max(model.auth_cooldown.remaining(model.now)),
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
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: crate::domain::TrapKey::InfiniteLoop,
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
                    seconds: None,
                },
                validation: ChallengeValidation {
                    validation_type: "LINE_MATCH".into(),
                    correct_line: Some(4),
                    expected_string: None,
                    explanation: None,
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

    /// Desfazer a saída devolve a sessão inteira, inclusive a do disco.
    ///
    /// O desfazer devolvia só o access token em memória: o refresh token já tinha sido
    /// apagado do cofre, e quem desfazia seguia jogando até fechar o app — na abertura
    /// seguinte, tela de login. A trilha também voltava vazia.
    #[test]
    fn test_undo_logout_puts_the_refresh_token_back() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = "66b670a2-41d2-4ba2-b863-78735b69ec7c".into();
        model.account_email = "jogador@example.com".into();
        model.session_expires_at = 1_792_600_000;
        model.nodes = vec![crate::domain::SkillNode {
            id: "10000000-0000-0000-0000-000000000001".into(),
            name: "Nó A".into(),
            description: crate::domain::TrapKey::FindTheBug,
            row: 0,
            column: 0,
            required_xp: 0,
            prerequisites: vec![],
            status: Default::default(),
            problems_solved: vec![],
        }];

        let mut cmd = app.update(Event::Logout, &mut model);
        match cmd.expect_one_effect() {
            Effect::SecureStore(r) => assert!(matches!(
                r.operation,
                KeyValueOperation::Delete { ref key } if key == "refresh_token"
            )),
            _ => panic!("esperava apagar o refresh token"),
        }

        let _ = app.update(
            Event::TokenCleared(KeyValueResult::Ok {
                response: KeyValueResponse::Delete {
                    previous: crux_kv::Value::Bytes(b"refresh_guardado".to_vec()),
                },
            }),
            &mut model,
        );
        assert!(model.access_token.is_none());
        assert!(app.view(&model).just_logged_out);

        let mut cmd = app.update(Event::UndoLogout, &mut model);
        let req = cmd
            .effects()
            .find(|e| matches!(e, Effect::SecureStore(_)))
            .expect("desfazer tem de regravar o refresh token");
        match req {
            Effect::SecureStore(r) => match r.operation {
                KeyValueOperation::Set { key, value } => {
                    assert_eq!(key, "refresh_token");
                    assert_eq!(value, b"refresh_guardado");
                }
                _ => panic!("esperava gravar o refresh token"),
            },
            _ => unreachable!(),
        }

        assert_eq!(model.access_token.as_deref(), Some("tok"));
        assert_eq!(model.user_id, "66b670a2-41d2-4ba2-b863-78735b69ec7c");
        assert_eq!(model.account_email, "jogador@example.com");
        assert_eq!(model.session_expires_at, 1_792_600_000);
        assert_eq!(model.nodes.len(), 1, "a trilha volta junto");
        assert!(!app.view(&model).just_logged_out);
    }

    /// O progresso do servidor manda no que a tela mostra.
    #[test]
    fn test_progress_comes_back_from_the_server() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());

        let mut cmd = app.update(Event::FetchProgress, &mut model);
        let req = cmd.expect_one_effect();
        assert!(matches!(req, Effect::Http(ref r) if r.operation.url == "/api/v1/progress"));

        let body = serde_json::to_vec(&serde_json::json!({
            "global_xp": 150, "bugs_found": 1, "dry_runs_completed": 1, "nodes": []
        })).unwrap();
        let _ = app.update(
            Event::ProgressFetched(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 200, headers: vec![], body,
            })),
            &mut model,
        );

        let view = app.view(&model);
        assert_eq!(view.global_xp, 150, "entrar de novo tem de devolver o XP");
        assert_eq!(view.bugs_found, 1);
        assert_eq!(view.dry_runs_completed, 1);
        assert_eq!(view.level, 1);
        assert_eq!(view.xp_to_next_level, 50);
    }

    /// O evento de partida precisa dizer de que nó veio.
    #[test]
    fn test_match_answer_event_carries_the_node() {
        use crate::domain::{Challenge, ChallengeContent, ChallengePayload, ChallengeValidation};

        let app = LogNApp::default();
        let mut model = Model::default();
        model.challenges = vec![Challenge {
            id: "ch_001".into(),
            node_id: "10000000-0000-0000-0000-000000000001".into(),
            template_type: "SPOT_THE_BUG".into(),
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: crate::domain::TrapKey::InfiniteLoop,
                    code_lines: vec!["while (a < b) {".into(), "    a = a;".into()],
                    options: None,
                    correct_options: None,
                    watch_variables: None,
                    watch_note: None,
                    seconds: None,
                },
                validation: ChallengeValidation {
                    validation_type: "LINE_MATCH".into(),
                    correct_line: Some(2),
                    expected_string: None,
                    explanation: None,
                },
            },
        }];

        let _ = app.update(
            Event::StartMatch { node_id: "10000000-0000-0000-0000-000000000001".into() },
            &mut model,
        );
        let _ = app.update(Event::MatchSelectLine { line: 1 }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);

        let event = model.pending_events.first().expect("a resposta vira evento de sync");
        assert!(
            event.payload_json.contains(r#""node_id":"10000000-0000-0000-0000-000000000001""#),
            "sem node_id o servidor não sabe a que trilha creditar: {}",
            event.payload_json
        );
    }

    /// Rejogar um desafio que já pagou não paga de novo — nem XP, nem contador.
    ///
    /// Cada aceito somava 50, e repetir o nó 1 abria o nó 7.
    #[test]
    fn test_a_challenge_pays_only_the_first_time() {
        use crate::domain::{Challenge, ChallengeContent, ChallengePayload, ChallengeValidation};
        const NODE: &str = "10000000-0000-0000-0000-000000000001";

        let bug = |id: &str| Challenge {
            id: id.into(),
            node_id: NODE.into(),
            template_type: "SPOT_THE_BUG".into(),
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: crate::domain::TrapKey::InfiniteLoop,
                    code_lines: vec!["while (a < b) {".into(), "    a = a;".into()],
                    options: None,
                    correct_options: None,
                    watch_variables: None,
                    watch_note: None,
                    seconds: None,
                },
                validation: ChallengeValidation {
                    validation_type: "LINE_MATCH".into(),
                    correct_line: Some(2),
                    expected_string: None,
                    explanation: None,
                },
            },
        };

        let app = LogNApp::default();
        let mut model = Model::default();
        model.challenges = vec![bug("ch_001"), bug("ch_002")];
        model.nodes = vec![crate::domain::SkillNode {
            id: NODE.into(),
            name: "Nó A".into(),
            description: crate::domain::TrapKey::FindTheBug,
            row: 0,
            column: 0,
            required_xp: 0,
            prerequisites: vec![],
            status: Default::default(),
            problems_solved: vec![],
        }];

        let play = |model: &mut Model, first_ts: i64| {
            let _ = app.update(Event::StartMatch { node_id: NODE.into() }, model);
            for i in 0..2 {
                let _ = app.update(Event::MatchSelectLine { line: 1 }, model);
                let _ = app.update(Event::MatchSubmit { timestamp: first_ts + i }, model);
            }
        };

        play(&mut model, 1_700_000_000);
        assert_eq!(model.global_xp, 100);
        assert_eq!(model.bugs_found, 2);
        let view = app.view(&model);
        assert_eq!(view.nodes[0].problems_solved, vec![true, true]);
        assert_eq!(view.match_view.xp_earned, 100);

        play(&mut model, 1_700_000_100);
        assert_eq!(model.global_xp, 100, "rejogar não paga XP");
        assert_eq!(model.bugs_found, 2, "o bug já tinha sido achado");
        let view = app.view(&model);
        assert_eq!(view.match_view.xp_earned, 0);
        assert!(view.match_view.balloon_states.iter().all(|b| b.is_accepted && b.already_paid));

        let last = model.pending_events.iter().rev()
            .find(|e| e.event_type == "MATCH_ANSWER")
            .expect("a resposta vira evento de sync");
        assert!(
            last.payload_json.contains(r#""challenge_id":"ch_002""#),
            "sem challenge_id o servidor não sabe se já pagou: {}",
            last.payload_json
        );
    }

    /// O que está na fila o servidor ainda não viu: continua pago depois do login, e o
    /// XP dele continua na conta. Sem isto, o desafio aparecia resolvido e o XP sumido —
    /// e como rejogar não paga mais, ele não voltava até o sync.
    #[test]
    fn test_paid_challenges_from_the_server_keep_the_queued_ones() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        let genesis = "0".repeat(64);
        model.pending_events = vec![
            GameEvent::new("match_1".into(), "MATCH_ANSWER".into(),
                r#"{"is_correct":true,"template_type":"DRY_RUN","challenge_id":"ch_fila"}"#.into(), 1, genesis.clone()),
            GameEvent::new("match_2".into(), "MATCH_ANSWER".into(),
                r#"{"is_correct":false,"challenge_id":"ch_errado"}"#.into(), 2, genesis),
        ];

        let body = serde_json::to_vec(&serde_json::json!({
            "global_xp": 50, "bugs_found": 1, "dry_runs_completed": 0, "nodes": [],
            "paid_challenge_ids": ["ch_servidor"]
        })).unwrap();
        let _ = app.update(
            Event::ProgressFetched(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 200, headers: vec![], body,
            })),
            &mut model,
        );

        assert_eq!(model.paid_challenges, vec!["ch_servidor".to_string(), "ch_fila".to_string()]);
        assert_eq!(model.global_xp, 100, "os 50 do servidor mais os 50 da fila");
        assert_eq!(model.dry_runs_completed, 1);

        // Chegar de novo não paga duas vezes: a fila já está na lista.
        let body = serde_json::to_vec(&serde_json::json!({
            "global_xp": 100, "bugs_found": 1, "dry_runs_completed": 1, "nodes": [],
            "paid_challenge_ids": ["ch_servidor", "ch_fila"]
        })).unwrap();
        let _ = app.update(
            Event::ProgressFetched(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 200, headers: vec![], body,
            })),
            &mut model,
        );
        assert_eq!(model.global_xp, 100);
    }

    /// Visitante não tem retrato: reabrir o app com a fila cheia devolve o XP dela.
    #[test]
    fn test_reopening_with_a_queue_gives_its_xp_back() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let queue = vec![GameEvent::new("match_1".into(), "MATCH_ANSWER".into(),
            r#"{"is_correct":true,"template_type":"SPOT_THE_BUG","challenge_id":"ch_001"}"#.into(),
            1, "0".repeat(64))];

        let _ = app.update(
            Event::OfflineQueueRestored(KeyValueResult::Ok {
                response: KeyValueResponse::Get {
                    value: crux_kv::Value::Bytes(serde_json::to_vec(&queue).unwrap()),
                },
            }),
            &mut model,
        );

        assert_eq!(model.global_xp, 50);
        assert_eq!(model.bugs_found, 1);
        assert_eq!(model.paid_challenges, vec!["ch_001".to_string()]);
    }

    /// Uma semente mínima, na forma que `just seed-bundle` gera.
    #[cfg(test)]
    fn seed_json() -> String {
        serde_json::json!({
            "version": crate::domain::TRAIL_SEED_VERSION,
            "generated_at": "2026-09-22T00:00:00Z",
            "nodes": [{
                "id": "10000000-0000-0000-0000-000000000001",
                "name": "Nó A",
                "description": "Descrição do nó A.",
                "row": 0, "column": 0, "required_xp": 0, "prerequisites": []
            }],
            "challenges": []
        })
        .to_string()
    }

    /// Monta um desafio com a forma que o seed do Postgres tem.
    #[cfg(test)]
    fn seeded_challenge(
        id: &str,
        template: &str,
        options: Vec<String>,
        correct: Vec<String>,
        explanation: &str,
    ) -> crate::domain::Challenge {
        use crate::domain::{Challenge, ChallengeContent, ChallengePayload, ChallengeValidation};
        Challenge {
            id: id.into(),
            node_id: "10000000-0000-0000-0000-000000000001".into(),
            template_type: template.into(),
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Merge Sort".into(),
                    description: crate::domain::TrapKey::FindTheBug,
                    code_lines: vec![],
                    options: Some(options),
                    correct_options: Some(correct),
                    watch_variables: None,
                    watch_note: None,
                    seconds: None,
                },
                validation: ChallengeValidation {
                    validation_type: template.into(),
                    correct_line: None,
                    expected_string: None,
                    explanation: Some(explanation.into()),
                },
            },
        }
    }

    /// A semente enche o que está vazio e não encosta no que já veio do servidor ou do
    /// retrato — sobrescrever faria o app regredir de conteúdo a cada abertura.
    #[test]
    fn test_bundled_trail_seeds_but_never_overwrites() {
        let app = LogNApp::default();

        // Modelo vazio: a semente entra.
        let mut model = Model::default();
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        assert_eq!(model.nodes.len(), 1, "sem nada no modelo, a semente enche");

        // Modelo com conteúdo mais novo: a semente passa sem tocar.
        let mut model = Model::default();
        model.nodes = vec![crate::domain::SkillNode {
            id: "70000000-0000-0000-0000-000000000007".into(),
            name: "Nó G".into(),
            description: crate::domain::TrapKey::FindTheBug,
            row: 4,
            column: 0,
            required_xp: 60,
            prerequisites: vec![],
            status: Default::default(),
            problems_solved: vec![],
        }];
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        assert_eq!(model.nodes.len(), 1);
        assert_eq!(
            model.nodes[0].name, "Nó G",
            "o que veio do servidor manda; a semente não regride o conteúdo"
        );
    }

    /// O relógio da questão sai do template, e o da sessão sai da soma.
    ///
    /// Antes os dois eram fixos — 60 e 180 — e isso tornava o da questão decorativo:
    /// com seis problemas a sessão dava 30 segundos por problema em média, então
    /// ninguém conseguia gastar o minuto em mais de três.
    #[test]
    fn test_each_template_gets_the_time_it_costs() {
        let app = LogNApp::default();
        let node = "10000000-0000-0000-0000-000000000001";

        // Prever saída custa muito mais que marcar padrão: três revisores cegos
        // independentes estimaram 70 a 150 segundos contra os 60 que havia.
        assert!(
            match_engine::seconds_for_template("DRY_RUN")
                > match_engine::seconds_for_template("TAG_THE_PATTERN"),
            "DRY_RUN não pode valer o mesmo que reconhecer um padrão"
        );

        let mut model = Model::default();
        let mut dry = seeded_challenge("ch_a", "DRY_RUN", vec![], vec![], "E.");
        dry.payload.validation.expected_string = Some("9".into());
        let tag = seeded_challenge(
            "ch_b", "TAG_THE_PATTERN",
            vec!["Pilha".into(), "Fila".into()],
            vec!["Pilha".into()],
            "E.",
        );
        model.challenges = vec![dry, tag];

        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let ms = model.match_state.as_ref().expect("a partida abre");

        assert_eq!(
            ms.question_seconds_remaining,
            match_engine::seconds_for_template("DRY_RUN"),
            "a primeira questão abre com o tempo do template dela"
        );

        let soma = match_engine::seconds_for_template("DRY_RUN")
            + match_engine::seconds_for_template("TAG_THE_PATTERN");
        assert_eq!(
            ms.contest_seconds_remaining,
            soma + soma / 5,
            "a sessão vale a soma dos problemas mais a folga, não um número fixo"
        );
    }

    /// O desafio atípico pode pedir outro tempo, e aí o dele manda.
    #[test]
    fn test_a_challenge_may_override_its_template_time() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut c = seeded_challenge(
            "ch_a", "TAG_THE_PATTERN",
            vec!["Pilha".into(), "Fila".into()],
            vec!["Pilha".into()],
            "E.",
        );
        c.payload.content.seconds = Some(200);
        model.challenges = vec![c];

        let node = "10000000-0000-0000-0000-000000000001";
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);

        assert_eq!(
            model.match_state.as_ref().unwrap().question_seconds_remaining,
            200,
            "o número do desafio manda sobre a régua do template"
        );
    }

    /// Confirmar só quando há o que perder. Pergunta que aparece quando não há nada a
    /// confirmar ensina o jogador a dispensá-la sem ler.
    #[test]
    fn test_leaving_asks_only_when_there_is_something_to_lose() {
        let app = LogNApp::default();
        let node = "10000000-0000-0000-0000-000000000001";

        fn um_desafio() -> Vec<crate::domain::Challenge> {
            vec![seeded_challenge(
                "ch_004", "COMPLEXITY_MATCH",
                vec!["O(n)".into(), "O(1)".into()],
                vec!["O(n)".into(), "O(1)".into()],
                "Irrelevante para este teste.",
            )]
        }

        // Partida intacta: três vidas, nada resolvido. Sai direto.
        let mut model = Model::default();
        model.challenges = um_desafio();
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let _ = app.update(Event::LeaveMatch, &mut model);
        assert!(model.match_state.is_none(), "sem nada a perder, o X sai direto");

        // Uma vida gasta: aí pergunta.
        let mut model = Model::default();
        model.challenges = um_desafio();
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let _ = app.update(Event::MatchSetDropTime { value: "O(1)".into() }, &mut model);
        let _ = app.update(Event::MatchSetDropSpace { value: "O(n)".into() }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);

        let _ = app.update(Event::LeaveMatch, &mut model);
        let ms = model.match_state.as_ref().expect("a partida continua enquanto pergunta");
        assert!(ms.leave_pending, "com vida gasta, pergunta antes");
        assert!(ms.is_paused, "e o relógio para enquanto a pessoa decide");

        // Desistir de sair devolve o relógio.
        let _ = app.update(Event::CancelLeaveMatch, &mut model);
        let ms = model.match_state.as_ref().unwrap();
        assert!(!ms.leave_pending);
        assert!(!ms.is_paused, "voltar para a partida volta a contar");

        // Confirmar encerra. O XP já ganho não é assunto daqui: ele entra por resposta
        // aceita e já está na fila de sync.
        let _ = app.update(Event::LeaveMatch, &mut model);
        let _ = app.update(Event::ConfirmLeaveMatch, &mut model);
        assert!(model.match_state.is_none(), "confirmar encerra a partida");
        assert!(model.match_node_id.is_empty(), "e solta o nó de onde ela saiu");
    }

    /// Sair de uma partida respondida fecha o contest com `MATCH_END` abandonado; sair
    /// sem ter respondido nada não deixa rastro.
    #[test]
    fn test_leaving_a_played_match_closes_it_as_abandoned() {
        let app = LogNApp::default();
        let node = "10000000-0000-0000-0000-000000000001";
        let desafio = || vec![seeded_challenge(
            "ch_004", "COMPLEXITY_MATCH",
            vec!["O(n)".into(), "O(1)".into()],
            vec!["O(n)".into(), "O(1)".into()],
            "Irrelevante para este teste.",
        )];

        // Entrou e saiu: nada na fila.
        let mut model = Model::default();
        model.challenges = desafio();
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let mut cmd = app.update(Event::ConfirmLeaveMatch, &mut model);
        assert!(model.pending_events.is_empty(), "nó aberto por engano não é partida");
        assert!(!cmd.effects().any(|e| matches!(e, Effect::Time(_))));

        // Respondeu errado e saiu: a resposta e o fim vão para a fila, encadeados.
        let mut model = Model::default();
        model.challenges = desafio();
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let _ = app.update(Event::MatchSetDropTime { value: "O(1)".into() }, &mut model);
        let _ = app.update(Event::MatchSetDropSpace { value: "O(n)".into() }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);
        let answer_hash = model.last_hash.clone();

        let mut cmd = app.update(Event::ConfirmLeaveMatch, &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Time(_))), "o fim pede a hora de agora");
        let _ = app.update(Event::MatchAbandonedAt { solved: 0, now: 1_700_000_050 }, &mut model);

        let end = model.pending_events.last().expect("a saída vira evento");
        assert_eq!(end.event_type, "MATCH_END");
        assert_eq!(end.payload_json, r#"{"solved":0,"abandoned":true}"#);
        assert_eq!(end.timestamp, 1_700_000_050);
        assert_eq!(end.previous_hash, answer_hash, "encadeado na resposta que veio antes");
        assert_eq!(model.last_hash, end.current_hash);
    }

    /// A semente envelhece com o binário, não com o conteúdo: quem instala e fica
    /// offline joga a trilha do dia do build. O app tem de conseguir dizer isso.
    #[test]
    fn test_the_app_knows_when_the_trail_is_the_frozen_one() {
        let app = LogNApp::default();

        let mut model = Model::default();
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        let view = app.view(&model);
        assert!(view.trail_from_bundle, "sem servidor, a trilha na tela é a do bundle");
        assert_eq!(
            view.trail_generated_at, "2026-09-22T00:00:00Z",
            "e o app sabe de quando ela é, para poder dizer na tela"
        );

        // Retrato do servidor por cima: deixa de ser conteúdo congelado.
        let snapshot = serde_json::to_vec(&serde_json::json!({
            "global_xp": 0, "bugs_found": 0, "dry_runs_completed": 0,
            "nodes": [{
                "id": "20000000-0000-0000-0000-000000000002",
                "name": "Nó B", "description": "Do servidor.",
                "row": 1, "column": -1, "required_xp": 100, "prerequisites": []
            }],
            "challenges": []
        }))
        .unwrap();
        let _ = app.update(
            Event::SnapshotRestored(KeyValueResult::Ok {
                response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(snapshot) },
            }),
            &mut model,
        );
        assert!(
            !app.view(&model).trail_from_bundle,
            "o retrato é o que o servidor respondeu, e é mais novo que a semente"
        );
    }

    /// Servidor que devolve menos nós tem de encolher a trilha, não conviver com a
    /// sobra da semente. Era lacuna declarada e sem teste.
    #[test]
    fn test_a_shorter_answer_from_the_server_shrinks_the_trail() {
        let app = LogNApp::default();
        let mut model = Model::default();

        // Semente com um nó.
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        assert_eq!(model.nodes.len(), 1);

        // Servidor responde com lista vazia: o nó tem de sumir.
        let _ = app.update(
            Event::NodesFetched(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 200, headers: vec![], body: b"[]".to_vec(),
            })),
            &mut model,
        );
        assert!(
            model.nodes.is_empty(),
            "a lista do servidor substitui a da semente inteira, não completa"
        );
        assert!(!model.trail_from_bundle, "e o app para de dizer que a trilha é a do bundle");
    }

    /// Semente de outra versão é ignorada em silêncio: o app segue buscando pela rede,
    /// que é o comportamento de antes de existir semente.
    #[test]
    fn test_bundled_trail_of_another_version_is_ignored() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let futura = seed_json().replace(
            &format!("\"version\":{}", crate::domain::TRAIL_SEED_VERSION),
            "\"version\":999",
        );
        let _ = app.update(Event::BundledTrailLoaded { json: futura }, &mut model);
        assert!(model.nodes.is_empty(), "versão que o app não conhece não entra");

        // E lixo no lugar da semente também não derruba nada.
        let _ = app.update(
            Event::BundledTrailLoaded { json: "não é json".into() },
            &mut model,
        );
        assert!(model.nodes.is_empty());
    }

    /// Ler de onde o problema veio não pode custar a questão — mas só na primeira vez
    /// de cada origem, senão o cartão vira um botão de pausa.
    #[test]
    fn test_origin_sheet_pauses_the_clock_only_the_first_time() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut ch = seeded_challenge(
            "ch_004",
            "COMPLEXITY_MATCH",
            vec!["O(n)".into(), "O(1)".into()],
            vec!["O(n)".into(), "O(1)".into()],
            "Irrelevante para este teste.",
        );
        ch.origin = "FARIAS".into();
        model.challenges = vec![ch];

        let node = "10000000-0000-0000-0000-000000000001";
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);

        // Primeira leitura: para o relógio e fica registrada.
        let _ = app.update(Event::OpenOriginSheet, &mut model);
        assert_eq!(model.origin_sheet, "FARIAS");
        assert!(model.match_state.as_ref().unwrap().is_paused, "a primeira leitura para o relógio");
        assert_eq!(model.origins_seen, vec!["FARIAS".to_string()]);

        let antes = model.match_state.as_ref().unwrap().question_seconds_remaining;
        let _ = app.update(Event::MatchTimerTick, &mut model);
        assert_eq!(
            model.match_state.as_ref().unwrap().question_seconds_remaining,
            antes,
            "o relógio não anda com o cartão aberto"
        );

        let _ = app.update(Event::CloseOriginSheet, &mut model);
        assert!(model.origin_sheet.is_empty());
        assert!(!model.match_state.as_ref().unwrap().is_paused, "fechar devolve o relógio");

        let _ = app.update(Event::MatchTimerTick, &mut model);
        assert_eq!(
            model.match_state.as_ref().unwrap().question_seconds_remaining,
            antes - 1,
            "fechado, o relógio volta a andar"
        );

        // Segunda leitura da mesma origem: abre, e o relógio segue correndo.
        let _ = app.update(Event::OpenOriginSheet, &mut model);
        assert_eq!(model.origin_sheet, "FARIAS", "o cartão abre de novo");
        assert!(
            !model.match_state.as_ref().unwrap().is_paused,
            "a cortesia vale uma vez por origem, senão vira botão de pausa"
        );
        assert_eq!(model.origins_seen.len(), 1, "a origem não entra duas vezes na lista");
    }

    /// O que veio do disco manda: quem já leu numa sessão anterior não ganha a pausa
    /// de novo ao reabrir o app.
    #[test]
    fn test_origins_seen_survives_from_storage() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.origins_seen = vec!["FARIAS".into()];

        let mut ch = seeded_challenge(
            "ch_004",
            "COMPLEXITY_MATCH",
            vec!["O(n)".into(), "O(1)".into()],
            vec!["O(n)".into(), "O(1)".into()],
            "Irrelevante para este teste.",
        );
        ch.origin = "FARIAS".into();
        model.challenges = vec![ch];

        let node = "10000000-0000-0000-0000-000000000001";
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let _ = app.update(Event::OpenOriginSheet, &mut model);

        assert_eq!(model.origin_sheet, "FARIAS");
        assert!(
            !model.match_state.as_ref().unwrap().is_paused,
            "já lida numa sessão anterior: abre sem parar o relógio"
        );
    }

    /// Desafio sem origem não tem selo, e tocar em nada não pode abrir cartão vazio.
    #[test]
    fn test_origin_sheet_stays_shut_without_an_origin() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.challenges = vec![seeded_challenge(
            "ch_004",
            "COMPLEXITY_MATCH",
            vec!["O(n)".into(), "O(1)".into()],
            vec!["O(n)".into(), "O(1)".into()],
            "Irrelevante para este teste.",
        )];

        let node = "10000000-0000-0000-0000-000000000001";
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let _ = app.update(Event::OpenOriginSheet, &mut model);

        assert!(model.origin_sheet.is_empty(), "sem origem não há cartão");
        assert!(!model.match_state.as_ref().unwrap().is_paused, "e nada para o relógio");
        assert!(model.origins_seen.is_empty());
    }

    /// COMPLEXITY_MATCH e TAG_THE_PATTERN: o motor julgava os dois e não havia
    /// desafio nenhum que os exercitasse, então nunca tinham rodado com dado real.
    #[test]
    fn test_choice_templates_judge_the_seeded_shape() {
        let app = LogNApp::default();

        // Complexidade: a ordem de `correct_options` é [tempo, espaço].
        let mut model = Model::default();
        model.challenges = vec![seeded_challenge(
            "ch_t04",
            "COMPLEXITY_MATCH",
            vec!["O(1)".into(), "O(log n)".into(), "O(n)".into(), "O(n²)".into()],
            vec!["O(n²)".into(), "O(1)".into()],
            "Dois laços aninhados dão O(n²) de tempo; o espaço é O(1).",
        )];
        let _ = app.update(Event::StartMatch { node_id: "10000000-0000-0000-0000-000000000001".into() }, &mut model);

        let _ = app.update(Event::MatchSetDropTime { value: "O(1)".into() }, &mut model);
        let _ = app.update(Event::MatchSetDropSpace { value: "O(n²)".into() }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);
        assert_eq!(app.view(&model).match_view.last_verdict, "WA", "trocar tempo e espaço é erro");
        assert_eq!(
            app.view(&model).match_view.trap_explanation,
            "Dois laços aninhados dão O(n²) de tempo; o espaço é O(1).",
            "a explicação tem de ser a do desafio, não a genérica"
        );

        let mut model = Model::default();
        model.challenges = vec![seeded_challenge(
            "ch_t04",
            "COMPLEXITY_MATCH",
            vec!["O(1)".into(), "O(n)".into(), "O(n²)".into()],
            vec!["O(n²)".into(), "O(1)".into()],
            "",
        )];
        let _ = app.update(Event::StartMatch { node_id: "10000000-0000-0000-0000-000000000001".into() }, &mut model);
        let _ = app.update(Event::MatchSetDropTime { value: "O(n²)".into() }, &mut model);
        let _ = app.update(Event::MatchSetDropSpace { value: "O(1)".into() }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);
        assert_eq!(app.view(&model).match_view.last_verdict, "AC");

        // Tags: a ordem em que o jogador marca não importa.
        let mut model = Model::default();
        model.challenges = vec![seeded_challenge(
            "ch_005",
            "TAG_THE_PATTERN",
            vec!["Grafos".into(), "BFS".into(), "DP".into(), "Greedy".into()],
            vec!["Grafos".into(), "BFS".into()],
            "",
        )];
        let _ = app.update(Event::StartMatch { node_id: "10000000-0000-0000-0000-000000000001".into() }, &mut model);
        assert_eq!(app.view(&model).match_view.max_selections, 2, "duas tags, duas marcações");

        let _ = app.update(Event::MatchToggleTag { tag: "BFS".into() }, &mut model);
        let _ = app.update(Event::MatchToggleTag { tag: "Grafos".into() }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);
        assert_eq!(app.view(&model).match_view.last_verdict, "AC");
    }

    /// Responder grava a fila; abrir de novo devolve ela e o topo da cadeia.
    ///
    /// O caminho da partida só empilhava na memória: fechar o app entre a resposta e
    /// o sync apagava o progresso, num app que promete offline-first.
    #[test]
    fn test_the_match_queue_survives_closing_the_app() {
        use crate::domain::{Challenge, ChallengeContent, ChallengePayload, ChallengeValidation};

        let app = LogNApp::default();
        let mut model = Model::default();
        model.challenges = vec![Challenge {
            id: "ch_001".into(),
            node_id: "node_1".into(),
            template_type: "SPOT_THE_BUG".into(),
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: crate::domain::TrapKey::InfiniteLoop,
                    code_lines: vec!["while (a < b) {".into(), "    a = a;".into()],
                    options: None,
                    correct_options: None,
                    watch_variables: None,
                    watch_note: None,
                    seconds: None,
                },
                validation: ChallengeValidation {
                    validation_type: "LINE_MATCH".into(),
                    correct_line: Some(2),
                    expected_string: None,
                    explanation: None,
                },
            },
        }];

        let _ = app.update(Event::StartMatch { node_id: "node_1".into() }, &mut model);
        let _ = app.update(Event::MatchSelectLine { line: 1 }, &mut model);
        let mut cmd = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);

        // A resposta desceu para o disco junto com o render.
        let stored = cmd
            .effects()
            .find_map(|e| match e {
                Effect::SecureStore(r) => match r.operation {
                    KeyValueOperation::Set { ref key, ref value } if key == "offline_events" => {
                        Some(value.clone())
                    }
                    _ => None,
                },
                _ => None,
            })
            .expect("a fila tem de ser gravada a cada resposta");

        // Um problema só: responder encerra a partida, então vão a resposta e o fim.
        let saved: Vec<GameEvent> = serde_json::from_slice(&stored).unwrap();
        assert_eq!(saved.len(), 2);

        // Sessão nova: o app abre sem nada na memória e recupera do disco.
        let mut fresh = Model::default();
        let result = KeyValueResult::Ok {
            response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(stored) },
        };
        let _ = app.update(Event::OfflineQueueRestored(result), &mut fresh);

        assert_eq!(fresh.pending_events.len(), saved.len(), "a fila volta inteira");
        assert_eq!(
            fresh.last_hash,
            saved[saved.len() - 1].current_hash,
            "e o topo da cadeia volta junto, senão o próximo evento parte do gênesis"
        );
        assert_eq!(app.view(&fresh).pending_sync_count, saved.len() as u32);
    }

    /// O 409 do servidor tem de virar rebase, não fila presa.
    ///
    /// O app reabria sem lembrar o topo da cadeia, o evento seguinte partia do gênesis
    /// e o servidor pedia rebase. O cliente só escrevia "divergiu do servidor" e a fila
    /// ficava lá, sem nada que o jogador pudesse fazer.
    #[test]
    fn test_a_rebase_reply_rechains_the_queue_and_retries() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = "66b670a2-41d2-4ba2-b863-78735b69ec7c".into();

        let orphan = GameEvent::new(
            "match_1".into(), "MATCH_ANSWER".into(), r#"{"is_correct":true}"#.into(),
            1_700_000_000, GameEvent::GENESIS.into(),
        );
        model.pending_events = vec![orphan.clone()];

        let server_top = "9ce55fdca5966683595a7282a7e7ebaabce4421ececadd97c8da5fc719f4aad5";
        let body = serde_json::to_vec(&serde_json::json!({
            "status": "rebase_required", "server_top": server_top, "events_applied": 0
        })).unwrap();

        let mut cmd = app.update(
            Event::SyncCompleted(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 409, headers: vec![], body,
            })),
            &mut model,
        );

        let rebased = &model.pending_events[0];
        assert_eq!(rebased.previous_hash, server_top, "reencadeou a partir do topo do servidor");
        assert_eq!(rebased.payload_json, orphan.payload_json, "o conteúdo é o mesmo");
        assert_ne!(rebased.current_hash, orphan.current_hash, "o hash muda junto com o elo");
        assert_eq!(model.last_hash, rebased.current_hash);

        // E grava a fila reencadeada antes de tentar de novo.
        let req = cmd.expect_one_effect();
        assert!(matches!(
            req,
            Effect::SecureStore(ref r) if matches!(r.operation, KeyValueOperation::Set { ref key, .. } if key == "offline_events")
        ));

        // Um rebase só: se o servidor insistir, para de tentar.
        let body = serde_json::to_vec(&serde_json::json!({
            "status": "rebase_required", "server_top": server_top, "events_applied": 0
        })).unwrap();
        let _ = app.update(
            Event::SyncCompleted(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 409, headers: vec![], body,
            })),
            &mut model,
        );
        assert_eq!(app.view(&model).status, StatusKey::SyncDiverged);
    }

    /// Sem rede, a sessão guardada continua valendo até o prazo dela.
    ///
    /// Abrir o app offline jogava para a tela de login mesmo com sessão dentro do
    /// prazo: pedia a senha para ver o que já estava no aparelho.
    #[test]
    fn test_an_unexpired_session_survives_having_no_network() {
        let app = LogNApp::default();
        let now = 1_790_000_000;

        // Dentro do prazo: entra, mesmo sem falar com o servidor.
        let mut model = Model::default();
        let _ = app.update(Event::Tick { now }, &mut model);
        let _ = app.update(Event::RefreshCompleted(HttpResult::Err(
            crux_http::HttpError::Io("offline".into()),
        )), &mut model);

        let expiry = (now + 60 * 60 * 24).to_string();
        let _ = app.update(
            Event::OfflineSessionChecked(KeyValueResult::Ok {
                response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(expiry.into_bytes()) },
            }),
            &mut model,
        );

        let view = app.view(&model);
        assert!(view.has_session, "sessão dentro do prazo não pode virar tela de login");
        assert!(view.is_offline_session, "e o app sabe que ainda não falou com o servidor");
        assert!(!view.has_access_token, "sem rede não há credencial nova");

        // O retrato da última vez que o servidor respondeu devolve o XP desta conta.
        let snapshot = serde_json::to_vec(&serde_json::json!({
            "global_xp": 300, "bugs_found": 2, "dry_runs_completed": 1,
            "nodes": [], "challenges": []
        })).unwrap();
        let _ = app.update(
            Event::SnapshotRestored(KeyValueResult::Ok {
                response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(snapshot) },
            }),
            &mut model,
        );
        let view = app.view(&model);
        assert_eq!(view.global_xp, 300, "offline mostra o último XP conhecido, não zero");

        // E quem garante trilha para jogar é a semente do bundle, não mais uma trilha
        // de mock: o retrato pode vir sem nós, e antes disso o app inventava quatro.
        assert!(view.nodes.is_empty(), "retrato sem nós não inventa nós");
        let _ = app.update(
            Event::BundledTrailLoaded { json: seed_json() },
            &mut model,
        );
        let view = app.view(&model);
        assert!(!view.nodes.is_empty(), "com a semente, há trilha para jogar");

        // Vencida: aí é login mesmo.
        let mut model = Model::default();
        let _ = app.update(Event::Tick { now }, &mut model);
        let expired = (now - 1).to_string();
        let _ = app.update(
            Event::OfflineSessionChecked(KeyValueResult::Ok {
                response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(expired.into_bytes()) },
            }),
            &mut model,
        );
        assert!(!app.view(&model).has_session, "prazo vencido não é sessão");

        // Servidor respondeu e recusou: não interessa o que está guardado.
        let mut model = Model::default();
        model.session_offline = true;
        model.session_expires_at = now + 999;
        let _ = app.update(Event::Tick { now }, &mut model);
        let _ = app.update(Event::RefreshCompleted(HttpResult::Ok(
            crux_http::protocol::HttpResponse { status: 401, headers: vec![], body: vec![] },
        )), &mut model);
        assert!(!app.view(&model).has_session, "recusa do servidor encerra a sessão");
    }

    /// O refresh rotaciona: o token novo tem de ser guardado.
    ///
    /// Se o cliente ignorar o `refresh_token` da resposta, a abertura seguinte
    /// apresenta um token já revogado e desloga quem não fez nada errado.
    #[test]
    fn test_the_rotated_refresh_token_is_stored() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let body = serde_json::to_vec(&serde_json::json!({
            "access_token": "novo_acesso",
            "refresh_token": "refresh_rotacionado",
            "user_id": "66b670a2-41d2-4ba2-b863-78735b69ec7c",
            "refresh_expires_at": 1_792_600_000i64,
        })).unwrap();

        let mut cmd = app.update(
            Event::RefreshCompleted(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 200, headers: vec![], body,
            })),
            &mut model,
        );

        let req = cmd.expect_one_effect();
        match req {
            Effect::SecureStore(r) => match r.operation {
                KeyValueOperation::Set { key, value } => {
                    assert_eq!(key, "refresh_token");
                    assert_eq!(String::from_utf8(value).unwrap(), "refresh_rotacionado");
                }
                _ => panic!("esperava gravar o refresh token rotacionado"),
            },
            _ => panic!("esperava efeito de cofre"),
        }

        assert_eq!(model.session_expires_at, 1_792_600_000);
        assert!(!model.session_offline, "falou com o servidor: a sessão está confirmada");

        // E a rotação não pode passar por `TokenStored`: aquele grava o `account_email`
        // do modelo, vazio no refresh, e apagava do cofre quem é a sessão.
        let mut cmd = app.update(
            Event::RotatedTokenStored(KeyValueResult::Ok {
                response: KeyValueResponse::Set { previous: crux_kv::Value::None },
            }),
            &mut model,
        );
        let req = cmd.expect_one_effect();
        match req {
            Effect::SecureStore(r) => match r.operation {
                KeyValueOperation::Set { key, .. } => assert_eq!(key, "session_expires_at"),
                _ => panic!("esperava gravar o prazo da sessão"),
            },
            _ => panic!("esperava efeito de cofre"),
        }
    }

    /// Nó sem desafio não vira partida.
    ///
    /// A árvore tem sete assuntos e o currículo escrito cobre um. Sem esta guarda o
    /// cliente navegava para uma partida sem problema nenhum e mostrava o relatório
    /// de "0 / 0 aceitos" — a tela de fim de uma sessão que nunca começou.
    #[test]
    fn test_a_node_without_challenges_starts_no_match() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.challenges = vec![]; // o nó existe na árvore, o conteúdo ainda não

        let _ = app.update(Event::StartMatch { node_id: "node_vazio".into() }, &mut model);

        assert!(model.match_state.is_none(), "não há partida a começar");
        assert!(!app.view(&model).match_view.is_active);
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

        let effects: Vec<_> = cmd.effects().collect();
        assert!(effects.iter().any(|e| matches!(e, Effect::Render(_))), "a espera tem de chegar à tela");
        match effects.into_iter().find(|e| matches!(e, Effect::Http(_))) {
            Some(Effect::Http(r)) => assert_eq!(r.operation.url, "/api/v1/auth/request-otp"),
            _ => panic!("Expected Http effect"),
        }

        // 2. OTP Requested Success
        let resp = crux_http::protocol::HttpResponse {
            status: 200,
            body: b"{}".to_vec(),
            headers: vec![],
        };
        let _ = app.update(Event::OTPRequested(HttpResult::Ok(resp)), &mut model);
        assert!(!model.is_authenticating);
        // Sucesso não diz nada ao jogador: a prova é a tela de código aparecer.
        assert_eq!(model.status_key, StatusKey::Silent);

        // 3. Verify OTP
        let mut cmd = app.update(Event::VerifyOTP { 
            email: "test@example.com".into(), 
            code: "123456".into(), 
            purpose: "verify_email".into() 
        }, &mut model);
        
        assert!(model.is_authenticating);

        let effects: Vec<_> = cmd.effects().collect();
        assert!(effects.iter().any(|e| matches!(e, Effect::Render(_))), "a espera tem de chegar à tela");
        match effects.into_iter().find(|e| matches!(e, Effect::Http(_))) {
            Some(Effect::Http(r)) => {
                assert_eq!(r.operation.url, "/api/v1/auth/verify-otp");
                assert!(String::from_utf8_lossy(&r.operation.body).contains("123456"));
            }
            _ => panic!("Expected Http effect"),
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
        assert_eq!(model.status_key, StatusKey::Silent, "verificar não fala; a tela avança");
    }

    #[test]
    fn test_login_flow() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut cmd = app.update(Event::Login { email: "test@x.com".into(), password_hash: "hash".into() }, &mut model);
        assert!(model.is_authenticating);

        // Sem o render junto do request, o botão "Entrar" não mostra que está esperando.
        let effects: Vec<_> = cmd.effects().collect();
        assert!(effects.iter().any(|e| matches!(e, Effect::Render(_))), "a espera tem de chegar à tela");
        match effects.into_iter().find(|e| matches!(e, Effect::Http(_))) {
            Some(Effect::Http(http_req)) => assert_eq!(http_req.operation.url, "/api/v1/auth/login"),
            _ => panic!("Expected Http effect"),
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

    fn too_many_requests(retry_after: Option<&str>) -> crux_http::protocol::HttpResponse {
        crux_http::protocol::HttpResponse {
            status: 429,
            headers: retry_after
                .map(|v| vec![crux_http::protocol::HttpHeader { name: "Retry-After".into(), value: v.into() }])
                .unwrap_or_default(),
            body: b"Too many requests\n".to_vec(),
        }
    }

    /// Resolve o `Time::now` pendente no comando e devolve o evento que ele gera.
    fn resolve_now(cmd: &mut Command<Effect, Event>, now: u64) -> Event {
        let mut request = cmd
            .effects()
            .find_map(|e| match e {
                Effect::Time(r) if r.operation == TimeRequest::Now => Some(r),
                _ => None,
            })
            .expect("o bloqueio pede a hora ao shell");
        request
            .resolve(crux_time::TimeResponse::Now { instant: crux_time::Instant::new(now, 0) })
            .unwrap();
        cmd.events().next().expect("a hora volta como evento")
    }

    /// O placar ainda sai do mock. Enquanto sair, a tela tem de dizer que é exemplo.
    #[test]
    fn test_standings_are_flagged_as_sample_while_they_come_from_the_mock() {
        let app = LogNApp::default();
        let view = app.view(&Model::default());
        assert!(view.standings_are_sample);
        assert!(!view.scoreboard.is_empty());
    }

    #[test]
    fn test_retry_after_is_read_and_bounded() {
        assert_eq!(retry_after_secs(&too_many_requests(Some("42"))), 42);
        assert_eq!(retry_after_secs(&too_many_requests(None)), 60);
        assert_eq!(retry_after_secs(&too_many_requests(Some("amanhã"))), 60);
        assert_eq!(retry_after_secs(&too_many_requests(Some("0"))), 60);
        assert_eq!(retry_after_secs(&too_many_requests(Some("99999"))), 3600);
    }

    /// Pedir código de novo cedo demais trava só o envio. O código que já chegou
    /// continua valendo, e o "Verificar" não pode ficar preso junto.
    #[test]
    fn test_429_on_request_otp_locks_only_resend_and_counts_down() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.now = 10; // hora velha da abertura: não pode ancorar o bloqueio

        let mut cmd = app.update(
            Event::OTPRequested(HttpResult::Ok(too_many_requests(Some("60")))),
            &mut model,
        );
        assert_eq!(model.status_key, StatusKey::RateLimited);
        assert!(!model.is_authenticating);

        let view = app.view(&model);
        assert_eq!(view.resend_cooldown_seconds, 60);
        assert_eq!(view.auth_cooldown_seconds, 0);

        let clock = resolve_now(&mut cmd, 1_000);
        let mut cmd = app.update(clock, &mut model);
        assert_eq!(model.resend_cooldown.until, 1_060, "ancorado na hora pedida, não na da abertura");
        assert!(
            cmd.effects().any(|e| matches!(e, Effect::Time(r) if matches!(r.operation, TimeRequest::NotifyAfter { .. }))),
            "com bloqueio ativo, o relógio volta a bater em um segundo"
        );

        // Trinta segundos depois, a contagem desce.
        let _ = app.update(Event::CooldownElapsed, &mut model);
        let _ = app.update(Event::CooldownClock { now: 1_030 }, &mut model);
        assert_eq!(app.view(&model).resend_cooldown_seconds, 30);

        // Com o envio travado, o toque não sai para o servidor.
        let mut cmd = app.update(
            Event::RequestOTP { email: "a@x.com".into(), purpose: "verify_email".into() },
            &mut model,
        );
        assert!(!cmd.effects().any(|e| matches!(e, Effect::Http(_))));

        // Mas verificar o código que já chegou continua valendo.
        let mut cmd = app.update(
            Event::VerifyOTP { email: "a@x.com".into(), code: "123456".into(), purpose: "verify_email".into() },
            &mut model,
        );
        assert!(cmd.effects().any(|e| matches!(e, Effect::Http(_))));

        // No fim da contagem, trava e aviso saem juntos.
        model.status_key = StatusKey::RateLimited;
        let _ = app.update(Event::CooldownElapsed, &mut model);
        let mut cmd = app.update(Event::CooldownClock { now: 1_060 }, &mut model);
        assert_eq!(app.view(&model).resend_cooldown_seconds, 0);
        assert_eq!(model.status_key, StatusKey::Silent);
        assert!(!cmd.effects().any(|e| matches!(e, Effect::Time(_))), "sem bloqueio, o relógio para");
    }

    /// 429 no login vem do limite por IP, que vale para todas as rotas de conta.
    #[test]
    fn test_429_on_login_locks_every_account_action() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.is_authenticating = true;

        let mut cmd = app.update(
            Event::LoginCompleted(HttpResult::Ok(too_many_requests(Some("45")))),
            &mut model,
        );
        let clock = resolve_now(&mut cmd, 2_000);
        let _ = app.update(clock, &mut model);

        let view = app.view(&model);
        assert_eq!(view.auth_cooldown_seconds, 45);
        assert_eq!(view.resend_cooldown_seconds, 45, "o envio de código também está barrado");
        assert_eq!(model.status_key, StatusKey::RateLimited);
        assert!(!model.is_authenticating);

        for event in [
            Event::Login { email: "a@x.com".into(), password_hash: "senha-forte".into() },
            Event::RequestOTP { email: "a@x.com".into(), purpose: "verify_email".into() },
            Event::VerifyOTP { email: "a@x.com".into(), code: "123456".into(), purpose: "verify_email".into() },
            Event::Register { email: "a@x.com".into(), password: "senha-forte".into(), otp: "123456".into() },
            Event::ResetPassword { email: "a@x.com".into(), new_password: "senha-forte".into(), otp: "123456".into() },
        ] {
            let mut cmd = app.update(event, &mut model);
            assert!(!cmd.effects().any(|e| matches!(e, Effect::Http(_))), "nada sai durante o bloqueio");
        }
    }

    /// Dois 429 seguidos não põem dois relógios para bater.
    #[test]
    fn test_second_429_does_not_start_a_second_timer() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut cmd = app.update(Event::LoginCompleted(HttpResult::Ok(too_many_requests(Some("30")))), &mut model);
        let clock = resolve_now(&mut cmd, 5_000);
        let _ = app.update(clock, &mut model);
        assert!(model.cooldown_timer_running);

        let mut cmd = app.update(Event::OTPRequested(HttpResult::Ok(too_many_requests(Some("60")))), &mut model);
        let clock = resolve_now(&mut cmd, 5_001);
        let mut cmd = app.update(clock, &mut model);
        assert!(!cmd.effects().any(|e| matches!(e, Effect::Time(r) if matches!(r.operation, TimeRequest::NotifyAfter { .. }))));
    }

    /// 429 no refresh não é sessão recusada. Deslogava quem não tinha feito nada,
    /// bastava o limite por IP estourar num Wi-Fi compartilhado.
    #[test]
    fn test_429_on_refresh_keeps_the_session() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.session_expires_at = 9_999_999_999;

        let mut cmd = app.update(Event::RefreshCompleted(HttpResult::Ok(too_many_requests(None))), &mut model);
        assert_ne!(model.status_key, StatusKey::SessionExpired);
        assert_eq!(model.session_expires_at, 9_999_999_999);
        let asked_expiry = cmd.effects().any(|e| matches!(e,
            Effect::SecureStore(r) if matches!(&r.operation, KeyValueOperation::Get { key } if key == "session_expires_at")));
        assert!(asked_expiry, "decide pelo prazo guardado, como sem rede");
    }
}
