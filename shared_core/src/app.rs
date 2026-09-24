use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crux_http::protocol::{HttpRequest, HttpResult};
use crux_kv::{KeyValueOperation, KeyValueResult, KeyValueResponse};
use crux_time::{Time, TimeRequest};
use crate::domain::{
    BootCheck, BootDetail, BootLine, BootVerdict, Challenge, GameEvent, StatusKey, SyncPayload,
    TelemetryOperation,
};
use crate::match_engine;

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[repr(C)]
#[facet(fg::namespace = "LogN")]
pub enum Event {
    SetLocale(String),
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
    /// A fila guardada sob a chave de `owner`. Chega depois de `claim_queue` e é
    /// descartada se o dono mudou no meio do caminho.
    OfflineQueueRestored { owner: String, result: KeyValueResult },
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
    /// `legal_accepted` é a caixa dos termos e da política. Quais versões e em que
    /// língua quem decide é o Core, com o que `FetchLegalVersions` trouxe: o shell não
    /// tem como saber a versão vigente.
    Register { email: String, password: String, otp: String, age_confirmed: bool, legal_accepted: bool },
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
    /// A tela de cadastro abriu: busca as versões vigentes dos termos e da política,
    /// que são as que o cadastro vai aceitar, e a idade mínima do país (ISO 3166-1
    /// alfa-2 ou alfa-3, vazio se o shell não souber). O país vai também no cadastro.
    FetchLegalVersions { country: String },
    LegalVersionsFetched(HttpResult),
    /// Pede a exclusão da conta, com a senha. O nome do campo segue o do `Login`: o que
    /// viaja é a senha em si, sobre TLS, e o servidor confere com Argon2.
    DeleteAccount { password_hash: String },
    AccountDeleted(HttpResult),
    /// Fechou a tela "conta desativada, apagada até DD/MM".
    DismissDeletionNotice,
    /// Fechou o aviso de que a exclusão foi cancelada pelo login.
    DismissAccountRestoredNotice,
    /// O interruptor "Análise de uso" do perfil.
    SetAnalyticsEnabled(bool),
    /// Abertura do app: lê a escolha do interruptor guardada no aparelho.
    RestoreAnalyticsPreference,
    AnalyticsPreferenceRestored(KeyValueResult),
    /// Abertura do app: confere a sessão, traz a fila de quem é a sessão e manda o que
    /// estiver nela. É o que a splash mostra, linha a linha.
    StartBoot,
    /// Passou o tempo que a abertura espera pela rede numa das verificações.
    /// `attempt` é a tentativa que pôs o relógio para correr: o de uma tentativa
    /// anterior não corta a de agora.
    BootWatchdogElapsed { check: crate::domain::BootCheck, attempt: u32 },
    /// Sem rede na abertura: tentar de novo.
    RetryBoot,
    /// Sem rede na abertura: entrar com o que está no aparelho.
    ContinueOffline,
    /// O e-mail da sessão que acabou, para o login já vir preenchido.
    ResumeEmailRead(KeyValueResult),
    /// `account_user_id` do aparelho: de quem é a sessão que abriu sem rede.
    QueueOwnerRead(KeyValueResult),
    /// Uma fila de outro dono (visitante, ou a chave de antes das filas por conta),
    /// lida para ser adotada pelo dono atual.
    QueueToAdoptRead { from: String, result: KeyValueResult },
    /// A fila adotada já está gravada sob o dono novo: a chave antiga pode sair.
    QueueAdopted { from: String },
    /// A fila de `owner` lida do disco, para tirar dela os eventos que um sync de antes
    /// da troca de dono já entregou.
    SyncedQueueRead { owner: String, sent: Vec<String>, result: KeyValueResult },
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
    pub queue_owner: String,
}

/// A abertura em andamento: o que a splash mostra.
#[derive(Default, Clone)]
pub struct Boot {
    pub active: bool,
    pub session: Option<crate::domain::BootLine>,
    pub sync: Option<crate::domain::BootLine>,
    pub awaiting_offline_choice: bool,
    /// A espera pela sessão estourou e a abertura seguiu pelo caminho sem rede. A
    /// resposta do refresh que chegar depois ainda vale — o servidor já trocou o token,
    /// e jogá-la fora deixava no cofre um token revogado —, mas um erro de rede tardio
    /// não roda o caminho sem rede de novo.
    pub session_timed_out: bool,
    /// Conta as tentativas ("Tentar de novo" começa outra).
    pub attempt: u32,
}

impl Boot {
    fn session_running(&self) -> bool {
        self.active && matches!(&self.session, Some(l) if l.verdict == BootVerdict::Running)
    }

    fn sync_running(&self) -> bool {
        self.active && matches!(&self.sync, Some(l) if l.verdict == BootVerdict::Running)
    }

    fn session_ok(&self) -> bool {
        matches!(&self.session, Some(l) if l.verdict == BootVerdict::Ok)
    }
}

fn boot_line(check: BootCheck, verdict: BootVerdict, detail: BootDetail, count: usize) -> BootLine {
    BootLine { check, verdict, detail, count: count as u32 }
}

/// Quanto a abertura espera pela rede em cada verificação antes de seguir sem ela.
/// Sem teto, o timeout padrão do `URLSession` prendia o jogador por um minuto.
const BOOT_WAIT_SECS: u64 = 5;

fn boot_watchdog(check: BootCheck, attempt: u32) -> Command<Effect, Event> {
    let (tick, _handle) = Time::notify_after(std::time::Duration::from_secs(BOOT_WAIT_SECS));
    tick.then_send(move |_| Event::BootWatchdogElapsed { check: check.clone(), attempt })
}

/// Dono da fila de quem joga sem conta.
const GUEST_QUEUE_OWNER: &str = "guest";

/// Onde a fila de `owner` mora no aparelho.
///
/// Era uma chave só, `offline_events`, e ela não dizia de quem era: entrar com outra
/// conta depois de a sessão expirar subia a fila da primeira como se fosse da segunda.
/// Dono vazio é essa chave antiga, que só existe até ser adotada.
fn queue_key(owner: &str) -> String {
    if owner.is_empty() {
        "offline_events".to_string()
    } else {
        format!("offline_events:{}", owner)
    }
}

/// Grava a fila do modelo sob a chave do dono dela.
fn store_queue(model: &Model) -> KeyValueOperation {
    KeyValueOperation::Set {
        key: queue_key(&model.queue_owner),
        value: serde_json::to_vec(&model.pending_events).unwrap_or_default(),
    }
}

/// A sessão acabou (recusa do servidor, prazo local vencido): lê o e-mail dela para o
/// login já vir preenchido.
fn read_resume_email() -> Command<Effect, Event> {
    Command::request_from_shell(KeyValueOperation::Get { key: "account_email".to_string() })
        .then_send(Event::ResumeEmailRead)
}

/// A splash como a tela a desenha. A barra anda meia linha quando uma verificação
/// começa e a outra meia quando ela fecha.
fn boot_view(boot: &Boot) -> crate::domain::BootViewModel {
    let lines: Vec<BootLine> = [&boot.session, &boot.sync].into_iter().flatten().cloned().collect();
    let steps: u32 = lines
        .iter()
        .map(|l| if l.verdict == BootVerdict::Running { 1 } else { 2 })
        .sum();
    crate::domain::BootViewModel {
        in_progress: boot.active,
        progress: (steps * 100 / 4).min(100) as u8,
        lines,
        awaiting_offline_choice: boot.awaiting_offline_choice,
    }
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
    pub locale: String,
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
    /// Versão vigente de cada documento legal, `(kind, version)`, como o servidor disse.
    ///
    /// O cadastro manda o aceite destas versões. Antes o app mandava `"terms_v1"` como
    /// texto, o servidor esperava objeto, e todo cadastro caía em 400.
    pub legal_versions: Vec<(String, u32)>,
    /// País considerado na confirmação de idade, como o shell informou. Vai no cadastro.
    pub legal_country: String,
    /// A semente do bundle, com as três línguas. Guardada para trocar de língua sem rede.
    pub bundled_seed: Option<crate::domain::TrailSeed>,
    /// Em que língua está o conteúdo que o modelo tem agora (`pt-BR`, `en`, `es`).
    /// Vazio antes de qualquer conteúdo chegar.
    pub content_locale: String,
    /// Idade mínima do país, como o servidor disse. 0 = ainda não se sabe.
    pub min_age: u32,
    /// Até quando a conta cuja exclusão acabou de ser pedida pode ser recuperada, em
    /// segundos desde a época. 0 = sem aviso na tela.
    pub deletion_purge_after: i64,
    /// O último login ou troca de senha cancelou uma exclusão pedida. O app avisa uma vez.
    pub account_restored_notice: bool,
    /// O jogador desligou "Análise de uso". Guardado ao contrário para o padrão do
    /// `Default` (falso) ser o padrão do produto: ligado.
    pub analytics_disabled: bool,
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
    pub boot: Boot,
    /// De quem é a fila que está na memória: o id da conta, `guest`, ou vazio para a
    /// chave antiga, sem dono.
    pub queue_owner: String,
    /// A fila do dono já veio do disco, com as adoções. Até lá, trocar de dono não tem
    /// o que salvar, e o sync da abertura espera.
    pub queue_loaded: bool,
    /// Filas que o dono atual ainda vai adotar, na ordem.
    pub queue_adoptions: Vec<String>,
    /// E-mail da última sessão que acabou. É o que o login mostra preenchido.
    pub resume_email: String,
    /// Há um `/auth/refresh` no ar. Um segundo com o mesmo token, antes de o primeiro
    /// voltar, é reuso para o servidor, e reuso derruba todas as sessões da conta.
    pub refresh_in_flight: bool,
    /// Os eventos que foram no sync que está no ar, pelo id, e de que dono é a fila.
    /// A resposta é sobre eles: o que entrou na fila durante o envio fica. O id
    /// sobrevive ao rebase, que só troca o elo.
    pub sync_sent_ids: Vec<String>,
    pub sync_owner: String,
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
    pub locale: String,
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
    /// Já se sabe quais versões dos termos e da política o cadastro aceita. Sem isso o
    /// cadastro não tem o que mandar, e a tela segura o envio do código.
    pub legal_versions_ready: bool,
    /// Idade mínima para criar conta no país do aparelho — o N de "tenho N anos ou
    /// mais". Enquanto o servidor não responde, a idade padrão.
    pub min_age: u32,
    /// Até quando a conta que acabou de pedir exclusão pode ser recuperada, em segundos
    /// desde a época. 0 = sem aviso. O cliente formata a data.
    pub deletion_purge_after: i64,
    /// Mostrar, uma vez, que a exclusão foi cancelada e a conta voltou.
    pub account_restored_notice: bool,
    /// Estado do interruptor "Análise de uso".
    pub analytics_enabled: bool,
    /// A splash da abertura.
    pub boot: crate::domain::BootViewModel,
    /// E-mail para o login já vir preenchido depois de uma sessão que acabou. Vazio
    /// quando não há.
    pub resume_email: String,
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
        locale: model.content_locale.clone(),
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
    /// Língua do conteúdo guardado. Vazio é o retrato de antes das línguas: pt-BR.
    #[serde(default)]
    locale: String,
}

/// Tira o "A · " da frente do nome do problema.
///
/// A letra é posição na partida, não parte do nome: o mesmo desafio pode ser o A de um
/// nó e o C de outro. O seed guardava "A · Soma de Dois Números" e o core prefixava de
/// novo, então o enunciado abria como "A · A · SOMA DE DOIS NÚMEROS".
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
fn auth_headers(token: &Option<String>, locale: &str) -> Vec<crux_http::protocol::HttpHeader> {
    let mut headers = vec![
        crux_http::protocol::HttpHeader {
            name: "Content-Type".to_string(),
            value: "application/json".to_string(),
        },
        crux_http::protocol::HttpHeader {
            name: "Accept-Language".to_string(),
            value: locale.to_string(),
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

fn base_headers(locale: &str) -> Vec<crux_http::protocol::HttpHeader> {
    vec![
        crux_http::protocol::HttpHeader {
            name: "Content-Type".to_string(),
            value: "application/json".to_string(),
        },
        crux_http::protocol::HttpHeader {
            name: "Accept-Language".to_string(),
            value: locale.to_string(),
        }
    ]
}

/// Os dois documentos que o cadastro precisa aceitar.
const LEGAL_KINDS: [&str; 2] = ["terms", "privacy"];

/// Idade mínima enquanto o servidor não disse a do país. É a mesma padrão dele.
const DEFAULT_MIN_AGE: u32 = 13;

/// Chave, no armazenamento do aparelho, da escolha "Análise de uso". O shell lê a
/// mesma chave ao iniciar o PostHog, antes de o Core existir.
const ANALYTICS_DISABLED_KEY: &str = "analytics_disabled";

/// Põe no modelo a trilha da semente na língua pedida, ou em português se ela faltar.
fn apply_seed(model: &mut Model, locale: &str) {
    let Some(seed) = model.bundled_seed.as_ref() else { return };
    let (used, trail) = match seed.locales.get(locale) {
        Some(t) => (locale.to_string(), t.clone()),
        None => match seed.locales.get("pt-BR") {
            Some(t) => ("pt-BR".to_string(), t.clone()),
            None => return,
        },
    };
    model.nodes = trail.nodes;
    model.challenges = trail.challenges;
    model.content_locale = used;
    model.trail_from_bundle = true;
}

/// A língua do conteúdo que o servidor mandou, do `Content-Language`. Servidor antigo
/// não manda: vale a língua pedida.
fn content_language(response: &crux_http::protocol::HttpResponse, asked: &str) -> String {
    response
        .headers
        .iter()
        .find(|h| h.name.eq_ignore_ascii_case("content-language"))
        .map(|h| served_locale(h.value.trim()).to_string())
        .unwrap_or_else(|| served_locale(asked).to_string())
}

/// Parte do corpo das respostas de login e troca de senha que o Core lê além da sessão.
#[derive(Deserialize, Default)]
struct RestoredFlag {
    #[serde(default)]
    account_restored: bool,
}

/// O código estável de um erro da API (`{"code": "email_taken", ...}`). O Core decide
/// por ele, e nunca pela frase: `message` é para log, e servidor antigo mandava texto
/// solto, que aqui vira `None`.
fn api_code(body: &[u8]) -> Option<String> {
    #[derive(Deserialize)]
    struct ApiError { code: String }
    serde_json::from_slice::<ApiError>(body).ok().map(|e| e.code)
}

/// O que dizer quando cadastro, envio de código ou troca de senha foi recusado por um
/// dado que a pessoa pode corrigir. `None` para o resto, e quem chama cai na mensagem
/// genérica da ação.
fn status_for_input_error(body: &[u8]) -> Option<StatusKey> {
    match api_code(body)?.as_str() {
        "invalid_email" => Some(StatusKey::InvalidEmail),
        "password_too_short" => Some(StatusKey::PasswordTooShort),
        "password_too_long" => Some(StatusKey::PasswordTooLong),
        "email_taken" => Some(StatusKey::EmailTaken),
        "otp_invalid" => Some(StatusKey::CodeInvalid),
        _ => None,
    }
}

/// Lê `account_restored` do corpo. O servidor antigo não manda o campo: vale falso.
fn account_restored(body: &[u8]) -> bool {
    serde_json::from_slice::<RestoredFlag>(body).map(|f| f.account_restored).unwrap_or(false)
}

/// Já se conhece a versão vigente dos dois documentos.
fn legal_versions_complete(versions: &[(String, u32)]) -> bool {
    LEGAL_KINDS.iter().all(|k| versions.iter().any(|(kind, _)| kind == k))
}

/// A língua servida para a língua do app, na forma do banco: `pt-BR`, `en` ou `es`. É a
/// língua em que o jogador leu os documentos e em que o conteúdo da trilha chega. Sem
/// língua conhecida, o português, que é o que prevalece.
fn served_locale(app_locale: &str) -> &'static str {
    let base = app_locale.split(['-', '_']).next().unwrap_or("").to_ascii_lowercase();
    match base.as_str() {
        "en" => "en",
        "es" => "es",
        _ => "pt-BR",
    }
}

/// O aceite que vai no corpo do cadastro: um por documento, na versão vigente.
fn legal_acceptances(versions: &[(String, u32)], app_locale: &str) -> serde_json::Value {
    let locale = served_locale(app_locale);
    serde_json::Value::Array(
        LEGAL_KINDS
            .iter()
            .filter_map(|k| versions.iter().find(|(kind, _)| kind == k))
            .map(|(kind, version)| serde_json::json!({ "kind": kind, "version": version, "locale": locale }))
            .collect(),
    )
}

impl LogNApp {
    /// Põe na memória a fila de `owner`, trazendo do disco a dele e, depois, as que ele
    /// adota (`adopt`, na ordem).
    ///
    /// A fila que estava na memória não se perde: toda mudança nela já desceu para o
    /// disco sob a chave do dono anterior. Trocar de dono é só ler outra chave.
    fn claim_queue(&self, model: &mut Model, owner: &str, adopt: &[&str]) -> Command<Effect, Event> {
        if model.queue_loaded && model.queue_owner == owner {
            // Mesmo dono: nada a ler. A abertura, se estiver esperando a fila, segue.
            return if model.boot.active { self.after_queue_loaded(model) } else { Command::done() };
        }
        model.queue_owner = owner.to_string();
        model.queue_loaded = false;
        model.queue_adoptions = adopt.iter().filter(|a| **a != owner).map(|a| a.to_string()).collect();
        model.pending_events.clear();
        model.last_hash.clear();
        model.rebase_attempted = false;

        let owner = owner.to_string();
        Command::request_from_shell(KeyValueOperation::Get { key: queue_key(&owner) })
            .then_send(move |result| Event::OfflineQueueRestored { owner: owner.clone(), result })
    }

    /// Lê a próxima fila a adotar, ou fecha o carregamento.
    fn next_adoption(&self, model: &mut Model) -> Command<Effect, Event> {
        if model.queue_adoptions.is_empty() {
            model.queue_loaded = true;
            return self.after_queue_loaded(model);
        }
        let from = model.queue_adoptions.remove(0);
        Command::request_from_shell(KeyValueOperation::Get { key: queue_key(&from) })
            .then_send(move |result| Event::QueueToAdoptRead { from: from.clone(), result })
    }

    /// A fila do dono está inteira na memória.
    fn after_queue_loaded(&self, model: &mut Model) -> Command<Effect, Event> {
        if model.boot.active {
            if !model.boot.session_ok() {
                return render::render();
            }
            if model.access_token.is_some() {
                return self.boot_start_sync(model);
            }
            // Sessão de pé sem rede. Com fila, a splash para e deixa o jogador escolher
            // entre tentar de novo e entrar com ela no aparelho. Sem fila não há o que
            // esperar da rede: entra direto, e a tarja de sem rede avisa.
            let queued = model.pending_events.len();
            model.boot.sync = Some(boot_line(BootCheck::Sync, BootVerdict::Warn, BootDetail::NoNetwork, queued));
            if queued == 0 {
                return self.finish_boot(model);
            }
            model.boot.awaiting_offline_choice = true;
            return render::render();
        }
        // Fora da abertura (acabou de entrar, adotou a fila do visitante): o que veio
        // junto sobe logo.
        if model.access_token.is_some() && !model.pending_events.is_empty() {
            return self.update(Event::SyncNow, model).and(render::render());
        }
        render::render()
    }

    fn boot_start_sync(&self, model: &mut Model) -> Command<Effect, Event> {
        let queued = model.pending_events.len();
        if queued == 0 {
            model.boot.sync = Some(boot_line(BootCheck::Sync, BootVerdict::Ok, BootDetail::NothingToSend, 0));
            return self.finish_boot(model);
        }
        model.boot.sync = Some(boot_line(BootCheck::Sync, BootVerdict::Running, BootDetail::Sending, queued));
        let sync = self.update(Event::SyncNow, model);
        // `SyncNow` pode recusar sem ir à rede (sem id de conta). Aí a linha fecha já.
        if !model.is_syncing && model.boot.sync_running() {
            model.boot.sync = Some(boot_line(BootCheck::Sync, BootVerdict::Warn, BootDetail::Rejected, 0));
            return sync.and(self.finish_boot(model));
        }
        sync.and(boot_watchdog(BootCheck::Sync, model.boot.attempt)).and(render::render())
    }

    /// Fecha a linha do sync da abertura, se ela estava rodando.
    fn settle_boot_sync(&self, model: &mut Model, verdict: BootVerdict, detail: BootDetail, count: usize) -> Command<Effect, Event> {
        if !model.boot.sync_running() {
            return Command::done();
        }
        model.boot.sync = Some(boot_line(BootCheck::Sync, verdict, detail, count));
        self.finish_boot(model)
    }

    /// A última linha fechou: a splash sai. Não há duração mínima.
    fn finish_boot(&self, model: &mut Model) -> Command<Effect, Event> {
        model.boot.active = false;
        model.boot.awaiting_offline_choice = false;
        render::render()
    }

    /// A sessão acabou (o servidor recusou, ou o prazo local venceu). A splash fecha na
    /// linha da sessão, o login vem com o e-mail dela, e a fila na memória passa a ser a
    /// do visitante — a da conta fica no disco, esperando ela voltar.
    fn session_ended(&self, model: &mut Model) -> Command<Effect, Event> {
        let boot = if model.boot.active {
            model.boot.session = Some(boot_line(BootCheck::Session, BootVerdict::Fail, BootDetail::SessionEnded, 0));
            model.boot.sync = None;
            self.finish_boot(model)
        } else {
            Command::done()
        };
        read_resume_email()
            .and(self.claim_queue(model, GUEST_QUEUE_OWNER, &[""]))
            .and(boot)
    }
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
            Event::SetLocale(lang) => {
                model.locale = lang;
                // A língua mudou com conteúdo já na tela: troca pelo da semente, que vale
                // sem rede, e pede ao servidor o da língua nova.
                let wanted = served_locale(&model.locale);
                if !model.content_locale.is_empty() && model.content_locale != wanted {
                    apply_seed(model, wanted);
                    return self.update(Event::FetchNodes, model).and(render::render());
                }
                Command::done()
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
                    headers: base_headers(&model.locale),
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
                            // Entrar dentro da carência cancelou a exclusão pedida.
                            model.account_restored_notice = account_restored(&response.body);
                            model.resume_email.clear();

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
                // A sessão que caiu no meio do jogo deixou a fila dela na memória, e o
                // visitante não pode gravar por cima.
                self.claim_queue(model, GUEST_QUEUE_OWNER, &[""])
                    .and(self.update(Event::FetchNodes, model))
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
                // Entrou agora (login, cadastro ou troca de senha). O id da conta desce
                // para o aparelho — é ele que diz de quem é a fila numa abertura sem
                // rede — e a fila passa a ser a desta conta. Quem jogou como visitante
                // antes de entrar leva o que jogou: a fila dele é adotada.
                //
                // A troca vem antes da busca do progresso: o XP que o servidor devolve
                // soma a fila que está na memória, e ela já tem de ser a desta conta.
                let owner = model.user_id.clone();
                let queue = if owner.is_empty() {
                    Command::done()
                } else {
                    Command::request_from_shell(KeyValueOperation::Set {
                        key: "account_user_id".to_string(),
                        value: owner.clone().into_bytes(),
                    })
                    .then_send(|_| Event::Ping)
                    .and(self.claim_queue(model, &owner, &[GUEST_QUEUE_OWNER, ""]))
                };

                // Identifica na telemetria e busca o que já está no servidor. Com a
                // análise de uso desligada, não identifica: o que o SDK ainda manda
                // (erros, medições) sai com identificador anônimo.
                if model.analytics_disabled {
                    return queue.and(self.update(Event::FetchProgress, model));
                }
                queue
                    .and(Command::request_from_shell(crate::domain::TelemetryOperation::Identify { user_id: model.user_id.clone() })
                        .then_send(|_| Event::TelemetrySent))
                    .and(self.update(Event::FetchProgress, model))
            }

            Event::Logout => {
                model.status = "Logging out".to_string();
                model.status_key = StatusKey::SigningOut;
                model.resume_email.clear();
                Command::request_from_shell(KeyValueOperation::Delete { key: "refresh_token".into() })
                    .then_send(Event::TokenCleared)
                    .and(Command::request_from_shell(KeyValueOperation::Delete { key: "account_email".into() }).then_send(|_| Event::Ping))
                    .and(Command::request_from_shell(KeyValueOperation::Delete { key: "account_user_id".into() }).then_send(|_| Event::Ping))
                    .and(Command::request_from_shell(KeyValueOperation::Delete { key: "session_expires_at".into() }).then_send(|_| Event::Ping))
                    // Só a fila de quem está saindo. A de outra conta, que tenha ficado
                    // no aparelho, espera a dona dela voltar.
                    .and(Command::request_from_shell(KeyValueOperation::Delete { key: queue_key(&model.queue_owner) }).then_send(|_| Event::Ping))
                    .and(Command::request_from_shell(KeyValueOperation::Delete { key: "offline_snapshot".into() }).then_send(|_| Event::Ping))
                    // Depois de sair, os eventos não seguem presos ao id da conta, e quem
                    // entrar em seguida neste aparelho começa limpo.
                    .and(Command::request_from_shell(TelemetryOperation::Reset).then_send(|_| Event::TelemetrySent))
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
                    queue_owner: model.queue_owner.clone(),
                });

                model.is_guest = false;
                model.session_offline = false;
                model.session_expires_at = 0;
                model.review_prompt_pending = false;
                // Quem conta que a saída deu certo é a tela de despedida. Deixar texto
                // aqui fazia a tela de login abrir com "Logged out successfully" em
                // vermelho, como se sair fosse um erro.
                model.status_key = StatusKey::Silent;
                // Quem jogar em seguida neste aparelho joga como visitante.
                self.claim_queue(model, GUEST_QUEUE_OWNER, &[""]).and(render::render())
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
                model.last_hash = model.pending_events.last().map(|e| e.current_hash.clone()).unwrap_or_default();
                // A fila do visitante que a saída começou a carregar chega depois e é
                // descartada: o dono voltou a ser quem saiu.
                model.queue_owner = snapshot.queue_owner;
                model.queue_loaded = true;
                model.queue_adoptions.clear();
                model.status_key = StatusKey::Silent;

                // A saída apagou do aparelho a sessão inteira, não só o token. Regravar
                // só o refresh token deixava a fila, o e-mail e o prazo na memória: fechar
                // o app depois do desfazer perdia a fila, e a abertura seguinte sem rede
                // caía no login por falta de prazo.
                let set = |key: &str, value: Vec<u8>| {
                    Command::request_from_shell(KeyValueOperation::Set { key: key.to_string(), value })
                        .then_send(|_| Event::Ping)
                };
                let token = match snapshot.refresh_token {
                    Some(token) => Command::request_from_shell(KeyValueOperation::Set {
                        key: "refresh_token".to_string(),
                        value: token,
                    })
                    .then_send(Event::LogoutUndone),
                    None => Command::done(),
                };
                let account = if snapshot.was_guest {
                    Command::done()
                } else {
                    set("account_email", model.account_email.clone().into_bytes())
                        .and(set("account_user_id", model.user_id.clone().into_bytes()))
                        .and(set("session_expires_at", model.session_expires_at.to_string().into_bytes()))
                        .and(save_offline_snapshot(model))
                };
                token
                    .and(account)
                    .and(Command::request_from_shell(store_queue(model)).then_send(|_| Event::Ping))
                    .and(render::render())
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
                        let snap_locale = if snap.locale.is_empty() { "pt-BR".to_string() } else { snap.locale };
                        // Retrato em outra língua (o app mudou de língua sem rede): o XP e
                        // os desafios pagos valem, o texto não. Fica a semente, que já
                        // entrou na língua certa, até a próxima busca com rede.
                        if !snap.nodes.is_empty() && snap_locale == served_locale(&model.locale) {
                            model.nodes = snap.nodes;
                            model.challenges = snap.challenges;
                            model.content_locale = snap_locale;
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
                        model.refresh_in_flight = true;
                        return Command::request_from_shell(request).then_send(Event::RefreshCompleted);
                    }
                }
                // Instead of showing an error on the login screen, we just remain silent
                model.status_key = StatusKey::Silent;
                model.access_token = None;
                model.is_authenticating = false;

                // Sem token no aparelho não há sessão a conferir: a abertura acaba aqui,
                // antes de a splash ter o que mostrar, e quem joga agora é o visitante.
                if model.boot.session_running() {
                    model.boot.session = None;
                    return self.claim_queue(model, GUEST_QUEUE_OWNER, &[""]).and(self.finish_boot(model));
                }
                render::render()
            }

            Event::RefreshCompleted(result) => {
                model.refresh_in_flight = false;
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

                            // Na abertura, a linha da sessão fecha aqui — mesmo que a
                            // espera já tenha estourado e a splash esteja parada no "sem
                            // rede": a rede voltou, e a abertura segue para o sync.
                            if model.boot.active {
                                model.boot.session = Some(boot_line(BootCheck::Session, BootVerdict::Ok, BootDetail::TokenRenewed, 0));
                                model.boot.sync = None;
                                model.boot.awaiting_offline_choice = false;
                            }

                            // O id desce para o aparelho a cada refresh: a abertura sem
                            // rede precisa dele para saber de quem é a fila. E a fila
                            // passa a ser a desta conta, com a da chave antiga adotada.
                            let owner = model.user_id.clone();
                            let queue = if owner.is_empty() {
                                Command::done()
                            } else {
                                Command::request_from_shell(KeyValueOperation::Set {
                                    key: "account_user_id".to_string(),
                                    value: owner.clone().into_bytes(),
                                })
                                .then_send(|_| Event::Ping)
                                .and(self.claim_queue(model, &owner, &[""]))
                            };

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
                                .then_send(Event::RotatedTokenStored)
                                .and(queue)
                                .and(render::render());
                            }

                            // O `/refresh` devolve só o token; quem a sessão é fica no cofre.
                            return Command::request_from_shell(KeyValueOperation::Get {
                                key: "account_email".to_string(),
                            })
                            .then_send(Event::AccountEmailRead)
                            .and(queue)
                            .and(render::render());
                        } else {
                            model.status = "Failed to parse refresh response".to_string();
                        }
                    }
                    // Sem rede, ou o servidor pedindo para esperar (429). Nos dois casos a
                    // sessão guardada pode estar perfeitamente válida — quem decide é o
                    // prazo. O 429 caía no braço de baixo e deslogava quem não tinha
                    // feito nada: bastava o limite por IP estourar num Wi-Fi cheio.
                    HttpResult::Ok(response) if response.status == 429 => {
                        // A espera da abertura já estourou e mandou pelo caminho sem
                        // rede: não há o que refazer.
                        if model.boot.active && model.boot.session_timed_out {
                            return render::render();
                        }
                        return Command::request_from_shell(KeyValueOperation::Get {
                            key: "session_expires_at".to_string(),
                        })
                        .then_send(Event::OfflineSessionChecked);
                    }
                    HttpResult::Err(_) => {
                        if model.boot.active && model.boot.session_timed_out {
                            return render::render();
                        }
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
                        return self.session_ended(model).and(render::render());
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

                    // Na abertura, a fila de quem é a sessão sai do id guardado no
                    // aparelho: sem rede, o servidor não diz.
                    let owner = if model.boot.active {
                        model.boot.session = Some(boot_line(BootCheck::Session, BootVerdict::Ok, BootDetail::LocalTokenValid, 0));
                        Command::request_from_shell(KeyValueOperation::Get { key: "account_user_id".to_string() })
                            .then_send(Event::QueueOwnerRead)
                    } else {
                        Command::done()
                    };

                    // Offline não há o que buscar: devolve o retrato da última vez que
                    // o servidor respondeu — o XP e a trilha que são desta conta.
                    return Command::request_from_shell(KeyValueOperation::Get {
                        key: "offline_snapshot".to_string(),
                    })
                    .then_send(Event::SnapshotRestored)
                    .and(owner)
                    .and(render::render());
                }

                // Sem prazo guardado, ou prazo vencido: não dá para afirmar que há sessão.
                model.status_key = StatusKey::Silent;
                model.access_token = None;
                model.session_offline = false;
                self.session_ended(model).and(render::render())
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
                    headers: auth_headers(&model.access_token, &model.locale),
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
                    headers: auth_headers(&model.access_token, &model.locale),
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
                                model.content_locale = content_language(&response, &model.locale);
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
                    headers: auth_headers(&model.access_token, &model.locale),
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
                                model.content_locale = content_language(&response, &model.locale);
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
                    headers: base_headers(&model.locale),
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
                    // Pediu código de novo antes do intervalo do servidor
                    // (`otp_resend_too_soon`): trava só o envio, porque o código que já
                    // chegou continua valendo. O limite por IP (`rate_limited`) barra
                    // todas as rotas de conta, e trava tudo. Servidor antigo, sem código,
                    // fica como era: só o envio.
                    HttpResult::Ok(response) if response.status == 429 => {
                        let resend_only = api_code(&response.body).as_deref() != Some("rate_limited");
                        return rate_limited(model, &response, resend_only);
                    }
                    HttpResult::Ok(response) => {
                        model.status_key = status_for_input_error(&response.body).unwrap_or(StatusKey::CodeSentFailed);
                    }
                    HttpResult::Err(_) => {
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
                    headers: base_headers(&model.locale),
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
            Event::Register { email, password, otp, age_confirmed, legal_accepted } => {
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status = "Creating account".to_string();
                model.status_key = StatusKey::CreatingAccount;
                model.account_email = email.clone();

                // Caixa desmarcada manda lista vazia, e o servidor recusa. Quem decide
                // é ele; o Core só não inventa um aceite que a pessoa não deu.
                let acceptances = if legal_accepted {
                    legal_acceptances(&model.legal_versions, &model.locale)
                } else {
                    serde_json::json!([])
                };
                let body = serde_json::json!({
                    "email": email, "password": password, "otp": otp,
                    "age_confirmed": age_confirmed, "country": model.legal_country,
                    "legal_acceptances": acceptances,
                });
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/auth/register".to_string(),
                    headers: base_headers(&model.locale),
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
                    // O prefixo de texto é o servidor antigo, de antes dos códigos.
                    HttpResult::Ok(response) if response.status == 409
                        && (api_code(&response.body).as_deref() == Some("legal_version_outdated")
                            || response.body.starts_with(b"legal_version_outdated")) => {
                        // Saiu versão nova dos documentos enquanto a tela estava
                        // aberta. Busca a vigente; o próximo toque já aceita a certa.
                        model.status_key = StatusKey::AccountFailed;
                        let country = model.legal_country.clone();
                        return self.update(Event::FetchLegalVersions { country }, model).and(render::render());
                    }
                    HttpResult::Ok(response) => {
                        model.status_key = status_for_input_error(&response.body).unwrap_or(StatusKey::AccountFailed);
                    }
                    HttpResult::Err(_) => {
                        model.status_key = StatusKey::AccountFailed;
                    }
                }
                render::render()
            }
            Event::DeleteAccount { password_hash } => {
                if model.access_token.is_none() {
                    // Sem sessão com o servidor (offline ou visitante) não há como
                    // provar a senha; a tela só é alcançável com conta.
                    model.status_key = StatusKey::NoConnection;
                    return render::render();
                }
                model.is_authenticating = true;
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/users/me/delete".to_string(),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: serde_json::json!({ "password": password_hash }).to_string().into_bytes(),
                };
                Command::request_from_shell(request)
                    .then_send(Event::AccountDeleted)
                    .and(render::render())
            }
            Event::AccountDeleted(result) => {
                model.is_authenticating = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        // A conta está desativada no servidor e as sessões, recusadas.
                        // Aqui sai tudo o que ela deixou no aparelho, fila incluída — o
                        // aviso da tela já disse que as respostas não enviadas se perdem.
                        // Sem desfazer: `logout_undo` devolveria uma sessão que o
                        // servidor não aceita mais.
                        model.access_token = None;
                        model.user_id = String::new();
                        model.account_email = String::new();
                        model.pending_events.clear();
                        model.session_expires_at = 0;
                        model.session_offline = false;
                        model.is_guest = false;
                        model.logout_undo = None;
                        model.status_key = StatusKey::Silent;
                        // Até quando entrar ainda recupera a conta: a tela de despedida
                        // mostra a data. Resposta sem o campo não mostra tela nenhuma.
                        #[derive(Deserialize)]
                        struct Deleted { #[serde(default)] purge_after: i64 }
                        model.deletion_purge_after = serde_json::from_slice::<Deleted>(&response.body)
                            .map(|d| d.purge_after)
                            .unwrap_or(0);
                        let delete = |key: &str| {
                            Command::request_from_shell(KeyValueOperation::Delete { key: key.into() }).then_send(|_| Event::Ping)
                        };
                        let queue = queue_key(&model.queue_owner);
                        return delete("refresh_token")
                            .and(delete("account_email"))
                            .and(delete("account_user_id"))
                            .and(delete("session_expires_at"))
                            .and(delete(&queue))
                            .and(delete("offline_snapshot"))
                            // O aparelho para de mandar eventos com o id da conta apagada;
                            // os que já estão no PostHog somem pela retenção de 30 dias.
                            .and(Command::request_from_shell(TelemetryOperation::Reset).then_send(|_| Event::TelemetrySent))
                            .and(self.claim_queue(model, GUEST_QUEUE_OWNER, &[""]))
                            .and(render::render());
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.status_key = StatusKey::WrongCredentials;
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    HttpResult::Err(_) => {
                        model.status_key = StatusKey::NoConnection;
                    }
                    _ => {
                        model.status_key = StatusKey::ServerUnreadable;
                    }
                }
                render::render()
            }
            Event::DismissDeletionNotice => {
                model.deletion_purge_after = 0;
                render::render()
            }
            Event::DismissAccountRestoredNotice => {
                model.account_restored_notice = false;
                render::render()
            }
            Event::SetAnalyticsEnabled(enabled) => {
                model.analytics_disabled = !enabled;
                let store = Command::request_from_shell(KeyValueOperation::Set {
                    key: ANALYTICS_DISABLED_KEY.to_string(),
                    value: if enabled { b"0".to_vec() } else { b"1".to_vec() },
                })
                .then_send(|_| Event::Ping);
                let apply = Command::request_from_shell(TelemetryOperation::SetAnalyticsEnabled { enabled })
                    .then_send(|_| Event::TelemetrySent);
                let identity = if enabled {
                    // Religou com conta: os eventos voltam a ser da conta.
                    if model.user_id.is_empty() {
                        Command::done()
                    } else {
                        Command::request_from_shell(TelemetryOperation::Identify { user_id: model.user_id.clone() })
                            .then_send(|_| Event::TelemetrySent)
                    }
                } else {
                    // Desligou: o que continuar saindo (erros, medições) vai com um
                    // identificador anônimo novo, sem ligação com a conta.
                    Command::request_from_shell(TelemetryOperation::Reset).then_send(|_| Event::TelemetrySent)
                };
                store.and(apply).and(identity).and(render::render())
            }
            Event::RestoreAnalyticsPreference => {
                Command::request_from_shell(KeyValueOperation::Get { key: ANALYTICS_DISABLED_KEY.to_string() })
                    .then_send(Event::AnalyticsPreferenceRestored)
            }
            Event::AnalyticsPreferenceRestored(result) => {
                // O shell já configurou o PostHog com a mesma chave ao abrir; aqui o Core
                // só fica sabendo, para não identificar nem rastrear.
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result {
                    model.analytics_disabled = bytes == b"1";
                }
                render::render()
            }
            Event::FetchLegalVersions { country } => {
                // O país só passa se for um código ISO de duas ou três letras: a loja dá
                // alfa-3 (`BRA`), a região do iPhone dá alfa-2 (`BR`), e o servidor leva
                // os dois a alfa-2. O resto ele recusaria, e o cadastro inteiro travaria
                // por causa de um detalhe.
                let country = country.trim().to_ascii_uppercase();
                model.legal_country = if (2..=3).contains(&country.len()) && country.chars().all(|c| c.is_ascii_uppercase()) {
                    country
                } else {
                    String::new()
                };
                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: format!("/api/v1/legal/current?country={}", model.legal_country),
                    headers: base_headers(&model.locale),
                    body: vec![],
                };
                Command::request_from_shell(request).then_send(Event::LegalVersionsFetched)
            }
            Event::LegalVersionsFetched(result) => {
                #[derive(Deserialize)]
                struct Document { kind: String, version: u32 }
                #[derive(Deserialize)]
                struct Current { documents: Vec<Document>, #[serde(default)] min_age: u32 }

                if let HttpResult::Ok(response) = result {
                    if response.status == 200 {
                        if let Ok(current) = serde_json::from_slice::<Current>(&response.body) {
                            model.legal_versions = current.documents.into_iter().map(|d| (d.kind, d.version)).collect();
                            model.min_age = current.min_age;
                        }
                    }
                }
                // Falhou (sem rede, servidor fora): fica o que havia. A tela segue com
                // o envio do código travado, e abrir o cadastro de novo tenta outra vez.
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
                    headers: base_headers(&model.locale),
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
                            // Trocar a senha também cancela uma exclusão pedida.
                            model.account_restored_notice = account_restored(&response.body);

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
                    HttpResult::Ok(response) => {
                        model.status_key = status_for_input_error(&response.body).unwrap_or(StatusKey::ResetFailed);
                    }
                    HttpResult::Err(_) => {
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
                model.sync_sent_ids = model.pending_events.iter().map(|e| e.id.clone()).collect();
                model.sync_owner = model.queue_owner.clone();

                let payload = SyncPayload {
                    user_id: model.user_id.clone(),
                    events: model.pending_events.clone(),
                };
                
                let body_bytes = serde_json::to_vec(&payload).unwrap_or_default();
                
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/sync".to_string(),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: body_bytes,
                };
                
                Command::request_from_shell(request)
                    .then_send(Event::SyncCompleted)
            }
            Event::SyncCompleted(result) => {
                model.is_syncing = false;
                let queued = model.pending_events.len();
                match result {
                    HttpResult::Ok(response) => {
                        if response.status == 200 {
                            model.status_key = StatusKey::Silent;
                            model.rebase_attempted = false;
                            let sent = std::mem::take(&mut model.sync_sent_ids);
                            let boot = self.settle_boot_sync(model, BootVerdict::Ok, BootDetail::Sent, sent.len());

                            // A fila mudou de dono durante o envio (a sessão caiu, alguém
                            // entrou): os enviados saem também da fila do dono que os
                            // mandou, lida do disco. Sem isto ela os mandava de novo
                            // quando ele voltasse. Na memória eles só estão se o dono
                            // novo adotou essa fila, e o `retain` abaixo cuida disso.
                            let owner = std::mem::take(&mut model.sync_owner);
                            let previous_owner = if owner != model.queue_owner {
                                let sent = sent.clone();
                                Command::request_from_shell(KeyValueOperation::Get { key: queue_key(&owner) })
                                    .then_send(move |result| Event::SyncedQueueRead {
                                        owner: owner.clone(),
                                        sent: sent.clone(),
                                        result,
                                    })
                            } else {
                                Command::done()
                            };

                            // Só sai da fila o que foi no envio. Limpar tudo apagava a
                            // resposta dada enquanto o sync estava no ar, e ela nunca subia.
                            model.pending_events.retain(|e| !sent.contains(&e.id));
                            let boot = boot.and(previous_owner);

                            // O topo que o servidor confirmou vira o ponto de partida do
                            // próximo evento. Se sobrou fila, o topo é o dela: ela já foi
                            // encadeada a partir do último enviado.
                            #[derive(Deserialize)]
                            struct SyncOk { new_top: String }
                            if model.pending_events.is_empty() {
                                if let Ok(ok) = serde_json::from_slice::<SyncOk>(&response.body) {
                                    if !ok.new_top.is_empty() {
                                        model.last_hash = ok.new_top;
                                    }
                                }
                            }

                            // A fila que sobrou desce para o disco, sem os enviados.
                            let flush = Command::request_from_shell(store_queue(model))
                                .then_send(|_| Event::Ping);

                            // Entrou evento durante o envio: ele sobe agora. A saída que
                            // esperava o sync continua esperando.
                            if !model.pending_events.is_empty() {
                                return flush.and(boot).and(self.update(Event::SyncNow, model));
                            }
                            if model.logout_after_sync {
                                model.logout_after_sync = false;
                                // A fila subiu: agora sair é seguro.
                                return flush.and(boot).and(self.update(Event::Logout, model));
                            }
                            return flush.and(boot);
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

                                return Command::request_from_shell(store_queue(model))
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
                        model.logout_after_sync = false;
                        return self
                            .settle_boot_sync(model, BootVerdict::Warn, BootDetail::NoNetwork, queued)
                            .and(render::render());
                    }
                }
                // Falhou: fica. Sair aqui apagaria a fila que não subiu. Na abertura, a
                // linha do sync fecha em aviso: não segura o jogador.
                model.logout_after_sync = false;
                self.settle_boot_sync(model, BootVerdict::Warn, BootDetail::Rejected, 0)
                    .and(render::render())
            }
            Event::QueueSavedForSync(_) => {
                // Análise de uso desligada: acerto e erro de desafio não viram evento.
                if model.analytics_disabled {
                    return self.update(Event::SyncNow, model);
                }
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
                Command::request_from_shell(store_queue(model))
                    .then_send(Event::QueueSavedForSync)
                    .and(render::render())
            }

            // A fila de um dono voltou do disco. Se o dono mudou enquanto ela vinha (a
            // sessão caiu, alguém entrou), a resposta é de outra fila e fica de fora.
            Event::OfflineQueueRestored { owner, result } => {
                if owner != model.queue_owner || model.queue_loaded {
                    return Command::done();
                }
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
                self.next_adoption(model).and(render::render())
            }

            // A fila de outro dono entra no fim da do dono atual, reencadeada a partir
            // do topo dela: o conteúdo é o mesmo, só o elo muda — o rebase do Mini-Git.
            Event::QueueToAdoptRead { from, result } => {
                if model.queue_loaded {
                    return Command::done();
                }
                let adopted = match result {
                    KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } => {
                        serde_json::from_slice::<Vec<GameEvent>>(&bytes).unwrap_or_default()
                    }
                    _ => vec![],
                };
                if adopted.is_empty() {
                    return self.next_adoption(model);
                }
                let adopted = match model.pending_events.last() {
                    Some(top) => GameEvent::rebase(&adopted, &top.current_hash),
                    None => adopted,
                };
                model.pending_events.extend(adopted);
                model.last_hash = model.pending_events[model.pending_events.len() - 1].current_hash.clone();
                credit_queued_answers(model);

                // Grava sob o dono novo antes de apagar a chave velha: fechar o app no
                // meio duplica a fila, nunca a perde.
                Command::request_from_shell(store_queue(model))
                    .then_send(move |_| Event::QueueAdopted { from: from.clone() })
                    .and(render::render())
            }

            Event::SyncedQueueRead { owner, sent, result } => {
                let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result else {
                    return Command::done();
                };
                let mut queue = serde_json::from_slice::<Vec<GameEvent>>(&bytes).unwrap_or_default();
                queue.retain(|e| !sent.contains(&e.id));
                Command::request_from_shell(KeyValueOperation::Set {
                    key: queue_key(&owner),
                    value: serde_json::to_vec(&queue).unwrap_or_default(),
                })
                .then_send(|_| Event::Ping)
            }

            Event::QueueAdopted { from } => {
                Command::request_from_shell(KeyValueOperation::Delete { key: queue_key(&from) })
                    .then_send(|_| Event::Ping)
                    .and(self.next_adoption(model))
            }

            Event::QueueOwnerRead(result) => {
                let owner = match result {
                    KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } => {
                        String::from_utf8(bytes).unwrap_or_default()
                    }
                    _ => String::new(),
                };
                // Aparelho que ainda não guardava o id: a fila segue na chave antiga até
                // o servidor dizer de quem ela é.
                if owner.is_empty() {
                    return self.claim_queue(model, "", &[]);
                }
                if model.user_id.is_empty() {
                    model.user_id = owner.clone();
                }
                self.claim_queue(model, &owner, &[""])
            }

            Event::ResumeEmailRead(result) => {
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result {
                    model.resume_email = String::from_utf8(bytes).unwrap_or_default();
                }
                render::render()
            }

            // A abertura. A splash mostra cada verificação numa linha: a sessão, e depois
            // o envio da fila de quem é a sessão. Junto vem o resto do estado de disco
            // que a abertura sempre trouxe.
            Event::StartBoot => {
                let attempt = model.boot.attempt + 1;
                model.boot = Boot {
                    active: true,
                    session: Some(boot_line(BootCheck::Session, BootVerdict::Running, BootDetail::Checking, 0)),
                    attempt,
                    ..Boot::default()
                };
                // Tentar de novo com o refresh anterior ainda no ar espera a resposta
                // dele, em vez de mandar o mesmo token outra vez.
                let session = if model.refresh_in_flight {
                    Command::done()
                } else {
                    Command::request_from_shell(KeyValueOperation::Get { key: "refresh_token".to_string() })
                        .then_send(Event::TokenRead)
                };
                session
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
                    .and(boot_watchdog(BootCheck::Session, attempt))
                    .and(render::render())
            }

            Event::BootWatchdogElapsed { attempt, .. } if attempt != model.boot.attempt => Command::done(),
            Event::BootWatchdogElapsed { check, .. } => match check {
                // A rede não respondeu a tempo: segue como se não houvesse rede. O
                // refresh continua no ar e, se voltar, ainda vale.
                BootCheck::Session => {
                    if !model.boot.session_running() {
                        return Command::done();
                    }
                    model.boot.session_timed_out = true;
                    Command::request_from_shell(KeyValueOperation::Get {
                        key: "session_expires_at".to_string(),
                    })
                    .then_send(Event::OfflineSessionChecked)
                }
                // O envio demorou: a splash sai e ele termina por trás.
                BootCheck::Sync => {
                    let queued = model.pending_events.len();
                    self.settle_boot_sync(model, BootVerdict::Warn, BootDetail::StillSending, queued)
                }
            },

            Event::RetryBoot => {
                if !model.boot.active || !model.boot.awaiting_offline_choice {
                    return Command::done();
                }
                self.update(Event::StartBoot, model)
            }

            Event::ContinueOffline => {
                if !model.boot.active || !model.boot.awaiting_offline_choice {
                    return Command::done();
                }
                self.finish_boot(model)
            }

            // A semente só preenche o que está vazio. Retrato guardado e resposta do
            // servidor são mais novos, e sobrescrevê-los faria o app regredir de
            // conteúdo a cada abertura.
            Event::BundledTrailLoaded { json } => {
                match serde_json::from_str::<crate::domain::TrailSeed>(&json) {
                    Ok(seed) if seed.version == crate::domain::TRAIL_SEED_VERSION => {
                        model.trail_generated_at = seed.generated_at.clone();
                        let locale = served_locale(&model.locale);
                        let (used, trail) = match seed.locales.get(locale) {
                            Some(t) => (locale.to_string(), t.clone()),
                            None => ("pt-BR".to_string(), seed.locales.get("pt-BR").cloned().unwrap_or_default()),
                        };
                        // Só enche o que está vazio: o que veio do servidor ou do retrato
                        // é mais novo que a semente e manda.
                        let encheu = model.nodes.is_empty() || model.challenges.is_empty();
                        if model.nodes.is_empty() {
                            model.nodes = trail.nodes;
                        }
                        if model.challenges.is_empty() {
                            model.challenges = trail.challenges;
                        }
                        // Só marca quando a semente de fato entrou. Se o retrato já
                        // tinha enchido tudo, o jogador não está vendo conteúdo velho.
                        if encheu {
                            model.trail_from_bundle = true;
                            model.content_locale = used;
                        }
                        // Fica guardada: trocar a língua sem rede busca a trilha aqui.
                        model.bundled_seed = Some(seed);
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
                Command::request_from_shell(store_queue(model))
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
            locale: model.locale.clone(),
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
            legal_versions_ready: legal_versions_complete(&model.legal_versions),
            min_age: if model.min_age == 0 { DEFAULT_MIN_AGE } else { model.min_age },
            deletion_purge_after: model.deletion_purge_after,
            account_restored_notice: model.account_restored_notice,
            analytics_enabled: !model.analytics_disabled,
            boot: boot_view(&model.boot),
            resume_email: model.resume_email.clone(),
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
            id: "ch_t01".into(),
            node_id: "node_1".into(),
            template_type: "SPOT_THE_BUG".into(),
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: "Ache o laço infinito.".into(),
                    code_lines: vec![
                        "int i = 0, total = 0;".into(),
                        "while (i < n) {".into(),
                        "    total += nums[i];".into(),
                        "    if (total >= limit) {".into(),
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
            name: "1. Fundamentos & Notação".into(),
            description: "Descrição".into(),
            row: 0,
            column: 0,
            required_xp: 0,
            prerequisites: vec![],
            topic: String::new(),
            status: Default::default(),
            problems_solved: vec![],
        }];

        let mut cmd = app.update(Event::Logout, &mut model);
        let has_refresh_delete = cmd.effects().any(|e| {
            if let Effect::SecureStore(r) = e {
                matches!(r.operation, KeyValueOperation::Delete { ref key } if key == "refresh_token")
            } else {
                false
            }
        });
        assert!(has_refresh_delete, "esperava apagar o refresh token");

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
            id: "ch_t01".into(),
            node_id: "10000000-0000-0000-0000-000000000001".into(),
            template_type: "SPOT_THE_BUG".into(),
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: "Ache o laço infinito.".into(),
                    code_lines: vec!["while (i < n) {".into(), "    i = i;".into()],
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
                    description: "Ache o laço infinito.".into(),
                    code_lines: vec!["while (i < n) {".into(), "    i = i;".into()],
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
        model.challenges = vec![bug("ch_t01"), bug("ch_t02")];
        model.nodes = vec![crate::domain::SkillNode {
            id: NODE.into(),
            name: "1. Fundamentos & Notação".into(),
            description: "Descrição".into(),
            row: 0,
            column: 0,
            required_xp: 0,
            prerequisites: vec![],
            topic: String::new(),
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
            last.payload_json.contains(r#""challenge_id":"ch_t02""#),
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
            Event::OfflineQueueRestored {
                owner: String::new(),
                result: KeyValueResult::Ok {
                    response: KeyValueResponse::Get {
                        value: crux_kv::Value::Bytes(serde_json::to_vec(&queue).unwrap()),
                    },
                },
            },
            &mut model,
        );

        assert_eq!(model.global_xp, 50);
        assert_eq!(model.bugs_found, 1);
        assert_eq!(model.paid_challenges, vec!["ch_001".to_string()]);
    }

    /// Uma semente mínima, na forma que `just seed-bundle` gera.
    #[cfg(test)]
    fn seed_json() -> String {
        let node = |name: &str| serde_json::json!({
            "id": "10000000-0000-0000-0000-000000000001",
            "name": name,
            "description": "Descrição do nó A.",
            "row": 0, "column": 0, "required_xp": 0, "prerequisites": [], "topic": "adhoc"
        });
        serde_json::json!({
            "version": crate::domain::TRAIL_SEED_VERSION,
            "generated_at": "2026-09-22T00:00:00Z",
            "locales": {
                "pt-BR": { "nodes": [node("1. Fundamentos & Notação")], "challenges": [] },
                "es": { "nodes": [node("1. Fundamentos y Notación")], "challenges": [] }
            }
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
                    description: "Merge Sort sobre n elementos".into(),
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
            name: "7. Tópicos Avançados".into(),
            description: "Descrição".into(),
            row: 4,
            column: 0,
            required_xp: 1100,
            prerequisites: vec![],
            topic: String::new(),
            status: Default::default(),
            problems_solved: vec![],
        }];
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        assert_eq!(model.nodes.len(), 1);
        assert_eq!(
            model.nodes[0].name, "7. Tópicos Avançados",
            "o que veio do servidor manda; a semente não regride o conteúdo"
        );
    }

    /// A semente traz as três línguas: entra a do app, e português quando a do app
    /// não veio no bundle.
    #[test]
    fn test_seed_picks_the_app_language_and_falls_back_to_portuguese() {
        let app = LogNApp::default();

        let mut model = Model::default();
        model.locale = "es-AR".into();
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        assert_eq!(model.nodes[0].name, "1. Fundamentos y Notación");
        assert_eq!(model.content_locale, "es");

        // A semente de teste não tem inglês.
        let mut model = Model::default();
        model.locale = "en-US".into();
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        assert_eq!(model.nodes[0].name, "1. Fundamentos & Notação");
        assert_eq!(
            model.content_locale, "pt-BR",
            "o modelo diz a língua do que tem, não a que pediu"
        );
    }

    /// Um retrato gravado em português antes de o app mudar para espanhol sem rede.
    #[cfg(test)]
    fn snapshot_in(locale: Option<&str>) -> Vec<u8> {
        let mut snap = serde_json::json!({
            "global_xp": 340, "bugs_found": 2, "dry_runs_completed": 1,
            "paid_challenge_ids": ["ch_001"],
            "nodes": [{
                "id": "20000000-0000-0000-0000-000000000002",
                "name": "2. Estruturas Básicas", "description": "Do servidor.",
                "row": 1, "column": -1, "required_xp": 100, "prerequisites": []
            }],
            "challenges": []
        });
        if let Some(l) = locale {
            snap["locale"] = serde_json::json!(l);
        }
        serde_json::to_vec(&snap).unwrap()
    }

    #[cfg(test)]
    fn restore(app: &LogNApp, model: &mut Model, bytes: Vec<u8>) {
        let _ = app.update(
            Event::SnapshotRestored(KeyValueResult::Ok {
                response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) },
            }),
            model,
        );
    }

    /// Retrato em outra língua: o progresso é da conta e vale, o texto não. Ficaria
    /// português na tela de um app em espanhol até a próxima busca com rede.
    #[test]
    fn test_snapshot_in_another_language_keeps_progress_not_text() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.locale = "es".into();
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);

        restore(&app, &mut model, snapshot_in(Some("pt-BR")));
        assert_eq!(model.global_xp, 340);
        assert_eq!(model.paid_challenges, vec!["ch_001".to_string()]);
        assert_eq!(model.nodes[0].name, "1. Fundamentos y Notación", "fica a semente em espanhol");
        assert_eq!(model.content_locale, "es");
        assert!(model.trail_from_bundle);
    }

    /// Retrato gravado antes de existir o campo foi gravado em português, que era a
    /// única língua do conteúdo.
    #[test]
    fn test_snapshot_without_language_counts_as_portuguese() {
        let app = LogNApp::default();

        let mut model = Model::default();
        restore(&app, &mut model, snapshot_in(None));
        assert_eq!(model.nodes[0].name, "2. Estruturas Básicas");
        assert_eq!(model.content_locale, "pt-BR");

        let mut model = Model::default();
        model.locale = "en".into();
        restore(&app, &mut model, snapshot_in(None));
        assert!(model.nodes.is_empty(), "retrato antigo não põe português num app em inglês");
        assert_eq!(model.global_xp, 340);
    }

    /// Trocar a língua com a trilha na tela troca o texto na hora, pela semente, e pede
    /// ao servidor o da língua nova.
    #[test]
    fn test_changing_language_swaps_to_the_seed_and_fetches() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::BundledTrailLoaded { json: seed_json() }, &mut model);
        assert_eq!(model.content_locale, "pt-BR");

        let mut cmd = app.update(Event::SetLocale("es-MX".into()), &mut model);
        assert_eq!(model.nodes[0].name, "1. Fundamentos y Notación");
        assert_eq!(model.content_locale, "es");
        let pediu = cmd.effects().any(|e| matches!(
            e,
            Effect::Http(ref r) if r.operation.url == "/api/v1/nodes"
                && r.operation.headers.iter().any(|h| h.name == "Accept-Language" && h.value.starts_with("es"))
        ));
        assert!(pediu, "e busca os nós na língua nova");

        // Mesma língua em outra região: nada muda, nada se busca.
        let mut cmd = app.update(Event::SetLocale("es-AR".into()), &mut model);
        assert!(cmd.effects().next().is_none());
    }

    /// A língua do conteúdo é a que o servidor diz que mandou, não a que se pediu: quem
    /// decide o que serve é ele, e o Core só registra.
    #[test]
    fn test_content_language_comes_from_the_response() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.locale = "es".into();
        let response = |headers: Vec<crux_http::protocol::HttpHeader>| {
            Event::NodesFetched(HttpResult::Ok(crux_http::protocol::HttpResponse {
                status: 200, headers, body: b"[]".to_vec(),
            }))
        };

        let _ = app.update(response(vec![crux_http::protocol::HttpHeader {
            name: "content-language".into(), value: "pt-BR".into(),
        }]), &mut model);
        assert_eq!(model.content_locale, "pt-BR");

        // Servidor antigo, sem o cabeçalho: vale o que se pediu.
        let _ = app.update(response(vec![]), &mut model);
        assert_eq!(model.content_locale, "es");
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
                "name": "2. Estruturas Básicas", "description": "Do servidor.",
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
            "ch_t05",
            "TAG_THE_PATTERN",
            vec!["Tag A".into(), "Tag B".into(), "Tag C".into(), "Tag D".into()],
            vec!["Tag A".into(), "Tag B".into()],
            "",
        )];
        let _ = app.update(Event::StartMatch { node_id: "10000000-0000-0000-0000-000000000001".into() }, &mut model);
        assert_eq!(app.view(&model).match_view.max_selections, 2, "duas tags, duas marcações");

        let _ = app.update(Event::MatchToggleTag { tag: "Tag B".into() }, &mut model);
        let _ = app.update(Event::MatchToggleTag { tag: "Tag A".into() }, &mut model);
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
            id: "ch_t01".into(),
            node_id: "node_1".into(),
            template_type: "SPOT_THE_BUG".into(),
            origin: String::new(),
            payload: ChallengePayload {
                content: ChallengeContent {
                    title: "Soma de Dois Números".into(),
                    description: "Ache o laço infinito.".into(),
                    code_lines: vec!["while (i < n) {".into(), "    i = i;".into()],
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
        let _ = app.update(Event::OfflineQueueRestored { owner: String::new(), result }, &mut fresh);

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

        let effects = kv_ops(&mut cmd);
        let stored = effects.iter().find_map(|op| match op {
            KeyValueOperation::Set { key, value } if key == "refresh_token" => Some(value.clone()),
            _ => None,
        });
        assert_eq!(stored.as_deref(), Some(&b"refresh_rotacionado"[..]), "esperava gravar o refresh token rotacionado");
        // E o id da conta desce junto: é ele que diz de quem é a fila sem rede.
        assert!(effects.iter().any(|op| matches!(op,
            KeyValueOperation::Set { key, value } if key == "account_user_id"
                && value == b"66b670a2-41d2-4ba2-b863-78735b69ec7c")));

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
        assert!(
            kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "account_email")),
            "Expected SecureStore effect reading the account e-mail"
        );

        let kv_result = KeyValueResult::Ok {
            response: KeyValueResponse::Get { value: crux_kv::Value::Bytes("jogador@example.com".into()) },
        };
        let mut cmd = app.update(Event::AccountEmailRead(kv_result), &mut model);

        assert_eq!(model.account_email, "jogador@example.com");
        assert!(model.pending_retry_event.is_none());

        let http_req = cmd.expect_one_effect();
        if let Effect::Http(r) = http_req {
            assert_eq!(r.operation.url, "/api/v1/challenges");
            assert_eq!(r.operation.headers[2].value, "Bearer new_tok");
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

    /// As operações de cofre que o comando pediu, na ordem.
    fn kv_ops(cmd: &mut Command<Effect, Event>) -> Vec<KeyValueOperation> {
        cmd.effects()
            .filter_map(|e| match e {
                Effect::SecureStore(r) => Some(r.operation.clone()),
                _ => None,
            })
            .collect()
    }

    fn kv_bytes(bytes: Vec<u8>) -> KeyValueResult {
        KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } }
    }

    fn kv_empty() -> KeyValueResult {
        KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::None } }
    }

    fn http(status: u16, body: serde_json::Value) -> HttpResult {
        HttpResult::Ok(crux_http::protocol::HttpResponse {
            status,
            headers: vec![],
            body: serde_json::to_vec(&body).unwrap(),
        })
    }

    const USER_A: &str = "66b670a2-41d2-4ba2-b863-78735b69ec7c";
    const USER_B: &str = "0b7c1e53-9a0a-4d3f-8f5e-2d1c0a9b8e77";

    fn answer(id: &str, previous: &str) -> GameEvent {
        GameEvent::new(
            id.into(), "MATCH_ANSWER".into(),
            format!(r#"{{"is_correct":false,"challenge_id":"{id}"}}"#), 1_700_000_000, previous.into(),
        )
    }

    fn queue_bytes(events: &[GameEvent]) -> Vec<u8> {
        serde_json::to_vec(events).unwrap()
    }

    /// Abre o app com token no cofre e o refresh aceito para `user`.
    fn boot_online(app: &LogNApp, model: &mut Model, user: &str) {
        let _ = app.update(Event::StartBoot, model);
        let _ = app.update(Event::TokenRead(kv_bytes(b"ref_tok".to_vec())), model);
        let _ = app.update(
            Event::RefreshCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "rot", "user_id": user,
                "refresh_expires_at": 1_792_600_000i64,
            }))),
            model,
        );
    }

    /// A abertura com rede: a sessão renova, a fila da conta sobe, a splash sai.
    #[test]
    fn test_boot_online_renews_the_session_and_sends_the_queue() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut cmd = app.update(Event::StartBoot, &mut model);
        let view = app.view(&model);
        assert!(view.boot.in_progress);
        assert_eq!(view.boot.lines, vec![boot_line(BootCheck::Session, BootVerdict::Running, BootDetail::Checking, 0)]);
        assert_eq!(view.boot.progress, 25);
        assert!(
            cmd.effects().any(|e| matches!(e, Effect::Time(_))),
            "a espera pela rede tem teto"
        );

        let _ = app.update(Event::TokenRead(kv_bytes(b"ref_tok".to_vec())), &mut model);
        let mut cmd = app.update(
            Event::RefreshCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "rot", "user_id": USER_A,
            }))),
            &mut model,
        );
        assert_eq!(
            model.boot.session,
            Some(boot_line(BootCheck::Session, BootVerdict::Ok, BootDetail::TokenRenewed, 0))
        );
        let key = format!("offline_events:{USER_A}");
        assert!(
            kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key: k } if *k == key)),
            "lê a fila desta conta"
        );

        let queued = vec![answer("a1", GameEvent::GENESIS), answer("a2", GameEvent::GENESIS)];
        let _ = app.update(
            Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_bytes(queue_bytes(&queued)) },
            &mut model,
        );
        // A chave antiga, sem dono, é adotada: vazia, a fila fica como está.
        let mut cmd = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);
        assert!(
            cmd.effects().any(|e| matches!(e, Effect::Http(ref r) if r.operation.url == "/api/v1/sync")),
            "com a fila na memória, o sync da abertura sai"
        );
        assert_eq!(
            model.boot.sync,
            Some(boot_line(BootCheck::Sync, BootVerdict::Running, BootDetail::Sending, 2))
        );
        assert_eq!(app.view(&model).boot.progress, 75);

        let _ = app.update(
            Event::SyncCompleted(http(200, serde_json::json!({ "new_top": "abc" }))),
            &mut model,
        );
        let view = app.view(&model);
        assert!(!view.boot.in_progress, "a última linha fechou: a splash sai");
        assert_eq!(view.boot.lines[1], boot_line(BootCheck::Sync, BootVerdict::Ok, BootDetail::Sent, 2));
        assert!(view.has_session);
    }

    #[test]
    fn test_boot_with_an_empty_queue_has_nothing_to_send() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_online(&app, &mut model, USER_A);
        let _ = app.update(Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_empty() }, &mut model);
        let mut cmd = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);

        assert!(!cmd.effects().any(|e| matches!(e, Effect::Http(_))), "fila vazia não vai à rede");
        let view = app.view(&model);
        assert!(!view.boot.in_progress);
        assert_eq!(view.boot.lines[1], boot_line(BootCheck::Sync, BootVerdict::Ok, BootDetail::NothingToSend, 0));
    }

    /// Sem token no cofre não há o que conferir: nada de splash, e a fila é a do visitante.
    #[test]
    fn test_boot_without_a_token_goes_straight_to_the_login() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::StartBoot, &mut model);
        let mut cmd = app.update(Event::TokenRead(kv_empty()), &mut model);

        let view = app.view(&model);
        assert!(!view.boot.in_progress);
        assert!(view.boot.lines.is_empty(), "não imprime linha de uma sessão que não existe");
        assert!(!view.has_session);
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "offline_events:guest")));
    }

    /// Sem rede e dentro do prazo, a splash para e deixa escolher.
    #[test]
    fn test_boot_offline_waits_for_retry_or_continue() {
        let app = LogNApp::default();
        let now = 1_790_000_000;
        let mut model = Model::default();
        let _ = app.update(Event::Tick { now }, &mut model);
        let _ = app.update(Event::StartBoot, &mut model);
        let _ = app.update(Event::TokenRead(kv_bytes(b"ref_tok".to_vec())), &mut model);
        let _ = app.update(Event::RefreshCompleted(HttpResult::Err(crux_http::HttpError::Io("offline".into()))), &mut model);

        let mut cmd = app.update(
            Event::OfflineSessionChecked(kv_bytes((now + 3600).to_string().into_bytes())),
            &mut model,
        );
        assert_eq!(
            model.boot.session,
            Some(boot_line(BootCheck::Session, BootVerdict::Ok, BootDetail::LocalTokenValid, 0))
        );
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "account_user_id")));

        let mut cmd = app.update(Event::QueueOwnerRead(kv_bytes(USER_A.as_bytes().to_vec())), &mut model);
        let key = format!("offline_events:{USER_A}");
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key: k } if *k == key)));
        assert_eq!(model.user_id, USER_A, "o id guardado vale enquanto o servidor não fala");

        let queued = vec![answer("a1", GameEvent::GENESIS)];
        let _ = app.update(
            Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_bytes(queue_bytes(&queued)) },
            &mut model,
        );
        let _ = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);

        let view = app.view(&model);
        assert!(view.boot.in_progress);
        assert!(view.boot.awaiting_offline_choice);
        assert_eq!(view.boot.lines[1], boot_line(BootCheck::Sync, BootVerdict::Warn, BootDetail::NoNetwork, 1));

        // Tentar de novo volta a conferir a sessão.
        let mut retry = model.clone();
        let mut cmd = app.update(Event::RetryBoot, &mut retry);
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "refresh_token")));
        assert!(!retry.boot.awaiting_offline_choice);
        assert_eq!(app.view(&retry).boot.lines.len(), 1);

        // Continuar entra com o que está no aparelho.
        let _ = app.update(Event::ContinueOffline, &mut model);
        let view = app.view(&model);
        assert!(!view.boot.in_progress);
        assert!(view.has_session && view.is_offline_session);
        assert_eq!(view.pending_sync_count, 1);
    }

    /// Sessão recusada: login com o e-mail dela, e a fila da conta fica no disco.
    #[test]
    fn test_boot_with_a_refused_session_prefills_the_login() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::StartBoot, &mut model);
        let _ = app.update(Event::TokenRead(kv_bytes(b"ref_tok".to_vec())), &mut model);
        let mut cmd = app.update(Event::RefreshCompleted(http(401, serde_json::json!({ "code": "session_invalid" }))), &mut model);

        let ops = kv_ops(&mut cmd);
        assert!(ops.iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "account_email")));
        assert!(ops.iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "offline_events:guest")));
        assert!(!ops.iter().any(|op| matches!(op, KeyValueOperation::Delete { .. })), "a sessão caiu, mas nada é apagado");
        let view = app.view(&model);
        assert!(!view.boot.in_progress);
        assert!(!view.has_session);
        assert_eq!(view.status, StatusKey::SessionExpired);

        let _ = app.update(Event::ResumeEmailRead(kv_bytes(b"jogador@example.com".to_vec())), &mut model);
        assert_eq!(app.view(&model).resume_email, "jogador@example.com");
    }

    /// A espera estoura, a splash segue sem rede, e a resposta tardia do refresh ainda
    /// grava o token rodado — jogá-la fora deixava no cofre um token revogado, e o reuso
    /// dele na abertura seguinte derruba todas as sessões da conta.
    #[test]
    fn test_boot_watchdog_goes_offline_but_keeps_the_late_token() {
        let app = LogNApp::default();
        let now = 1_790_000_000;
        let mut model = Model::default();
        let _ = app.update(Event::Tick { now }, &mut model);
        let _ = app.update(Event::StartBoot, &mut model);
        let _ = app.update(Event::TokenRead(kv_bytes(b"ref_tok".to_vec())), &mut model);
        assert!(model.refresh_in_flight);

        let mut cmd = app.update(Event::BootWatchdogElapsed { check: BootCheck::Session, attempt: 1 }, &mut model);
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "session_expires_at")));
        let _ = app.update(Event::OfflineSessionChecked(kv_bytes((now + 3600).to_string().into_bytes())), &mut model);
        let _ = app.update(Event::QueueOwnerRead(kv_bytes(USER_A.as_bytes().to_vec())), &mut model);
        let queued = vec![answer("a1", GameEvent::GENESIS)];
        let _ = app.update(
            Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_bytes(queue_bytes(&queued)) },
            &mut model,
        );
        let _ = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);
        assert!(model.boot.awaiting_offline_choice);

        // Tentar de novo com o refresh ainda no ar não manda o mesmo token outra vez.
        let mut cmd = app.update(Event::RetryBoot, &mut model);
        assert!(
            !kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "refresh_token")),
            "um segundo refresh com o mesmo token é reuso"
        );
        // O relógio da primeira tentativa não corta a segunda.
        let _ = app.update(Event::BootWatchdogElapsed { check: BootCheck::Session, attempt: 1 }, &mut model);
        assert!(model.boot.session_running());

        let mut cmd = app.update(
            Event::RefreshCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "rot", "user_id": USER_A,
            }))),
            &mut model,
        );
        let effects: Vec<Effect> = cmd.effects().collect();
        assert!(effects.iter().any(|e| matches!(e,
            Effect::SecureStore(r) if matches!(&r.operation, KeyValueOperation::Set { key, .. } if key == "refresh_token"))));
        assert_eq!(
            model.boot.session,
            Some(boot_line(BootCheck::Session, BootVerdict::Ok, BootDetail::TokenRenewed, 0))
        );
        assert!(!model.session_offline);
        assert!(!model.boot.awaiting_offline_choice, "a rede voltou: a pergunta sai");
        // A fila já era desta conta: a abertura segue direto para o sync.
        assert!(
            effects.iter().any(|e| matches!(e, Effect::Http(r) if r.operation.url == "/api/v1/sync")),
            "a fila sobe"
        );
        assert!(model.boot.sync_running());
    }

    /// A resposta dada com o sync no ar não some quando ele volta: só os enviados saem,
    /// e o que sobrou sobe em seguida, encadeado a partir do último enviado.
    #[test]
    fn test_an_answer_during_the_sync_stays_and_goes_next() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = USER_A.into();
        model.queue_owner = USER_A.into();
        model.queue_loaded = true;
        let first = answer("a1", GameEvent::GENESIS);
        model.pending_events = vec![first.clone()];

        let _ = app.update(Event::SyncNow, &mut model);
        assert!(model.is_syncing);
        // Responde enquanto o envio está no ar.
        let late = answer("a2", &first.current_hash);
        model.last_hash = late.current_hash.clone();
        model.pending_events.push(late.clone());

        let mut cmd = app.update(
            Event::SyncCompleted(http(200, serde_json::json!({ "new_top": first.current_hash }))),
            &mut model,
        );
        assert_eq!(model.pending_events.len(), 1, "a resposta do meio do envio fica");
        assert_eq!(model.pending_events[0].id, "a2");
        assert_eq!(model.last_hash, late.current_hash, "com fila sobrando, o topo é o dela, não o do servidor");
        let effects: Vec<Effect> = cmd.effects().collect();
        let stored = effects.iter().find_map(|e| match e {
            Effect::SecureStore(r) => match &r.operation {
                KeyValueOperation::Set { key, value } if *key == format!("offline_events:{USER_A}") => Some(value.clone()),
                _ => None,
            },
            _ => None,
        }).expect("a fila que sobrou desce para o disco");
        assert_eq!(serde_json::from_slice::<Vec<GameEvent>>(&stored).unwrap().len(), 1);
        let body = effects.iter().find_map(|e| match e {
            Effect::Http(r) if r.operation.url == "/api/v1/sync" => Some(r.operation.body.clone()),
            _ => None,
        }).expect("e sobe em seguida");
        let payload: SyncPayload = serde_json::from_slice(&body).unwrap();
        assert_eq!(payload.events.len(), 1);
        assert_eq!(payload.events[0].previous_hash, first.current_hash);
    }

    /// Se a fila trocou de dono com o sync no ar, os enviados saem da fila do dono que
    /// os mandou, no disco — senão ela os mandava de novo quando ele voltasse.
    #[test]
    fn test_a_sync_that_lands_after_the_owner_changed_cleans_the_right_queue() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = USER_A.into();
        model.queue_owner = USER_A.into();
        model.queue_loaded = true;
        let sent = answer("a1", GameEvent::GENESIS);
        model.pending_events = vec![sent.clone()];
        let _ = app.update(Event::SyncNow, &mut model);

        // A sessão caiu no meio: a fila na memória passa a ser a do visitante.
        let _ = app.update(Event::ContinueAsGuest, &mut model);
        let mut cmd = app.update(
            Event::SyncCompleted(http(200, serde_json::json!({ "new_top": sent.current_hash }))),
            &mut model,
        );
        let key_a = format!("offline_events:{USER_A}");
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if *key == key_a)));

        let later = answer("a2", &sent.current_hash);
        let mut cmd = app.update(
            Event::SyncedQueueRead {
                owner: USER_A.into(),
                sent: vec!["a1".into()],
                result: kv_bytes(queue_bytes(&[sent, later])),
            },
            &mut model,
        );
        let stored = kv_ops(&mut cmd).into_iter().find_map(|op| match op {
            KeyValueOperation::Set { key, value } if key == key_a => Some(value),
            _ => None,
        }).expect("regrava a fila de A");
        let left: Vec<GameEvent> = serde_json::from_slice(&stored).unwrap();
        assert_eq!(left.len(), 1);
        assert_eq!(left[0].id, "a2", "fica só o que não foi enviado");
    }

    /// Sem rede e sem fila não há o que esperar da rede: entra direto, sem perguntar.
    #[test]
    fn test_boot_offline_with_an_empty_queue_goes_straight_in() {
        let app = LogNApp::default();
        let now = 1_790_000_000;
        let mut model = Model::default();
        let _ = app.update(Event::Tick { now }, &mut model);
        let _ = app.update(Event::StartBoot, &mut model);
        let _ = app.update(Event::TokenRead(kv_bytes(b"ref_tok".to_vec())), &mut model);
        let _ = app.update(Event::RefreshCompleted(HttpResult::Err(crux_http::HttpError::Io("offline".into()))), &mut model);
        let _ = app.update(Event::OfflineSessionChecked(kv_bytes((now + 3600).to_string().into_bytes())), &mut model);
        let _ = app.update(Event::QueueOwnerRead(kv_bytes(USER_A.as_bytes().to_vec())), &mut model);
        let _ = app.update(Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_empty() }, &mut model);
        let _ = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);

        let view = app.view(&model);
        assert!(!view.boot.in_progress);
        assert!(!view.boot.awaiting_offline_choice);
        assert!(view.has_session && view.is_offline_session);
    }

    /// Um erro de rede tardio, depois de a espera já ter mandado pelo caminho sem rede,
    /// não roda esse caminho de novo.
    #[test]
    fn test_boot_ignores_a_late_network_error_after_the_watchdog() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::StartBoot, &mut model);
        let _ = app.update(Event::TokenRead(kv_bytes(b"ref_tok".to_vec())), &mut model);
        let _ = app.update(Event::BootWatchdogElapsed { check: BootCheck::Session, attempt: 1 }, &mut model);
        let mut cmd = app.update(Event::RefreshCompleted(HttpResult::Err(crux_http::HttpError::Io("offline".into()))), &mut model);
        assert!(kv_ops(&mut cmd).is_empty());
    }

    /// O sync que demora não prende a splash: ela sai e o envio termina por trás.
    #[test]
    fn test_a_slow_boot_sync_lets_the_player_in() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_online(&app, &mut model, USER_A);
        let queued = vec![answer("a1", GameEvent::GENESIS)];
        let _ = app.update(
            Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_bytes(queue_bytes(&queued)) },
            &mut model,
        );
        let _ = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);
        assert!(model.is_syncing);

        let _ = app.update(Event::BootWatchdogElapsed { check: BootCheck::Sync, attempt: 1 }, &mut model);
        let view = app.view(&model);
        assert!(!view.boot.in_progress);
        assert_eq!(view.boot.lines[1], boot_line(BootCheck::Sync, BootVerdict::Warn, BootDetail::StillSending, 1));
        assert!(model.is_syncing, "o envio segue");
    }

    /// Entrar com outra conta não sobe a fila da anterior.
    ///
    /// A fila era uma chave só e não dizia de quem era: depois de a sessão de A expirar,
    /// B entrava e o sync mandava as partidas de A como se fossem de B.
    #[test]
    fn test_another_account_does_not_inherit_the_previous_queue() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.queue_owner = USER_A.into();
        model.queue_loaded = true;
        model.pending_events = vec![answer("a1", GameEvent::GENESIS)];
        model.last_hash = model.pending_events[0].current_hash.clone();

        let _ = app.update(Event::Login { email: "b@x.com".into(), password_hash: "senha".into() }, &mut model);
        let _ = app.update(
            Event::LoginCompleted(http(200, serde_json::json!({
                "access_token": "acc_b", "refresh_token": "ref_b", "user_id": USER_B,
            }))),
            &mut model,
        );
        let _ = app.update(Event::TokenStored(kv_empty()), &mut model);
        let _ = app.update(Event::AccountEmailStored(kv_empty()), &mut model);
        let mut cmd = app.update(Event::SessionExpiryStored(kv_empty()), &mut model);

        assert!(model.pending_events.is_empty(), "a fila de A saiu da memória antes do progresso de B chegar");
        assert!(model.last_hash.is_empty());
        let ops = kv_ops(&mut cmd);
        let key_b = format!("offline_events:{USER_B}");
        assert!(ops.iter().any(|op| matches!(op, KeyValueOperation::Get { key } if *key == key_b)));
        assert!(ops.iter().any(|op| matches!(op, KeyValueOperation::Set { key, value } if key == "account_user_id" && value == USER_B.as_bytes())));
        assert!(
            !ops.iter().any(|op| matches!(op, KeyValueOperation::Delete { key } if key.contains(USER_A))),
            "a fila de A fica no disco esperando A voltar"
        );
    }

    /// Quem jogou como visitante e entra leva o que jogou: a fila do visitante vai para o
    /// fim da fila da conta, reencadeada, e sobe.
    #[test]
    fn test_the_guest_queue_is_adopted_on_login() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.queue_owner = GUEST_QUEUE_OWNER.into();
        model.queue_loaded = true;
        let guest = vec![answer("g1", GameEvent::GENESIS)];
        model.pending_events = guest.clone();

        let _ = app.update(
            Event::LoginCompleted(http(200, serde_json::json!({
                "access_token": "acc_b", "refresh_token": "ref_b", "user_id": USER_B,
            }))),
            &mut model,
        );
        let _ = app.update(Event::SessionExpiryStored(kv_empty()), &mut model);

        let own = vec![answer("b1", GameEvent::GENESIS)];
        let mut cmd = app.update(
            Event::OfflineQueueRestored { owner: USER_B.into(), result: kv_bytes(queue_bytes(&own)) },
            &mut model,
        );
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "offline_events:guest")));

        let mut cmd = app.update(
            Event::QueueToAdoptRead { from: GUEST_QUEUE_OWNER.into(), result: kv_bytes(queue_bytes(&guest)) },
            &mut model,
        );
        assert_eq!(model.pending_events.len(), 2);
        assert_eq!(model.pending_events[0].id, "b1", "a fila da conta vem primeiro");
        assert_eq!(model.pending_events[1].id, "g1");
        assert_eq!(model.pending_events[1].previous_hash, model.pending_events[0].current_hash, "reencadeada");
        let key_b = format!("offline_events:{USER_B}");
        let stored = kv_ops(&mut cmd).into_iter().find_map(|op| match op {
            KeyValueOperation::Set { key, value } if key == key_b => Some(value),
            _ => None,
        }).expect("grava a fila juntada sob a conta");
        assert_eq!(serde_json::from_slice::<Vec<GameEvent>>(&stored).unwrap().len(), 2);

        // Só depois de gravar apaga a do visitante.
        let mut cmd = app.update(Event::QueueAdopted { from: GUEST_QUEUE_OWNER.into() }, &mut model);
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Delete { key } if key == "offline_events:guest")));

        let mut cmd = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);
        assert!(
            cmd.effects().any(|e| matches!(e, Effect::Http(ref r) if r.operation.url == "/api/v1/sync")),
            "o que o visitante jogou sobe logo"
        );
    }

    /// A fila da chave antiga, sem dono, passa para a conta da sessão na primeira abertura.
    #[test]
    fn test_the_legacy_queue_is_adopted_by_the_session() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_online(&app, &mut model, USER_A);
        let _ = app.update(Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_empty() }, &mut model);
        let legacy = vec![answer("old", GameEvent::GENESIS)];
        let mut cmd = app.update(
            Event::QueueToAdoptRead { from: String::new(), result: kv_bytes(queue_bytes(&legacy)) },
            &mut model,
        );
        let key = format!("offline_events:{USER_A}");
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Set { key: k, .. } if *k == key)));
        let mut cmd = app.update(Event::QueueAdopted { from: String::new() }, &mut model);
        let ops = kv_ops(&mut cmd);
        assert!(ops.iter().any(|op| matches!(op, KeyValueOperation::Delete { key } if key == "offline_events")));
        assert_eq!(model.pending_events.len(), 1);
        assert_eq!(model.boot.sync.as_ref().map(|l| l.verdict.clone()), Some(BootVerdict::Running));
    }

    /// A resposta de uma fila que chega depois de o dono mudar é de outra fila.
    #[test]
    fn test_a_queue_of_a_previous_owner_is_ignored() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.queue_owner = USER_B.into();
        let _ = app.update(
            Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_bytes(queue_bytes(&[answer("a1", GameEvent::GENESIS)])) },
            &mut model,
        );
        assert!(model.pending_events.is_empty());
    }

    /// Sair apaga o id da conta e só a fila dela.
    #[test]
    fn test_logout_clears_the_account_id_and_only_its_queue() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.queue_owner = USER_A.into();
        model.queue_loaded = true;
        model.resume_email = "a@x.com".into();
        let mut cmd = app.update(Event::Logout, &mut model);
        let deleted: Vec<String> = kv_ops(&mut cmd).into_iter().filter_map(|op| match op {
            KeyValueOperation::Delete { key } => Some(key),
            _ => None,
        }).collect();
        assert!(deleted.contains(&"account_user_id".to_string()));
        assert!(deleted.contains(&format!("offline_events:{USER_A}")));
        assert!(!deleted.iter().any(|k| k == "offline_events:guest" || k == "offline_events"));
        assert!(model.resume_email.is_empty(), "quem saiu de propósito não vê o e-mail no login");

        let mut cmd = app.update(
            Event::TokenCleared(KeyValueResult::Ok { response: KeyValueResponse::Delete { previous: crux_kv::Value::None } }),
            &mut model,
        );
        assert_eq!(model.queue_owner, GUEST_QUEUE_OWNER, "quem jogar agora joga como visitante");
        assert!(kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "offline_events:guest")));
    }

    /// Desfazer a saída devolve ao aparelho a sessão inteira — a fila, o id, o e-mail e o
    /// prazo —, não só o token. Antes, fechar o app depois do desfazer perdia a fila.
    #[test]
    fn test_undo_logout_writes_the_session_back_to_the_device() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = USER_A.into();
        model.account_email = "jogador@example.com".into();
        model.session_expires_at = 1_792_600_000;
        model.queue_owner = USER_A.into();
        model.queue_loaded = true;
        model.pending_events = vec![answer("a1", GameEvent::GENESIS)];

        let _ = app.update(Event::Logout, &mut model);
        let _ = app.update(
            Event::TokenCleared(KeyValueResult::Ok {
                response: KeyValueResponse::Delete { previous: crux_kv::Value::Bytes(b"ref".to_vec()) },
            }),
            &mut model,
        );
        let mut cmd = app.update(Event::UndoLogout, &mut model);
        let sets: Vec<(String, Vec<u8>)> = kv_ops(&mut cmd).into_iter().filter_map(|op| match op {
            KeyValueOperation::Set { key, value } => Some((key, value)),
            _ => None,
        }).collect();
        let get = |k: &str| sets.iter().find(|(key, _)| key == k).map(|(_, v)| v.clone());

        assert_eq!(get("refresh_token").as_deref(), Some(&b"ref"[..]));
        assert_eq!(get("account_user_id").as_deref(), Some(USER_A.as_bytes()));
        assert_eq!(get("account_email").as_deref(), Some(&b"jogador@example.com"[..]));
        assert_eq!(get("session_expires_at").as_deref(), Some(&b"1792600000"[..]));
        let queue = get(&format!("offline_events:{USER_A}")).expect("a fila volta para o disco");
        assert_eq!(serde_json::from_slice::<Vec<GameEvent>>(&queue).unwrap().len(), 1);
        assert!(get("offline_snapshot").is_some(), "e o retrato, que a saída também apagou");
        assert_eq!(model.queue_owner, USER_A);
        assert_eq!(model.last_hash, model.pending_events[0].current_hash);

        // A fila do visitante que a saída pediu chega depois e não substitui a de A.
        let _ = app.update(Event::OfflineQueueRestored { owner: GUEST_QUEUE_OWNER.into(), result: kv_empty() }, &mut model);
        assert_eq!(model.pending_events.len(), 1);
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
            Event::Register { email: "a@x.com".into(), password: "senha-forte".into(), otp: "123456".into(), age_confirmed: true, legal_accepted: true },
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

    /// Desafios como o servidor manda: enunciado em texto livre, TAG com rótulos e DRY
    /// com watch. O enunciado chegou a ser tipado como chave do catálogo (`TrapKey`), e
    /// nenhuma resposta de verdade desserializava — a lista inteira caía, e os testes
    /// não viam porque as sementes de teste vinham sem desafio nenhum.
    #[test]
    fn test_real_challenges_deserialize() {
        let body = r#"[
          {"id":"ch_t05","node_id":"10000000-0000-0000-0000-000000000001","template_type":"TAG_THE_PATTERN","origin":"",
           "payload":{"content":{"title":"Padrão de Solução","options":["Tag A","Tag B","Tag C"],"code_lines":[],
             "description":"Marque os dois rótulos que descrevem a solução deste exercício.",
             "correct_options":["Tag A","Tag B"]},
             "validation":{"type":"TAG_MATCH","explanation":"Os dois rótulos juntos descrevem a solução esperada."}}},
          {"id":"ch_t06","node_id":"20000000-0000-0000-0000-000000000002","template_type":"DRY_RUN","origin":"FARIAS",
           "payload":{"content":{"title":"Dobrando o Valor","code_lines":["int x = 3;","x = x * 2;"],
             "watch_note":"antes da linha 2","description":"Quanto vale x no fim?",
             "watch_variables":[{"name":"x","value":"3"}]},
             "validation":{"type":"OUTPUT_MATCH","explanation":"A segunda linha dobra o três.","trace_cells":1,"expected_string":"6"}}}
        ]"#;
        let challenges: Vec<Challenge> = serde_json::from_str(body).expect("desafio real desserializa");
        assert_eq!(challenges.len(), 2);
        assert_eq!(challenges[0].payload.content.description, "Marque os dois rótulos que descrevem a solução deste exercício.");

        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::ChallengesFetched(HttpResult::Ok(crux_http::protocol::HttpResponse {
            status: 200, headers: vec![], body: body.as_bytes().to_vec(),
        })), &mut model);
        assert_eq!(model.challenges.len(), 2, "o Core aceita a lista que o servidor manda");
    }

    fn api_error(status: u16, code: &str) -> HttpResult {
        HttpResult::Ok(crux_http::protocol::HttpResponse {
            status,
            headers: vec![crux_http::protocol::HttpHeader { name: "Retry-After".into(), value: "30".into() }],
            body: format!(r#"{{"code":"{code}","message":"x"}}"#).into_bytes(),
        })
    }

    /// Cadastro recusado por dado que a pessoa pode corrigir diz qual dado. Antes tudo
    /// virava "não deu para criar a conta".
    #[test]
    fn test_register_errors_say_what_to_fix() {
        let app = LogNApp::default();
        for (status, code, want) in [
            (409, "email_taken", StatusKey::EmailTaken),
            (400, "invalid_email", StatusKey::InvalidEmail),
            (400, "password_too_short", StatusKey::PasswordTooShort),
            (400, "password_too_long", StatusKey::PasswordTooLong),
            (401, "otp_invalid", StatusKey::CodeInvalid),
            (400, "age_not_confirmed", StatusKey::AccountFailed),
            (500, "internal", StatusKey::AccountFailed),
        ] {
            let mut model = Model::default();
            let _ = app.update(Event::RegisterCompleted(api_error(status, code)), &mut model);
            assert_eq!(model.status_key, want, "{code}");
        }

        let mut model = Model::default();
        let _ = app.update(Event::ResetPasswordCompleted(api_error(400, "password_too_long")), &mut model);
        assert_eq!(model.status_key, StatusKey::PasswordTooLong);

        let mut model = Model::default();
        let _ = app.update(Event::OTPRequested(api_error(400, "invalid_email")), &mut model);
        assert_eq!(model.status_key, StatusKey::InvalidEmail);
    }

    /// 429 no envio de código: reenvio cedo demais trava só o reenvio; o limite por IP
    /// trava todas as ações de conta.
    #[test]
    fn test_otp_429_codes_lock_different_things() {
        let app = LogNApp::default();

        let mut model = Model::default();
        let mut cmd = app.update(Event::OTPRequested(api_error(429, "otp_resend_too_soon")), &mut model);
        let clock = resolve_now(&mut cmd, 1_000);
        let _ = app.update(clock, &mut model);
        assert_eq!(app.view(&model).resend_cooldown_seconds, 30, "o reenvio fica travado");
        assert_eq!(app.view(&model).auth_cooldown_seconds, 0, "reenvio cedo demais não trava o login");

        let mut model = Model::default();
        let mut cmd = app.update(Event::OTPRequested(api_error(429, "rate_limited")), &mut model);
        let clock = resolve_now(&mut cmd, 1_000);
        let _ = app.update(clock, &mut model);
        assert_eq!(app.view(&model).auth_cooldown_seconds, 30, "limite por IP trava tudo");
    }

    /// O 409 de termos chega em JSON com código; o de texto é o servidor antigo.
    #[test]
    fn test_legal_outdated_in_json() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(Event::RegisterCompleted(api_error(409, "legal_version_outdated")), &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Http(ref r) if r.operation.url.starts_with("/api/v1/legal/current"))));
    }

    fn legal_current(body: &str) -> HttpResult {
        HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body: body.as_bytes().to_vec() })
    }

    fn register_body(cmd: &mut Command<Effect, Event>) -> serde_json::Value {
        let req = cmd.effects().find_map(|e| match e {
            Effect::Http(r) if r.operation.url == "/api/v1/auth/register" => Some(r.operation.body.clone()),
            _ => None,
        });
        serde_json::from_slice(&req.expect("o cadastro vai para o servidor")).unwrap()
    }

    /// O cadastro aceita a versão vigente dos dois documentos, na língua do app. Mandava
    /// `["terms_v1", "privacy_v1"]` como texto, e o servidor, que espera objeto,
    /// recusava todo cadastro com 400.
    #[test]
    fn test_register_sends_the_current_legal_versions() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut cmd = app.update(Event::FetchLegalVersions { country: "br".into() }, &mut model);
        assert!(matches!(cmd.expect_one_effect(), Effect::Http(ref r) if r.operation.url == "/api/v1/legal/current?country=BR"));
        assert!(!app.view(&model).legal_versions_ready);
        assert_eq!(app.view(&model).min_age, 13, "antes da resposta, a idade padrão");

        let _ = app.update(Event::LegalVersionsFetched(legal_current(
            r#"{"documents":[{"kind":"terms","version":3,"effective_at":"2026-10-01"},{"kind":"privacy","version":2,"effective_at":"2026-09-24"}],"min_age":14}"#,
        )), &mut model);
        assert!(app.view(&model).legal_versions_ready);
        assert_eq!(app.view(&model).min_age, 14);

        let _ = app.update(Event::SetLocale("es-MX".into()), &mut model);
        let mut cmd = app.update(Event::Register {
            email: "a@x.com".into(), password: "senha-forte".into(), otp: "123456".into(),
            age_confirmed: true, legal_accepted: true,
        }, &mut model);
        let body = register_body(&mut cmd);
        assert_eq!(body["age_confirmed"], true);
        assert_eq!(body["country"], "BR");
        assert_eq!(body["legal_acceptances"], serde_json::json!([
            { "kind": "terms", "version": 3, "locale": "es" },
            { "kind": "privacy", "version": 2, "locale": "es" },
        ]));
    }

    /// Caixa desmarcada não vira aceite. O servidor é quem recusa, mas o Core não
    /// inventa um aceite que a pessoa não deu.
    #[test]
    fn test_register_without_the_box_sends_no_acceptance() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.legal_versions = vec![("terms".into(), 1), ("privacy".into(), 1)];

        let mut cmd = app.update(Event::Register {
            email: "a@x.com".into(), password: "senha-forte".into(), otp: "123456".into(),
            age_confirmed: true, legal_accepted: false,
        }, &mut model);
        assert_eq!(register_body(&mut cmd)["legal_acceptances"], serde_json::json!([]));
    }

    #[test]
    fn test_served_locale_falls_back_to_portuguese() {
        assert_eq!(served_locale("pt-BR"), "pt-BR");
        assert_eq!(served_locale("pt_PT"), "pt-BR");
        assert_eq!(served_locale("EN"), "en");
        assert_eq!(served_locale("es_AR"), "es");
        assert_eq!(served_locale("fr"), "pt-BR");
        assert_eq!(served_locale(""), "pt-BR");
    }

    /// Resposta ilegível ou falha de rede não apaga versões que já se sabiam, nem marca
    /// como prontas versões que faltam.
    #[test]
    fn test_legal_versions_survive_a_failed_fetch() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.legal_versions = vec![("terms".into(), 1), ("privacy".into(), 1)];

        let _ = app.update(Event::LegalVersionsFetched(HttpResult::Err(crux_http::HttpError::Io("offline".into()))), &mut model);
        assert!(app.view(&model).legal_versions_ready);

        let _ = app.update(Event::LegalVersionsFetched(legal_current(r#"{"documents":[{"kind":"terms","version":2}],"min_age":13}"#)), &mut model);
        assert!(!app.view(&model).legal_versions_ready, "só os termos: a política ficou sem versão");
    }

    /// País inválido não vai para o servidor: o cadastro seguiria sem ele, com a idade
    /// padrão, em vez de ser recusado por um detalhe.
    #[test]
    fn test_invalid_country_is_dropped() {
        let app = LogNApp::default();
        let mut model = Model::default();
        for bad in ["BRAZ", "1", "", "b r", "B1"] {
            let mut cmd = app.update(Event::FetchLegalVersions { country: bad.into() }, &mut model);
            assert!(matches!(cmd.expect_one_effect(), Effect::Http(ref r) if r.operation.url == "/api/v1/legal/current?country="));
            assert_eq!(model.legal_country, "");
        }
    }

    fn auth_ok(body: &str) -> HttpResult {
        HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body: body.as_bytes().to_vec() })
    }

    /// O login que cancela uma exclusão liga o aviso, e o aviso some ao ser fechado.
    /// Servidor antigo, sem o campo, não liga nada.
    #[test]
    fn test_login_that_restores_the_account_shows_the_notice() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let _ = app.update(Event::LoginCompleted(auth_ok(
            r#"{"access_token":"a","refresh_token":"r","user_id":"u","refresh_expires_at":1,"account_restored":true}"#,
        )), &mut model);
        assert!(app.view(&model).account_restored_notice);
        let _ = app.update(Event::DismissAccountRestoredNotice, &mut model);
        assert!(!app.view(&model).account_restored_notice);

        let _ = app.update(Event::LoginCompleted(auth_ok(r#"{"access_token":"a","refresh_token":"r"}"#)), &mut model);
        assert!(!app.view(&model).account_restored_notice);

        let _ = app.update(Event::ResetPasswordCompleted(auth_ok(
            r#"{"access_token":"a","refresh_token":"r","account_restored":true}"#,
        )), &mut model);
        assert!(app.view(&model).account_restored_notice, "trocar a senha também recupera");
    }

    /// A exclusão aceita guarda até quando a conta pode voltar, e esquece a identidade
    /// na telemetria.
    #[test]
    fn test_account_deleted_keeps_the_purge_date_and_resets_telemetry() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());

        let mut cmd = app.update(Event::AccountDeleted(auth_ok(r#"{"purge_after":1792600000}"#)), &mut model);
        assert_eq!(app.view(&model).deletion_purge_after, 1_792_600_000);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Telemetry(ref r) if matches!(r.operation, TelemetryOperation::Reset))));

        let _ = app.update(Event::DismissDeletionNotice, &mut model);
        assert_eq!(app.view(&model).deletion_purge_after, 0);
    }

    #[test]
    fn test_logout_resets_telemetry() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(Event::Logout, &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Telemetry(ref r) if matches!(r.operation, TelemetryOperation::Reset))));
    }

    /// Desligar "Análise de uso" guarda a escolha, avisa o shell, esquece a identidade
    /// e para de montar os eventos de uso. Religar com conta identifica de novo.
    #[test]
    fn test_analytics_switch() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.user_id = "u1".into();
        assert!(app.view(&model).analytics_enabled, "ligado por padrão");

        let mut cmd = app.update(Event::SetAnalyticsEnabled(false), &mut model);
        let effects: Vec<_> = cmd.effects().collect();
        assert!(effects.iter().any(|e| matches!(e, Effect::SecureStore(r)
            if matches!(&r.operation, KeyValueOperation::Set { key, value } if key == "analytics_disabled" && value == b"1"))));
        assert!(effects.iter().any(|e| matches!(e, Effect::Telemetry(r)
            if matches!(r.operation, TelemetryOperation::SetAnalyticsEnabled { enabled: false }))));
        assert!(effects.iter().any(|e| matches!(e, Effect::Telemetry(r) if matches!(r.operation, TelemetryOperation::Reset))));
        assert!(!app.view(&model).analytics_enabled);

        // Com ela desligada, acerto de desafio não vira evento, e entrar não identifica.
        let mut cmd = app.update(Event::QueueSavedForSync(KeyValueResult::Ok {
            response: KeyValueResponse::Set { previous: crux_kv::Value::None },
        }), &mut model);
        assert!(!cmd.effects().any(|e| matches!(e, Effect::Telemetry(_))));
        let mut cmd = app.update(Event::SessionExpiryStored(KeyValueResult::Ok {
            response: KeyValueResponse::Set { previous: crux_kv::Value::None },
        }), &mut model);
        assert!(!cmd.effects().any(|e| matches!(e, Effect::Telemetry(_))));

        let mut cmd = app.update(Event::SetAnalyticsEnabled(true), &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Telemetry(r)
            if matches!(&r.operation, TelemetryOperation::Identify { user_id } if user_id == "u1"))));
        assert!(app.view(&model).analytics_enabled);
    }

    #[test]
    fn test_analytics_preference_is_restored() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(Event::RestoreAnalyticsPreference, &mut model);
        assert!(matches!(cmd.expect_one_effect(), Effect::SecureStore(r)
            if matches!(&r.operation, KeyValueOperation::Get { key } if key == "analytics_disabled")));
        let _ = app.update(Event::AnalyticsPreferenceRestored(KeyValueResult::Ok {
            response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(b"1".to_vec()) },
        }), &mut model);
        assert!(!app.view(&model).analytics_enabled);
    }

    /// Excluir a conta manda a senha e, aceito, apaga do aparelho tudo o que a conta
    /// deixou — sem desfazer, porque o servidor já recusa aquela sessão.
    #[test]
    fn test_delete_account_clears_the_device_without_undo() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = "66b670a2-41d2-4ba2-b863-78735b69ec7c".into();
        model.account_email = "a@x.com".into();
        model.queue_owner = model.user_id.clone();
        model.queue_loaded = true;
        model.pending_events = vec![GameEvent::new(
            "evt_1".into(), "MATCH_ANSWER".into(), "{}".into(), 1_700_000_000,
            "0000000000000000000000000000000000000000000000000000000000000000".into(),
        )];

        let mut cmd = app.update(Event::DeleteAccount { password_hash: "senha-forte".into() }, &mut model);
        let body = cmd.effects().find_map(|e| match e {
            Effect::Http(r) if r.operation.url == "/api/v1/users/me/delete" => Some(r.operation.body.clone()),
            _ => None,
        }).expect("pede a exclusão ao servidor");
        assert_eq!(serde_json::from_slice::<serde_json::Value>(&body).unwrap()["password"], "senha-forte");

        let mut cmd = app.update(Event::AccountDeleted(HttpResult::Ok(crux_http::protocol::HttpResponse {
            status: 200, headers: vec![], body: vec![],
        })), &mut model);
        let deleted: Vec<String> = cmd.effects().filter_map(|e| match e {
            Effect::SecureStore(r) => match &r.operation {
                KeyValueOperation::Delete { key } => Some(key.clone()),
                _ => None,
            },
            _ => None,
        }).collect();
        for key in [
            "refresh_token", "account_email", "account_user_id", "session_expires_at",
            "offline_events:66b670a2-41d2-4ba2-b863-78735b69ec7c", "offline_snapshot",
        ] {
            assert!(deleted.contains(&key.to_string()), "faltou apagar {key}");
        }
        assert!(model.access_token.is_none());
        assert!(model.pending_events.is_empty());
        assert!(model.logout_undo.is_none(), "sem desfazer");
        assert!(!app.view(&model).has_session);
    }

    #[test]
    fn test_delete_account_with_wrong_password_keeps_the_session() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());

        let _ = app.update(Event::AccountDeleted(HttpResult::Ok(crux_http::protocol::HttpResponse {
            status: 401, headers: vec![], body: vec![],
        })), &mut model);
        assert_eq!(model.status_key, StatusKey::WrongCredentials);
        assert!(model.access_token.is_some());
    }

    /// Versão nova publicada com a tela aberta: o servidor recusa com 409, e o Core
    /// busca a vigente para o próximo toque aceitar a certa.
    #[test]
    fn test_outdated_legal_version_refetches_current() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.is_authenticating = true;
        model.legal_country = "BR".into();

        let mut cmd = app.update(Event::RegisterCompleted(HttpResult::Ok(crux_http::protocol::HttpResponse {
            status: 409, headers: vec![], body: b"legal_version_outdated\n".to_vec(),
        })), &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Http(ref r) if r.operation.url == "/api/v1/legal/current?country=BR")),
            "busca de novo, com o mesmo país");
        assert_eq!(model.status_key, StatusKey::AccountFailed);
        assert!(!model.is_authenticating);
    }
}
