use crux_core::{render::{self, RenderOperation}, App, macros::effect, Command};
use serde::{Deserialize, Serialize};
use facet::Facet;
use facet_generate_attrs as fg;
use crux_http::protocol::{HttpRequest, HttpResult};
use crux_kv::{KeyValueOperation, KeyValueResult, KeyValueResponse};
use crux_time::{Time, TimeRequest};
use sha2::{Digest, Sha256};
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
    /// A senha viaja como a pessoa digitou, sobre TLS; quem roda o Argon2 é o servidor.
    Login { email: String, password: String },
    LoginCompleted(HttpResult),
    /// O shell fez o login no provedor e entrega o ID token e o nonce cru que gerou
    /// (ADR 0016). O pedido ao provedor levou o SHA-256 do nonce; o servidor confere.
    SocialLogin { provider: String, id_token: String, nonce: String },
    SocialLoginCompleted(HttpResult),
    /// Na tela de idade e termos do primeiro login pelo provedor. Mesmo sentido de
    /// `legal_accepted` do `Register`.
    CompleteSocialSignup { age_confirmed: bool, legal_accepted: bool },
    /// Fechou a tela de idade e termos sem aceitar: o login pelo provedor acaba.
    CancelSocialSignup,
    /// O login no provedor falhou no aparelho, antes de chegar ao servidor (rede, troca
    /// do código). Cancelar pelo próprio jogador não manda isto.
    SocialLoginFailed,
    /// O shell fez o login no GitHub e entrega o código, o verifier do PKCE e o nonce
    /// cru que gerou (ADR 0019). O GitHub não emite ID token: o Core troca o código pelo
    /// bilhete no servidor e segue como `SocialLogin`.
    GitHubCodeReceived { code: String, code_verifier: String, nonce: String },
    GitHubExchanged(HttpResult),
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
    /// O shell dá o `identifierForVendor` na abertura. Vai no `X-Device-ID` da licença.
    SetDeviceId(String),
    /// O catálogo de trilhas pagas.
    FetchTracks,
    TracksFetched(HttpResult),
    /// A loja entregou uma transação verificada. O Core manda ao servidor, e só depois
    /// de ele confirmar o shell pode finalizar a transação na loja (spec, seção 5).
    /// `restore` é o "Restaurar compras": a transação já foi finalizada antes.
    /// `product_id` diz de que trilha é a compra, para a tela do passo a passo (F3).
    ///
    /// `provider` é a loja (ADR 0022): vazio é a App Store, que prova a compra pelo `jws`;
    /// `google_play` prova pelo `purchase_token`, e o `transaction_id` é o próprio token.
    SubmitPurchase {
        jws: String,
        transaction_id: String,
        product_id: String,
        restore: bool,
        provider: String,
        purchase_token: String,
    },
    PurchaseSubmitted {
        jws: String,
        transaction_id: String,
        product_id: String,
        restore: bool,
        provider: String,
        purchase_token: String,
        result: HttpResult,
    },
    /// O shell finalizou a transação na loja.
    PurchaseFinished { transaction_id: String },
    /// Fechou a tela do passo a passo da compra. Antes de pronta, a compra segue em
    /// segundo plano.
    ClosePurchaseFlow,
    /// O jogador tocou em comprar este produto: a compra dele abre o passo a passo.
    PurchaseIntent { product_id: String },
    /// "Restaurar compras" começou com `count` transações do Apple ID.
    RestoreStarted { count: u32 },
    DismissRestoreResult,
    /// A árvore passa a mostrar esta trilha.
    SelectTrack { track_id: String },
    /// "Agora não" na oferta do fim da amostra: ela não volta nesta sessão.
    DismissSampleOffer { track_id: String },
    /// Abriu o catálogo: o selo de "N dias" do botão Trilhas some até amanhã.
    CatalogOpened,
    /// Fuso do aparelho, em segundos. O "hoje" do selo é o do jogador, não o UTC.
    SetUtcOffset { seconds: i32 },
    /// Onboarding fechado ("Por onde começar?"). Não volta mais neste aparelho.
    CompleteOnboarding,
    /// Abertura: lê a trilha escolhida, o onboarding e o dia do selo.
    RestorePreferences,
    PreferenceRestored { key: String, result: KeyValueResult },
    /// Pede a licença da trilha: na compra, ao baixar e a cada abertura com rede.
    FetchLicense { track_id: String },
    /// `user_id` é de quem pediu: a resposta que chega depois de trocar de conta não
    /// entra na conta nova.
    LicenseFetched { user_id: String, track_id: String, result: HttpResult },
    FetchPackage { track_id: String },
    PackageFetched { user_id: String, track_id: String, result: HttpResult },
    /// A maior hora que este aparelho já viu, guardada com as licenças.
    ClockRead { user_id: String, result: KeyValueResult },
    /// Lê do aparelho as trilhas que a conta baixou, e revalida com o servidor.
    LoadTrackDownloads,
    TrackIndexRead { user_id: String, result: KeyValueResult },
    LicenseRead { user_id: String, track_id: String, result: KeyValueResult },
    PackageRead { user_id: String, track_id: String, result: KeyValueResult },
    /// Apaga do aparelho a chave e o pacote da trilha. A compra continua na conta.
    DeleteTrackDownload { track_id: String },

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
    /// TRADEOFF_MATCH: a opção tocada vai para a primeira casa vazia.
    MatchPickTradeoff { value: String },
    MatchClearBenefit,
    MatchClearDrawback,
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
    /// Pede a exclusão da conta, com a senha. Como no `Login`, viaja a senha em si,
    /// sobre TLS, e o servidor confere com Argon2.
    DeleteAccount { password: String },
    /// Pede a exclusão provando que é dono com um login novo no provedor: é o caminho
    /// da conta que não tem senha. `authorization_code` é o da Apple, com que o servidor
    /// revoga o acesso; vazio no Google.
    DeleteAccountWithProvider { provider: String, id_token: String, nonce: String, authorization_code: String },
    /// A exclusão confirmada por um login novo no GitHub (ADR 0019). A troca devolve o
    /// bilhete e o access token, que vai como `authorization_code` para o servidor
    /// revogar a autorização.
    DeleteAccountWithGitHub { code: String, code_verifier: String, nonce: String },
    GitHubDeleteExchanged(HttpResult),
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
    /// Passou o tempo que a entrada pelo login espera pela trilha. `attempt` como no da
    /// abertura.
    EnterWatchdogElapsed { attempt: u32 },
    /// A versão do app e a plataforma, que o aceite dos termos grava. O shell manda na
    /// abertura, junto da língua.
    SetClientInfo { app_version: String, platform: String },
    /// Pergunta ao servidor o que a conta tem para aceitar (ADR 0020). A abertura roda
    /// depois do sync; o login, quando a sessão fica de pé.
    FetchTermsPending,
    /// `owner` é a conta que perguntou: a resposta de uma conta que já saiu não decide o
    /// bloqueio da que entrou depois.
    TermsPendingFetched { owner: String, result: HttpResult },
    /// "Aceitar e continuar" na tela de novo aceite. A caixa é da tela: o botão só
    /// manda isto marcada.
    AcceptTerms,
    TermsAccepted(HttpResult),
    /// O aceite da faixa de mudanças não relevantes, mandado quando ela aparece.
    TermsNoticeAccepted(HttpResult),
    /// Fechou a faixa de mudanças não relevantes.
    DismissTermsNotice,
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
    /// Abriu a aba Placar: mostra o guardado e pede o de agora (docs/specs/logn_placar_spec.md).
    LeaderboardOpened,
    /// Puxou para atualizar.
    LeaderboardRefresh,
    /// `owner` é de quem pediu: a resposta que chega depois de trocar de conta não entra.
    LeaderboardFetched { owner: String, result: HttpResult },
    LeaderboardRestored { owner: String, result: KeyValueResult },
    /// Abriu a folha de escolher apelido.
    NicknameStarted,
    /// "Continuar": confere o formato e vai para a confirmação.
    NicknameChecked(String),
    /// "Voltar e corrigir".
    NicknameBack,
    /// "Confirmar": manda ao servidor. É uma escolha só.
    NicknameSubmitted(String),
    NicknameSaved { owner: String, draft: String, result: HttpResult },
    NicknameFlowClosed,
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
    /// O placar e o nome dele: a saída apaga do aparelho, o desfazer devolve.
    pub leaderboard: Option<crate::leaderboard::LeaderboardCache>,
    pub profile: crate::leaderboard::ProfileIdentity,
}

/// A abertura em andamento: o que a splash mostra.
#[derive(Default, Clone)]
pub struct Boot {
    pub active: bool,
    pub session: Option<crate::domain::BootLine>,
    pub sync: Option<crate::domain::BootLine>,
    /// A terceira linha, depois do sync (ADR 0020).
    pub terms: Option<crate::domain::BootLine>,
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

    fn terms_running(&self) -> bool {
        self.active && matches!(&self.terms, Some(l) if l.verdict == BootVerdict::Running)
    }
}

/// O que a conta tem para aceitar, como `GET /api/v1/legal/pending` disse (ADR 0020).
/// Quem decide se bloqueia é o servidor.
#[derive(Clone, Debug, Default, Deserialize)]
pub struct TermsPending {
    #[serde(default)]
    pub blocking: bool,
    #[serde(default)]
    pub documents: Vec<PendingLegalDoc>,
    #[serde(default)]
    pub changes: Vec<PendingLegalChange>,
}

#[derive(Clone, Debug, Deserialize)]
pub struct PendingLegalDoc {
    pub kind: String,
    pub version: u32,
    pub locale: String,
    #[serde(default)]
    pub effective_at: String,
    pub sha256: String,
    #[serde(default)]
    pub accepted_version: u32,
    #[serde(default)]
    pub accepted_effective_at: String,
}

#[derive(Clone, Debug, Deserialize)]
pub struct PendingLegalChange {
    pub id: String,
    pub kind: String,
    pub version: u32,
    pub change: String,
    pub section: String,
    pub summary: String,
}

/// Abre ou fecha a folha do apelido do zero. O envio que ainda estiver no ar continua
/// valendo: fechar e reabrir a folha não destrava um segundo PUT, e a resposta dele
/// ainda grava o apelido.
fn reset_nickname_flow(model: &mut Model) {
    let in_flight = model.nickname_flow.submitting && model.nickname_flow.owner == model.user_id;
    // O envio que levou 401 e espera a sessão: com a folha fechada, ele não sai mais.
    if matches!(model.pending_retry_event, Some(Event::NicknameSubmitted(_))) {
        model.pending_retry_event = None;
    }
    model.nickname_flow = crate::leaderboard::NicknameFlow {
        owner: model.user_id.clone(),
        submitting: in_flight,
        ..Default::default()
    };
}

/// Pede o placar, marcando de quem é o pedido.
fn fetch_leaderboard(model: &mut Model) -> Command<Effect, Event> {
    if model.access_token.is_none() {
        // Sessão sem rede: nada a pedir, a lista guardada é o que há.
        return Command::done();
    }
    model.leaderboard_in_flight = Some(model.user_id.clone());
    let request = HttpRequest {
        method: "GET".to_string(),
        url: "/api/v1/leaderboard".to_string(),
        headers: auth_headers(&model.access_token, &model.locale),
        body: vec![],
    };
    let owner = model.user_id.clone();
    Command::request_from_shell(request).then_send(move |result| Event::LeaderboardFetched { owner: owner.clone(), result })
}

/// Pergunta pelos termos, marcando de que conta é a pergunta.
fn fetch_terms_pending(model: &Model) -> Command<Effect, Event> {
    let request = HttpRequest {
        method: "GET".to_string(),
        url: "/api/v1/legal/pending".to_string(),
        headers: auth_headers(&model.access_token, &model.locale),
        body: vec![],
    };
    let owner = model.user_id.clone();
    Command::request_from_shell(request).then_send(move |result| Event::TermsPendingFetched { owner: owner.clone(), result })
}

/// O aceite de tudo o que está pendente, com a prova: versão, língua servida, hash do
/// texto, versão de origem e o que a tela mostrou. `source` é `reaccept` (a tela que
/// bloqueia) ou `notice` (a faixa).
fn terms_accept_request(model: &Model, pending: &TermsPending, source: &str) -> HttpRequest {
    let documents: Vec<serde_json::Value> = pending
        .documents
        .iter()
        .map(|d| {
            serde_json::json!({
                "kind": d.kind, "version": d.version, "locale": d.locale,
                "sha256": d.sha256, "from_version": d.accepted_version,
            })
        })
        .collect();
    let shown: Vec<&str> = pending.changes.iter().map(|c| c.id.as_str()).collect();
    HttpRequest {
        method: "POST".to_string(),
        url: "/api/v1/legal/accept".to_string(),
        headers: auth_headers(&model.access_token, &model.locale),
        body: serde_json::json!({
            "documents": documents,
            "shown_changes": shown,
            "client": { "app": model.client_app_version, "platform": model.client_platform },
            "source": source,
        })
        .to_string()
        .into_bytes(),
    }
}

/// A tela de novo aceite, quando há versão relevante e sessão.
fn terms_update_view(model: &Model) -> Option<crate::domain::TermsUpdateViewModel> {
    let pending = model.terms_pending.as_ref()?;
    if !pending.blocking || model.access_token.is_none() {
        return None;
    }
    // Termos e política andam juntos, mas uma conta pode ter aceitado um sem o outro:
    // "de" é o aceite mais antigo, "para" a vigente mais nova.
    let from = pending.documents.iter().min_by_key(|d| d.accepted_version)?;
    let to = pending.documents.iter().max_by_key(|d| d.version)?;
    let mut versions: Vec<u32> = pending.changes.iter().map(|c| c.version).collect();
    versions.sort_unstable();
    versions.dedup();
    let sections = |kind: &str| -> Vec<String> {
        let mut out: Vec<String> = Vec::new();
        for c in pending.changes.iter().filter(|c| c.kind == kind) {
            if !out.contains(&c.section) {
                out.push(c.section.clone());
            }
        }
        out
    };
    Some(crate::domain::TermsUpdateViewModel {
        from_version: from.accepted_version,
        from_date: from.accepted_effective_at.clone(),
        to_version: to.version,
        to_date: to.effective_at.clone(),
        versions_skipped: versions.len().max(1) as u32,
        changes: pending
            .changes
            .iter()
            .map(|c| crate::domain::TermsChange {
                kind: c.kind.clone(),
                version: c.version,
                change: match c.change.as_str() {
                    "added" => crate::domain::TermsChangeKind::Added,
                    "removed" => crate::domain::TermsChangeKind::Removed,
                    _ => crate::domain::TermsChangeKind::Changed,
                },
                section: c.section.clone(),
                summary: c.summary.clone(),
            })
            .collect(),
        terms_sections: sections("terms"),
        privacy_sections: sections("privacy"),
        accepting: model.terms_accepting,
    })
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

/// Quanto o login espera pela trilha antes de abrir o app com o que já tem. Sem teto,
/// uma resposta presa deixava o botão girando para sempre.
const ENTER_WAIT_SECS: u64 = 6;

/// Põe o prazo da entrada para correr. O app abre quando a trilha chegar ou ele acabar.
fn enter_watchdog(model: &mut Model) -> Command<Effect, Event> {
    model.entering_attempt += 1;
    let attempt = model.entering_attempt;
    let (tick, _handle) = Time::notify_after(std::time::Duration::from_secs(ENTER_WAIT_SECS));
    tick.then_send(move |_| Event::EnterWatchdogElapsed { attempt })
}

/// Acaba a entrada, com a trilha que houver. Renderiza só quando de fato estava
/// entrando: nas buscas de fundo de quem já está no app, o render é de quem chamou.
fn finish_entering(model: &mut Model) -> Command<Effect, Event> {
    if !model.entering_session {
        return Command::done();
    }
    model.entering_session = false;
    render::render()
}

/// Quem está entrando agora, e não quem já estava no app. Troca de senha de dentro do
/// app devolve token também, e não pode jogar o jogador de volta ao login. Lido antes
/// de o token novo entrar no modelo.
fn arriving_from_login(model: &Model) -> bool {
    model.access_token.is_none() && !model.session_offline && !model.is_guest
}

/// Dono da fila de quem joga sem conta.
const GUEST_QUEUE_OWNER: &str = "guest";

/// Eventos por pedido de sync. O servidor recusa acima de 500 (`MaxSyncEvents`); o lote
/// fica bem abaixo para caber no teto de corpo com folga.
const SYNC_BATCH: usize = 200;

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
    let lines: Vec<BootLine> = [&boot.session, &boot.sync, &boot.terms].into_iter().flatten().cloned().collect();
    let steps: u32 = lines
        .iter()
        .map(|l| if l.verdict == BootVerdict::Running { 1 } else { 2 })
        .sum();
    crate::domain::BootViewModel {
        in_progress: boot.active,
        progress: (steps * 100 / 6).min(100) as u8,
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
    /// Os cartões de origem na língua da trilha carregada — de `/api/v1/nodes` ou da
    /// semente empacotada. `origins_seen` (abaixo) é quem já foi lido, por id; isto é
    /// o texto de cada id.
    pub origins: Vec<crate::domain::OriginCard>,
    /// O catálogo de trilhas pagas, como o servidor disse. Vai no retrato: não é segredo.
    pub tracks: Vec<crate::domain::Track>,
    /// Licenças da conta neste aparelho, lidas do Keychain. Nunca vão ao retrato.
    pub licenses: std::collections::HashMap<String, crate::domain::TrackLicense>,
    /// Conteúdo fechado já aberto, por trilha: só na memória. O retrato guarda os
    /// desafios em `UserDefaults`, e o que é fechado nunca vai para lá em claro.
    pub track_content: std::collections::HashMap<String, TrackContent>,
    /// Trilhas que a conta baixou neste aparelho.
    pub track_index: Vec<String>,
    /// De quem são as trilhas carregadas do aparelho. Evita reler a cada busca.
    pub track_downloads_owner: String,
    /// XP ganho em cada trilha paga. O portão de um nó conta só o da trilha dele; o da
    /// gratuita é o global menos a soma destes (spec, seção 8).
    pub paid_track_xp: std::collections::HashMap<String, i32>,
    /// Transações que o servidor confirmou e que o shell ainda tem de finalizar na loja.
    pub purchases_to_finish: Vec<String>,
    pub purchase_in_flight: bool,
    /// `identifierForVendor`, dado pelo shell.
    pub device_id: String,
    /// A maior hora já vista pela conta neste aparelho, pelo relógio ou pela emissão de
    /// uma licença. A validade offline conta a partir dela: atrasar o relógio depois
    /// que a licença venceu não a ressuscita.
    pub clock_high_water: i64,
    /// A trilha que a árvore mostra. Vazio é a principal.
    pub selected_track: String,
    pub onboarding_done: bool,
    /// As preferências do aparelho já foram lidas: antes disso o onboarding não aparece.
    pub prefs_loaded: bool,
    /// Fuso do aparelho, em segundos.
    pub utc_offset: i32,
    /// Dia local (dias desde a época) em que o catálogo foi aberto por último.
    pub catalog_seen_day: i64,
    /// Trilhas cuja oferta do fim da amostra o jogador dispensou nesta sessão.
    pub offer_dismissed: Vec<String>,
    /// A compra em andamento na tela do passo a passo: trilha, passo e o porquê de
    /// ter parado.
    pub purchase_flow: Option<(String, crate::domain::PurchaseStage, StatusKey)>,
    /// O produto que o jogador acabou de pedir na loja. Só a compra dele abre a tela do
    /// passo a passo; a transação que a loja reentrega na abertura segue em silêncio.
    pub purchase_intent: String,
    /// Compras que bateram em 401 e esperam o refresh.
    pub purchase_retries: Vec<Event>,
    /// Trilhas cuja licença já levou um 401 e foi pedida de novo depois do refresh. O
    /// segundo 401 é falha: sem isso, um servidor que recusa sempre girava refresh sem fim.
    pub license_retried: Vec<String>,
    /// De quem é o índice de trilhas já lido do aparelho.
    pub track_index_loaded_for: String,
    pub restore: RestoreProgress,
    pub last_hash: String,
    pub user_id: String,
    pub access_token: Option<String>,
    pub is_syncing: bool,
    pub is_fetching: bool,
    pub is_authenticating: bool,
    /// Entrou pela tela de login e a trilha ainda não chegou.
    ///
    /// O app abria com o token, antes do conteúdo, e cada resposta redesenhava a
    /// árvore: a semente com a tarja, a trilha do servidor, e o cabeçalho quando o
    /// catálogo vinha. Enquanto isto vale, o shell continua no login com o botão
    /// esperando, e a árvore aparece uma vez só, pronta.
    pub entering_session: bool,
    /// Tentativa de entrada que pôs o prazo de `ENTER_WAIT_SECS` para correr. O prazo
    /// de uma entrada anterior não corta a de agora.
    pub entering_attempt: u32,
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
    /// De onde o desafio veio, quando não foi escrito para o LogN; vazio é o caso
    /// comum. Aqui vão as origens cujo cartão o jogador já leu, por id (`FARIAS`).
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
    /// O login pelo provedor que espera idade e aceite para virar conta (ADR 0016). O
    /// mesmo token volta ao servidor com eles; some quando o login acaba, dê certo ou
    /// não. Só na memória: é credencial de uso único e vence em uma hora.
    pub social_pending: Option<SocialCredential>,
    /// O servidor respondeu `signup_required` e o app mostra a tela de idade e termos.
    pub social_signup_required: bool,
    /// O nonce cru de um login ou exclusão pelo GitHub, entre a troca do código e o
    /// pedido que usa o bilhete (ADR 0019). Só na memória.
    pub github_nonce: Option<String>,
    /// O que a conta tem para aceitar, quando há versão relevante (ADR 0020). Fica de
    /// pé na saída da conta: o desfazer devolve a sessão, e o bloqueio tem de voltar com
    /// ela. Quem entra em seguida busca de novo.
    pub terms_pending: Option<TermsPending>,
    /// De que conta é o `terms_pending`.
    pub terms_pending_owner: String,
    /// O aceite da tela de termos foi enviado e espera o servidor.
    pub terms_accepting: bool,
    /// Só mudanças não relevantes: a faixa na árvore, uma vez.
    pub terms_notice: bool,
    /// A versão do app e a plataforma, que o aceite grava. Vêm do shell na abertura.
    pub client_app_version: String,
    pub client_platform: String,
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
    /// O placar geral de XP, com o dono. Fica no disco para a tela abrir sem rede.
    pub leaderboard: Option<crate::leaderboard::LeaderboardCache>,
    /// De que conta o placar guardado já foi lido. Outra conta na sessão lê de novo.
    pub leaderboard_disk_read_for: String,
    /// A leitura do disco ainda não voltou.
    pub leaderboard_disk_pending: bool,
    /// O pedido do placar no ar, por dono. Só a resposta esperada entra: a que chega
    /// depois de sair, ou de outra conta, ou de um pedido já superado, fica de fora.
    pub leaderboard_in_flight: Option<String>,
    /// O último pedido do placar falhou (rede, servidor): a lista guardada é velha.
    pub leaderboard_failed: bool,
    /// O nome do placar da conta: número, apelido e se ainda pode escolher.
    pub profile: crate::leaderboard::ProfileIdentity,
    pub nickname_flow: crate::leaderboard::NicknameFlow,
}

/// "Restaurar compras" em andamento: quantas transações o shell mandou e como voltaram.
#[derive(Clone, Debug, Default)]
pub struct RestoreProgress {
    /// Há restauração na tela, mesmo sem nenhuma transação ("nada para restaurar").
    pub started: bool,
    pub total: u32,
    pub done: u32,
    pub other_account: u32,
    pub restored_tracks: Vec<String>,
}

/// O conteúdo fechado de uma trilha, aberto do pacote.
#[derive(Clone, Debug)]
pub struct TrackContent {
    pub content_version: i32,
    pub bytes: u64,
    pub challenges: std::collections::HashMap<String, Vec<Challenge>>,
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
    /// O login pelo provedor ainda não tem conta: o shell mostra idade e termos e
    /// responde com `CompleteSocialSignup` ou `CancelSocialSignup`.
    pub social_signup_required: bool,
    /// Estado do interruptor "Análise de uso".
    pub analytics_enabled: bool,
    /// A splash da abertura.
    pub boot: crate::domain::BootViewModel,
    /// Versão relevante dos termos para aceitar: a tela cobre o app até aceitar ou sair
    /// (ADR 0020).
    pub terms_update: Option<crate::domain::TermsUpdateViewModel>,
    /// Só mudanças não relevantes: a faixa discreta na árvore, até fechar.
    pub terms_notice: bool,
    /// E-mail para o login já vir preenchido depois de uma sessão que acabou. Vazio
    /// quando não há.
    pub resume_email: String,
    /// Id da conta em sessão. O shell põe no `appAccountToken` da compra, e o servidor
    /// confere que quem manda a transação é quem comprou. Vazio no visitante.
    pub account_user_id: String,
    /// O catálogo: a principal primeiro, depois as pagas, com compra e o que está baixado.
    pub tracks: Vec<crate::domain::TrackView>,
    /// A trilha que a árvore mostra. `nodes` já vem filtrado por ela.
    pub current_track: crate::domain::TrackView,
    /// O selo do botão Trilhas.
    pub catalog_badge: crate::domain::CatalogBadge,
    /// Mostrar "Por onde começar?" (uma vez por aparelho).
    pub show_onboarding: bool,
    pub purchase_flow: crate::domain::PurchaseFlowView,
    pub restore_result: crate::domain::RestoreResultView,
    /// A oferta do fim da amostra, quando a partida que acabou foi a amostra.
    pub sample_offer: crate::domain::SampleOfferView,
    /// Transações confirmadas pelo servidor que o shell tem de finalizar na loja e
    /// devolver com `PurchaseFinished`.
    pub purchases_to_finish: Vec<String>,
    pub purchase_in_flight: bool,
    /// O placar geral de XP (docs/specs/logn_placar_spec.md).
    pub leaderboard: crate::domain::LeaderboardView,
    /// O nome do placar: o número de "jogador #N" (0 enquanto o servidor não disse) e o
    /// apelido, quando houver. O Perfil mostra o apelido ou a frase do catálogo.
    pub profile_anon_number: i32,
    pub profile_nickname: Option<String>,
    /// Mostra "Escolher apelido": conta sem apelido e com a chance de pé.
    pub can_choose_nickname: bool,
    pub nickname_flow: crate::domain::NicknameFlowView,
}

#[effect(facet_typegen)]
#[facet(fg::namespace = "LogN")]
pub enum Effect {
    Render(RenderOperation),
    Http(HttpRequest),
    SecureStore(KeyValueOperation),
    Telemetry(crate::domain::TelemetryOperation),
    Monitoring(crate::domain::MonitoringOperation),
    Log(crate::domain::LogOperation),
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
    // O retrato vai para o `UserDefaults`, em claro: o conteúdo fechado da trilha paga
    // fica de fora, e volta do pacote cifrado na abertura.
    let closed = closed_node_ids(model);
    let snapshot = OfflineSnapshot {
        global_xp: model.global_xp,
        bugs_found: model.bugs_found,
        dry_runs_completed: model.dry_runs_completed,
        paid_challenge_ids: model.paid_challenges.clone(),
        nodes: model.nodes.clone(),
        challenges: model.challenges.iter().filter(|c| !closed.contains(&c.node_id)).cloned().collect(),
        paid_track_xp: model.paid_track_xp.clone(),
        tracks: model.tracks.clone(),
        locale: model.content_locale.clone(),
        anon_number: model.profile.anon_number,
        nickname: model.profile.nickname.clone(),
        nickname_locked: model.profile.nickname_locked,
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
    let queued: Vec<(String, String, String)> = model
        .pending_events
        .iter()
        .filter(|e| e.event_type == "MATCH_ANSWER")
        .filter_map(|e| serde_json::from_str::<serde_json::Value>(&e.payload_json).ok())
        .filter(|p| p["is_correct"] == serde_json::Value::Bool(true))
        .filter_map(|p| {
            let id = p["challenge_id"].as_str()?.to_string();
            let template = p["template_type"].as_str().unwrap_or_default().to_string();
            let node_id = p["node_id"].as_str().unwrap_or_default().to_string();
            (!id.is_empty()).then_some((id, template, node_id))
        })
        .collect();

    for (id, template, node_id) in queued {
        if model.paid_challenges.contains(&id) {
            continue;
        }
        model.paid_challenges.push(id);
        model.global_xp += match_engine::XP_PER_ACCEPTED;
        credit_track_xp(&model.nodes, &mut model.paid_track_xp, &node_id);
        match template.as_str() {
            "SPOT_THE_BUG" => model.bugs_found += 1,
            "DRY_RUN" => model.dry_runs_completed += 1,
            _ => {}
        }
    }
}

/// Os nós que só abrem com licença, pelo que o servidor disse de cada um.
fn closed_node_ids(model: &Model) -> std::collections::HashSet<String> {
    model.nodes.iter().filter(|n| n.requires_purchase).map(|n| n.id.clone()).collect()
}

/// A trilha é paga: tem nó que pede compra. O Core não conhece o id da gratuita.
fn is_paid_track(model: &Model, track_id: &str) -> bool {
    !track_id.is_empty() && model.nodes.iter().any(|n| n.track_id == track_id && n.requires_purchase)
}

/// O XP que conta no portão da trilha. A gratuita fica com o global menos o das pagas:
/// o XP de antes das trilhas, que nunca teve trilha, é todo dela.
fn track_xp(model: &Model, track_id: &str) -> i32 {
    if is_paid_track(model, track_id) {
        return model.paid_track_xp.get(track_id).copied().unwrap_or(0);
    }
    (model.global_xp - model.paid_track_xp.values().sum::<i32>()).max(0)
}

/// Soma o XP de um aceito na trilha paga do nó, se for de uma. Recebe os campos, não o
/// modelo: a partida em curso segura um empréstimo dele.
fn credit_track_xp(
    nodes: &[crate::domain::SkillNode],
    paid_track_xp: &mut std::collections::HashMap<String, i32>,
    node_id: &str,
) {
    let Some(track_id) = nodes.iter().find(|n| n.id == node_id).map(|n| n.track_id.clone()) else {
        return;
    };
    if nodes.iter().any(|n| n.track_id == track_id && n.requires_purchase) {
        *paid_track_xp.entry(track_id).or_insert(0) += match_engine::XP_PER_ACCEPTED;
    }
}

/// A hora contra a qual a licença vale: a do relógio, nunca antes da maior já vista.
fn license_now(model: &Model) -> i64 {
    model.now.max(model.clock_high_water)
}

/// A licença da trilha abre agora, sem rede.
fn track_open(model: &Model, track_id: &str) -> bool {
    model
        .licenses
        .get(track_id)
        .is_some_and(|l| crate::tracks::license_valid(l, license_now(model)))
}

/// Sobe a marca d'água do relógio e a guarda, quando a conta tem trilha no aparelho.
fn raise_clock(model: &mut Model, at: i64) -> Command<Effect, Event> {
    if at <= model.clock_high_water {
        return Command::done();
    }
    model.clock_high_water = at;
    if model.user_id.is_empty() || model.track_index.is_empty() {
        return Command::done();
    }
    Command::request_from_shell(KeyValueOperation::Set {
        key: crate::tracks::clock_key(&model.user_id),
        value: at.to_string().into_bytes(),
    })
    .then_send(|_| Event::Ping)
}

/// O nó pode ser jogado: é aberto, ou a trilha dele tem licença válida.
fn node_open(model: &Model, node: &crate::domain::SkillNode) -> bool {
    !node.requires_purchase || track_open(model, &node.track_id)
}

/// Refaz a lista de desafios com o conteúdo fechado que a licença abre agora.
///
/// Os desafios fechados saem todos e voltam só os das trilhas com licença válida, na
/// língua do app. É chamada depois de tudo que mexe na lista, na licença ou no pacote.
fn merge_track_challenges(model: &mut Model) {
    let closed = closed_node_ids(model);
    model.challenges.retain(|c| !closed.contains(&c.node_id));

    let lang = served_locale(&model.locale);
    let mut open: Vec<&String> = model
        .track_content
        .keys()
        .filter(|t| track_open(model, t))
        .collect();
    open.sort();
    let mut extra = Vec::new();
    for track_id in open {
        let content = &model.track_content[track_id];
        let list = content.challenges.get(lang).or_else(|| content.challenges.get("pt-BR"));
        extra.extend(list.into_iter().flatten().cloned());
    }
    model.challenges.extend(extra);
}

/// Tira a trilha do aparelho: licença, pacote e a entrada no índice. Na memória e no
/// disco.
fn forget_track(model: &mut Model, track_id: &str) -> Command<Effect, Event> {
    model.licenses.remove(track_id);
    model.track_content.remove(track_id);
    model.track_index.retain(|t| t != track_id);
    merge_track_challenges(model);
    let user = model.user_id.clone();
    if user.is_empty() {
        return Command::done();
    }
    let delete = |key: String| Command::request_from_shell(KeyValueOperation::Delete { key }).then_send(|_| Event::Ping);
    delete(crate::tracks::license_key(&user, track_id))
        .and(delete(crate::tracks::package_key(&user, track_id)))
        .and(store_track_index(model))
}

fn store_track_index(model: &Model) -> Command<Effect, Event> {
    Command::request_from_shell(KeyValueOperation::Set {
        key: crate::tracks::index_key(&model.user_id),
        value: serde_json::to_vec(&model.track_index).unwrap_or_default(),
    })
    .then_send(|_| Event::Ping)
}

/// A trilha que a árvore mostra: a escolhida, se o app a conhece; senão a principal.
///
/// Sem catálogo (primeira abertura sem rede), a principal é a trilha de um nó que não
/// é de trilha paga — o Core não conhece o id dela de cor.
fn effective_track(model: &Model) -> String {
    let selected = &model.selected_track;
    let known = !selected.is_empty()
        && (model.tracks.iter().any(|t| &t.id == selected) || model.nodes.iter().any(|n| &n.track_id == selected));
    if known {
        return selected.clone();
    }
    if let Some(free) = model.tracks.iter().find(|t| t.kind == "free") {
        return free.id.clone();
    }
    model
        .nodes
        .iter()
        .find(|n| !is_paid_track(model, &n.track_id))
        .map(|n| n.track_id.clone())
        .unwrap_or_default()
}

/// O dia do jogador, em dias desde a época, pelo fuso que o shell deu.
fn local_day(model: &Model) -> i64 {
    (license_now(model) + model.utc_offset as i64).div_euclid(86_400)
}

/// Onde a licença da trilha está na escada de dias sem contato: estado, dias que
/// faltam, dias desde o contato e até quando vale.
fn offline_state(model: &Model, track_id: &str) -> (crate::domain::OfflineState, u32, u32, i64) {
    use crate::domain::OfflineState;
    let Some(license) = model.licenses.get(track_id) else {
        return (OfflineState::NoLicense, 0, 0, 0);
    };
    let now = license_now(model);
    let since = ((now - license.issued_at).max(0) / 86_400) as u32;
    if !crate::tracks::license_valid(license, now) {
        return (OfflineState::Expired, 0, since, license.valid_until);
    }
    let left = ((license.valid_until - now) / 86_400) as u32;
    let state = match left {
        0 => OfflineState::Today,
        1..=3 => OfflineState::Soon,
        _ => OfflineState::Silent,
    };
    (state, left, since, license.valid_until)
}

/// A trilha como a tela a vê. `nodes` são os nós com o estado já calculado.
fn track_view(model: &Model, track: &crate::domain::Track, nodes: &[crate::domain::SkillNode]) -> crate::domain::TrackView {
    let (offline, days_left, since, valid_until) = offline_state(model, &track.id);
    let valid = matches!(
        offline,
        crate::domain::OfflineState::Silent | crate::domain::OfflineState::Soon | crate::domain::OfflineState::Today
    );
    let content = model.track_content.get(&track.id);
    let in_track = |n: &&crate::domain::SkillNode| n.track_id == track.id;
    let open_nodes: Vec<&String> = model.nodes.iter().filter(in_track).filter(|n| !n.requires_purchase).map(|n| &n.id).collect();
    crate::domain::TrackView {
        id: track.id.clone(),
        is_free: track.kind == "free",
        name: track.name.clone(),
        description: track.description.clone(),
        author: track.author.clone(),
        color: track.color.clone(),
        product_id: track.product_id.clone(),
        node_count: track.node_count,
        problem_count: track.problem_count,
        languages: track.languages.clone(),
        selected: effective_track(model) == track.id,
        // O servidor diz no catálogo; uma licença no aparelho diz o mesmo sem rede.
        owned: track.owned || offline != crate::domain::OfflineState::NoLicense,
        revoked: !track.revoked_reason.is_empty(),
        revoked_reason: track.revoked_reason.clone(),
        discontinued: track.status == "discontinued",
        nodes_done: nodes
            .iter()
            .filter(in_track)
            .filter(|n| n.status == crate::domain::NodeStatus::Completed)
            .count() as u32,
        closed_node_count: model.nodes.iter().filter(in_track).filter(|n| n.requires_purchase).count() as u32,
        sample_balloons: model
            .challenges
            .iter()
            .filter(|c| open_nodes.contains(&&c.node_id) && model.paid_challenges.contains(&c.id))
            .count() as u32,
        sample_done: {
            let sample: Vec<&Challenge> = model.challenges.iter().filter(|c| open_nodes.contains(&&c.node_id)).collect();
            !sample.is_empty() && sample.iter().all(|c| model.paid_challenges.contains(&c.id))
        },
        track_xp: track_xp(model, &track.id),
        downloaded: content.is_some() && valid,
        download_bytes: content.map(|c| c.bytes).unwrap_or(0),
        offline,
        offline_days_left: days_left,
        days_since_contact: since,
        valid_until,
        nodes: {
            let mut rows: Vec<&crate::domain::SkillNode> = nodes.iter().filter(in_track).collect();
            rows.sort_by_key(|n| (n.row, n.column));
            rows.into_iter()
                .map(|n| crate::domain::TrackNodeRow {
                    id: n.id.clone(),
                    name: n.name.clone(),
                    free: !n.requires_purchase,
                    done: n.status == crate::domain::NodeStatus::Completed,
                    active: n.status == crate::domain::NodeStatus::Active,
                })
                .collect()
        },
    }
}

/// O selo do botão Trilhas: a comprada que mais pede atenção. O de "N dias" some no
/// dia em que o catálogo foi aberto; "hoje" e "vencida" ficam (LogN Validade Offline).
fn catalog_badge(model: &Model, views: &[crate::domain::TrackView]) -> crate::domain::CatalogBadge {
    use crate::domain::OfflineState;
    let rank = |s: OfflineState| match s {
        OfflineState::Expired => 3,
        OfflineState::Today => 2,
        OfflineState::Soon => 1,
        _ => 0,
    };
    let Some(worst) = views
        .iter()
        .filter(|v| rank(v.offline) > 0)
        .max_by_key(|v| (rank(v.offline), std::cmp::Reverse(v.offline_days_left)))
    else {
        return crate::domain::CatalogBadge::default();
    };
    if worst.offline == OfflineState::Soon && model.catalog_seen_day == local_day(model) {
        return crate::domain::CatalogBadge::default();
    }
    crate::domain::CatalogBadge { state: worst.offline, days_left: worst.offline_days_left }
}

/// A oferta do fim da amostra: a partida que acabou foi a amostra de uma trilha paga,
/// dominada, que a conta não tem, e o jogador não dispensou nesta sessão.
fn sample_offer(model: &Model) -> crate::domain::SampleOfferView {
    let none = crate::domain::SampleOfferView::default();
    let Some(ms) = model.match_state.as_ref() else { return none };
    if ms.is_active {
        return none;
    }
    let Some(node) = model.nodes.iter().find(|n| n.id == model.match_node_id) else { return none };
    let track_id = node.track_id.clone();
    let owned = model.tracks.iter().any(|t| t.id == track_id && t.owned) || model.licenses.contains_key(&track_id);
    if node.requires_purchase
        || !is_paid_track(model, &track_id)
        || owned
        || model.offer_dismissed.contains(&track_id)
        || !node_mastered(&model.challenges, &model.paid_challenges, &node.id)
    {
        return none;
    }

    let mut track_nodes: Vec<&crate::domain::SkillNode> = model.nodes.iter().filter(|n| n.track_id == track_id).collect();
    track_nodes.sort_by_key(|n| (n.row, n.column));
    let next = track_nodes
        .iter()
        .find(|n| n.requires_purchase && n.prerequisites.contains(&node.id))
        .or_else(|| track_nodes.iter().find(|n| n.requires_purchase));
    let Some(next) = next else { return none };
    let open: Vec<&String> = track_nodes.iter().filter(|n| !n.requires_purchase).map(|n| &n.id).collect();
    let sample_problems = model.challenges.iter().filter(|c| open.contains(&&c.node_id)).count() as u32;
    let problem_count = model.tracks.iter().find(|t| t.id == track_id).map(|t| t.problem_count).unwrap_or(0);
    crate::domain::SampleOfferView {
        active: true,
        track_id,
        next_node_name: next.name.clone(),
        next_node_index: track_nodes.iter().position(|n| n.id == next.id).map(|i| i as u32 + 1).unwrap_or(0),
        remaining_nodes: track_nodes.iter().filter(|n| n.requires_purchase).count() as u32,
        remaining_problems: problem_count.saturating_sub(sample_problems),
    }
}

/// Para a compra na tela com o motivo, se ela é desta trilha (vazio: qualquer uma) e
/// ainda não terminou. Sem isto a tela ficava em "Aguarde" para sempre quando a rede
/// caía depois da cobrança.
fn fail_purchase(model: &mut Model, track_id: &str, reason: StatusKey) {
    if let Some(flow) = model.purchase_flow.as_mut() {
        if (track_id.is_empty() || flow.0 == track_id) && flow.1 != crate::domain::PurchaseStage::Ready {
            flow.1 = crate::domain::PurchaseStage::Failed;
            flow.2 = reason;
        }
    }
}

/// Põe o pedido de licença na fila do refresh, uma vez por trilha, e renova a sessão.
/// A fila é a das compras: a restauração pede várias licenças de uma vez.
fn retry_license_after_refresh(app: &LogNApp, model: &mut Model, track_id: String) -> Command<Effect, Event> {
    let queued = model
        .purchase_retries
        .iter()
        .any(|e| matches!(e, Event::FetchLicense { track_id: queued } if *queued == track_id));
    if !queued {
        model.purchase_retries.push(Event::FetchLicense { track_id });
    }
    app.update(Event::AttemptRefresh, model)
}

/// Muda o passo da compra na tela, se ela é desta trilha.
fn advance_purchase(model: &mut Model, track_id: &str, stage: crate::domain::PurchaseStage) {
    if let Some(flow) = model.purchase_flow.as_mut() {
        if flow.0 == track_id {
            flow.1 = stage;
        }
    }
}

/// A trilha do produto da loja, pelo catálogo.
fn track_for_product(model: &Model, product_id: &str) -> String {
    model
        .tracks
        .iter()
        .find(|t| !product_id.is_empty() && t.product_id == product_id)
        .map(|t| t.id.clone())
        .unwrap_or_default()
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
    #[serde(default)]
    paid_track_xp: std::collections::HashMap<String, i32>,
    #[serde(default)]
    tracks: Vec<crate::domain::Track>,
    /// Língua do conteúdo guardado. Vazio é o retrato de antes das línguas: pt-BR.
    #[serde(default)]
    locale: String,
    /// O nome do placar, para o Perfil abrir sem rede. 0 é o retrato de antes dele.
    #[serde(default)]
    anon_number: i32,
    #[serde(default)]
    nickname: Option<String>,
    #[serde(default)]
    nickname_locked: bool,
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

/// A loja do Android, como o servidor a nomeia (`domain.ProviderGooglePlay`).
const PROVIDER_GOOGLE_PLAY: &str = "google_play";

/// Preferências do aparelho que a abertura relê (`RestorePreferences`).
const SELECTED_TRACK_KEY: &str = "selected_track";
const CATALOG_SEEN_KEY: &str = "catalog_seen_day";
const ONBOARDING_KEY: &str = "onboarding_done";

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

/// Resolve `Model::origin_sheet` (o id) para o cartão com o texto, em `Model::origins`.
///
/// Servidor ou semente sem o cartão daquele id — conteúdo atrasado em relação ao app, ou
/// id que mudou — nunca trava a partida: abre um cartão com o próprio id no lugar do
/// nome e o resto vazio, em vez de recusar abrir.
fn resolve_origin_card(model: &Model) -> Option<crate::domain::OriginCard> {
    if model.origin_sheet.is_empty() {
        return None;
    }
    Some(
        model
            .origins
            .iter()
            .find(|o| o.id == model.origin_sheet)
            .cloned()
            .unwrap_or_else(|| crate::domain::OriginCard {
                id: model.origin_sheet.clone(),
                name: model.origin_sheet.clone(),
                role: String::new(),
                body: String::new(),
            }),
    )
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

/// Um registro de log para o shell.
fn log(level: crate::domain::LogLevel, message: &str, attributes: &[(&str, String)]) -> Command<Effect, Event> {
    Command::notify_shell(crate::domain::LogOperation {
        level,
        message: message.to_string(),
        attributes: attributes.iter().map(|(k, v)| (k.to_string(), v.clone())).collect(),
    })
    .build()
}

/// A rota do servidor que cada resposta HTTP responde. Só os eventos que carregam uma
/// resposta estão aqui.
fn http_route(event: &Event) -> Option<(&'static str, &HttpResult)> {
    Some(match event {
        Event::LoginCompleted(r) => ("/api/v1/auth/login", r),
        Event::SocialLoginCompleted(r) => ("/api/v1/auth/social", r),
        Event::GitHubExchanged(r) => ("/api/v1/auth/github/exchange", r),
        Event::GitHubDeleteExchanged(r) => ("/api/v1/auth/github/exchange", r),
        Event::TermsPendingFetched { result, .. } => ("/api/v1/legal/pending", result),
        Event::TermsAccepted(r) => ("/api/v1/legal/accept", r),
        Event::TermsNoticeAccepted(r) => ("/api/v1/legal/accept", r),
        Event::RegisterCompleted(r) => ("/api/v1/auth/register", r),
        Event::RefreshCompleted(r) => ("/api/v1/auth/refresh", r),
        Event::OTPRequested(r) => ("/api/v1/auth/request-otp", r),
        Event::OTPVerified(r) => ("/api/v1/auth/verify-otp", r),
        Event::ResetPasswordCompleted(r) => ("/api/v1/auth/reset-password", r),
        Event::AccountDeleted(r) => ("/api/v1/users/me/delete", r),
        Event::SyncCompleted(r) => ("/api/v1/sync", r),
        Event::NodesFetched(r) => ("/api/v1/nodes", r),
        Event::ChallengesFetched(r) => ("/api/v1/challenges", r),
        Event::ProgressFetched(r) => ("/api/v1/progress", r),
        Event::TracksFetched(r) => ("/api/v1/tracks", r),
        Event::LegalVersionsFetched(r) => ("/api/v1/legal/current", r),
        Event::PurchaseSubmitted { result, .. } => ("/api/v1/purchases", result),
        Event::LicenseFetched { result, .. } => ("/api/v1/tracks/{id}/license", result),
        Event::PackageFetched { result, .. } => ("/api/v1/tracks/{id}/package", result),
        _ => return None,
    })
}

/// O que registrar de uma resposta HTTP. O caminho feliz e os 4xx (erro de quem usa,
/// que a tela já trata) não viram nada. Sem rede é o normal de um app offline-first:
/// só informação. 5xx é defeito do servidor: log de erro e issue. A rota vai com o id
/// no lugar do valor, para não carregar id de conta nem de trilha.
fn observe_http(event: &Event) -> Option<Command<Effect, Event>> {
    use crate::domain::LogLevel;
    let (route, result) = http_route(event)?;
    match result {
        HttpResult::Ok(response) if response.status >= 500 => {
            let status = response.status.to_string();
            let code = api_code(&response.body).unwrap_or_default();
            let message = format!("server error {status} on {route}");
            Some(
                log(LogLevel::Error, &message, &[("route", route.into()), ("status", status.clone()), ("code", code.clone())])
                    .and(Command::notify_shell(crate::domain::MonitoringOperation::LogError {
                        message,
                        details: if code.is_empty() { status } else { format!("{status} {code}") },
                    })
                    .build()),
            )
        }
        HttpResult::Ok(_) => None,
        HttpResult::Err(error) => {
            let (level, kind) = match error {
                crux_http::HttpError::Io(_) => (LogLevel::Info, "io"),
                crux_http::HttpError::Timeout => (LogLevel::Warn, "timeout"),
                crux_http::HttpError::Url(_) => (LogLevel::Error, "url"),
                _ => (LogLevel::Warn, "other"),
            };
            // A frase do erro fica de fora: pode trazer o endereço inteiro.
            Some(log(level, &format!("request did not complete on {route}"), &[("route", route.into()), ("kind", kind.into())]))
        }
    }
}

/// O login pelo provedor esperando a tela de idade e termos (ADR 0016).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SocialCredential {
    pub provider: String,
    pub id_token: String,
    pub nonce: String,
}

/// O app que grava os aceites do cadastro, como no reaceite. Shell que ainda não disse a
/// plataforma manda o client vazio, que o servidor grava como NULL: meio client (versão
/// sem plataforma) ele recusa, e isso travaria o cadastro.
///
/// A versão segue o padrão do servidor (`^[0-9A-Za-z.+-]{1,32}$`); fora dele vai vazia,
/// e só a plataforma é gravada. Um sufixo de build estranho não pode impedir o cadastro.
fn signup_client(model: &Model) -> serde_json::Value {
    if model.client_platform.is_empty() {
        return serde_json::json!({});
    }
    let version = &model.client_app_version;
    let readable = version.len() <= 32 && version.chars().all(|c| c.is_ascii_alphanumeric() || matches!(c, '.' | '+' | '-'));
    let app = if readable { version.as_str() } else { "" };
    serde_json::json!({ "app": app, "platform": model.client_platform })
}

/// O corpo do `POST /api/v1/auth/social`. Sem idade e aceite, é a primeira tentativa;
/// com eles, a criação da conta.
fn social_login_body(model: &Model, cred: &SocialCredential, signup: Option<(bool, bool)>) -> Vec<u8> {
    let mut body = serde_json::json!({
        "provider": cred.provider, "id_token": cred.id_token, "nonce": cred.nonce,
    });
    if let Some((age_confirmed, legal_accepted)) = signup {
        // Caixa desmarcada manda lista vazia, como no cadastro por e-mail: quem recusa
        // é o servidor.
        body["age_confirmed"] = serde_json::json!(age_confirmed);
        body["country"] = serde_json::json!(model.legal_country);
        body["legal_acceptances"] = if legal_accepted {
            legal_acceptances(&model.legal_versions, &model.locale)
        } else {
            serde_json::json!([])
        };
        body["client"] = signup_client(model);
    }
    body.to_string().into_bytes()
}

/// O pedido de `POST /api/v1/auth/github/exchange` (ADR 0019). O nonce vai só como
/// hash; o cru fica no modelo até o pedido que usa o bilhete.
fn github_exchange_request(model: &Model, code: &str, code_verifier: &str, nonce: &str, purpose: &str) -> HttpRequest {
    let nonce_hash = Sha256::digest(nonce.as_bytes()).iter().map(|b| format!("{:02x}", b)).collect::<String>();
    HttpRequest {
        method: "POST".to_string(),
        url: "/api/v1/auth/github/exchange".to_string(),
        headers: base_headers(&model.locale),
        body: serde_json::json!({
            "code": code, "code_verifier": code_verifier, "nonce_hash": nonce_hash, "purpose": purpose,
        })
        .to_string()
        .into_bytes(),
    }
}

/// O que a troca do GitHub devolve. `access_token` só vem na troca de exclusão.
#[derive(Deserialize)]
struct GitHubExchange {
    ticket: String,
    #[serde(default)]
    access_token: String,
}

/// Abre a sessão com a resposta de login do servidor e manda o refresh token para o
/// cofre. `None` quando o corpo não é uma sessão.
fn begin_session(model: &mut Model, body: &[u8]) -> Option<Command<Effect, Event>> {
    #[derive(Deserialize)]
    struct AuthResp {
        access_token: String,
        refresh_token: String,
        #[serde(default)]
        user_id: String,
        #[serde(default)]
        refresh_expires_at: i64,
        #[serde(default)]
        email: String,
    }
    let data = serde_json::from_slice::<AuthResp>(body).ok()?;
    model.entering_session = arriving_from_login(model);
    model.access_token = Some(data.access_token);
    if !data.user_id.is_empty() {
        model.user_id = data.user_id;
    }
    if data.refresh_expires_at > 0 {
        model.session_expires_at = data.refresh_expires_at;
    }
    // No login pelo provedor o app não digitou e-mail: quem diz qual é a conta é o
    // servidor.
    if !data.email.is_empty() {
        model.account_email = data.email;
    }
    model.session_offline = false;
    model.is_guest = false;
    model.status_key = StatusKey::Silent;
    model.account_restored_notice = account_restored(body);
    model.resume_email.clear();
    Some(
        Command::request_from_shell(KeyValueOperation::Set {
            key: "refresh_token".to_string(),
            value: data.refresh_token.into_bytes(),
        })
        .then_send(Event::TokenStored),
    )
}

/// Acaba o login pelo provedor, com ou sem conta: o token não volta a ser usado.
fn end_social_login(model: &mut Model) {
    model.social_pending = None;
    model.social_signup_required = false;
    model.is_authenticating = false;
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

    /// O sync fechou (ou não tinha o que fazer): falta a linha dos termos. Com a sessão de
    /// pé e rede, ela pergunta ao servidor; sem uma das duas, fica para a próxima abertura
    /// (ADR 0020). Sessão que caiu não chega aqui com linha nenhuma: vai para o login.
    fn finish_boot(&self, model: &mut Model) -> Command<Effect, Event> {
        if model.boot.active && model.boot.terms.is_none() && model.boot.session_ok() {
            model.boot.awaiting_offline_choice = false;
            if model.access_token.is_some() {
                model.boot.terms = Some(boot_line(BootCheck::Terms, BootVerdict::Running, BootDetail::TermsChecking, 0));
                return fetch_terms_pending(model)
                    .and(boot_watchdog(BootCheck::Terms, model.boot.attempt))
                    .and(render::render());
            }
            model.boot.terms = Some(boot_line(BootCheck::Terms, BootVerdict::Skipped, BootDetail::TermsDeferred, 0));
        }
        self.close_boot(model)
    }

    /// A última linha fechou: a splash sai. Não há duração mínima.
    fn close_boot(&self, model: &mut Model) -> Command<Effect, Event> {
        model.boot.active = false;
        model.boot.awaiting_offline_choice = false;
        render::render()
    }

    /// Fecha a linha dos termos da abertura, se ela estava rodando.
    fn settle_boot_terms(&self, model: &mut Model, verdict: BootVerdict, detail: BootDetail) -> Command<Effect, Event> {
        if !model.boot.terms_running() {
            return render::render();
        }
        model.boot.terms = Some(boot_line(BootCheck::Terms, verdict, detail, 0));
        self.close_boot(model)
    }

    /// A sessão acabou (o servidor recusou, ou o prazo local venceu). A splash fecha na
    /// linha da sessão, o login vem com o e-mail dela, e a fila na memória passa a ser a
    /// do visitante — a da conta fica no disco, esperando ela voltar.
    /// Pede a licença das trilhas que a conta comprou e que este aparelho não tem.
    fn fetch_missing_licenses(&self, model: &mut Model) -> Command<Effect, Event> {
        let missing: Vec<String> = model
            .tracks
            .iter()
            .filter(|t| t.owned && !model.track_index.contains(&t.id))
            .map(|t| t.id.clone())
            .collect();
        let mut cmd = Command::done();
        for track_id in missing {
            cmd = cmd.and(self.update(Event::FetchLicense { track_id }, model));
        }
        cmd
    }

    fn session_ended(&self, model: &mut Model) -> Command<Effect, Event> {
        // Sem sessão, a compra na tela não anda mais: ela para, e o que esperava o
        // refresh não tem mais o que esperar. A transação fica aberta na loja e volta
        // depois do login.
        fail_purchase(model, "", StatusKey::SessionExpired);
        model.purchase_retries.clear();
        model.license_retried.clear();
        // A sessão caiu no meio da entrada: o login volta a ser tocável já, sem esperar
        // o prazo da entrada.
        model.entering_session = false;
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
        // As respostas HTTP que falharam viram log (e as que não deviam acontecer, erro)
        // aqui, num lugar só, antes de cada handler tratar a sua. Os handlers seguem
        // devolvendo cedo com `return`; a closure é o que deixa o log sair junto.
        let observed = observe_http(&event);
        let handle = || -> Command<Effect, Event> {
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

            Event::Login { email, password } => {
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status = "Logging in".to_string();
                model.status_key = StatusKey::SigningIn;
                model.account_email = email.clone();

                let body = serde_json::json!({
                    "email": email,
                    "password": password
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
                            model.entering_session = arriving_from_login(model);
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
                    // Senha errada demais para o e-mail (`login_locked`): não trava os
                    // botões, porque o login social e a troca de senha pelo código
                    // continuam abertos. O limite por IP (`rate_limited`) trava tudo.
                    HttpResult::Ok(response) if response.status == 429
                        && api_code(&response.body).as_deref() == Some("login_locked") => {
                        model.status_key = StatusKey::LoginLocked;
                        model.is_authenticating = false;
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

            Event::SocialLogin { provider, id_token, nonce } => {
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status = "Logging in with provider".to_string();
                model.status_key = StatusKey::SigningIn;
                model.social_signup_required = false;
                let cred = SocialCredential { provider, id_token, nonce };
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/auth/social".to_string(),
                    headers: base_headers(&model.locale),
                    body: social_login_body(model, &cred, None),
                };
                model.social_pending = Some(cred);
                Command::request_from_shell(request)
                    .then_send(Event::SocialLoginCompleted)
                    .and(render::render())
            }

            Event::CompleteSocialSignup { age_confirmed, legal_accepted } => {
                let Some(cred) = model.social_pending.clone() else {
                    return Command::done();
                };
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                model.is_authenticating = true;
                model.status_key = StatusKey::CreatingAccount;
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/auth/social".to_string(),
                    headers: base_headers(&model.locale),
                    body: social_login_body(model, &cred, Some((age_confirmed, legal_accepted))),
                };
                Command::request_from_shell(request)
                    .then_send(Event::SocialLoginCompleted)
                    .and(render::render())
            }

            Event::CancelSocialSignup => {
                // A tela também fecha sozinha quando o login acaba, e o shell pode mandar
                // isto depois. Sem nada pendente não há o que cancelar, e o erro que
                // fechou a tela continua na linha de status.
                if model.social_pending.is_none() && !model.social_signup_required {
                    return Command::done();
                }
                end_social_login(model);
                model.status_key = StatusKey::Silent;
                render::render()
            }

            Event::SocialLoginFailed => {
                end_social_login(model);
                model.github_nonce = None;
                model.status_key = StatusKey::SocialSignInFailed;
                render::render()
            }

            Event::GitHubCodeReceived { code, code_verifier, nonce } => {
                if model.auth_cooldown.is_active() {
                    return still_rate_limited(model);
                }
                // Uma troca por vez: o nonce da outra seria trocado por este no meio.
                if model.github_nonce.is_some() {
                    return Command::done();
                }
                model.is_authenticating = true;
                model.status_key = StatusKey::SigningIn;
                model.social_signup_required = false;
                let request = github_exchange_request(model, &code, &code_verifier, &nonce, "login");
                model.github_nonce = Some(nonce);
                Command::request_from_shell(request)
                    .then_send(Event::GitHubExchanged)
                    .and(render::render())
            }

            Event::GitHubExchanged(result) => {
                model.is_authenticating = false;
                // Resposta sem troca pendente é de um login que já acabou: não abre nada.
                let Some(nonce) = model.github_nonce.take() else {
                    return Command::done();
                };
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        match serde_json::from_slice::<GitHubExchange>(&response.body) {
                            // Dali em diante é o login pelo provedor de sempre, com o
                            // bilhete no lugar do ID token.
                            Ok(ex) if !ex.ticket.is_empty() => {
                                return self.update(
                                    Event::SocialLogin { provider: "github".into(), id_token: ex.ticket, nonce },
                                    model,
                                );
                            }
                            _ => model.status_key = StatusKey::ServerUnreadable,
                        }
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    HttpResult::Ok(response) => {
                        model.status_key = match api_code(&response.body).as_deref() {
                            Some("provider_disabled") => StatusKey::SocialProviderDisabled,
                            _ => StatusKey::SocialSignInFailed,
                        };
                    }
                    HttpResult::Err(_) => {
                        model.status_key = StatusKey::NoConnection;
                    }
                }
                render::render()
            }

            Event::SocialLoginCompleted(result) => {
                model.is_authenticating = false;
                // Na tela de idade e termos, o erro que a pessoa corrige ali (caixa,
                // versão nova, rede) mantém o login de pé para o próximo toque. Fora
                // dela, todo erro acaba o login.
                let in_signup = model.social_signup_required;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        end_social_login(model);
                        if let Some(cmd) = begin_session(model, &response.body) {
                            return cmd;
                        }
                        model.status_key = StatusKey::ServerUnreadable;
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        if !in_signup {
                            end_social_login(model);
                        }
                        return rate_limited(model, &response, false);
                    }
                    HttpResult::Ok(response) => match api_code(&response.body).as_deref() {
                        // Resposta que chega depois de o login ter sido cancelado não abre
                        // tela: sem token guardado, "Criar conta" não teria o que mandar.
                        Some("signup_required") if model.social_pending.is_some() => {
                            model.social_signup_required = true;
                            model.status_key = StatusKey::Silent;
                        }
                        Some("legal_version_outdated") if in_signup => {
                            model.status_key = StatusKey::AccountFailed;
                            let country = model.legal_country.clone();
                            return self.update(Event::FetchLegalVersions { country }, model).and(render::render());
                        }
                        Some("age_not_confirmed" | "legal_acceptance_required" | "invalid_country") if in_signup => {
                            model.status_key = StatusKey::AccountFailed;
                        }
                        Some("social_email_unverified") => {
                            end_social_login(model);
                            model.status_key = StatusKey::SocialEmailUnverified;
                        }
                        Some("provider_disabled") => {
                            end_social_login(model);
                            model.status_key = StatusKey::SocialProviderDisabled;
                        }
                        Some("social_token_invalid") => {
                            end_social_login(model);
                            model.status_key = StatusKey::SocialSignInFailed;
                        }
                        _ => {
                            end_social_login(model);
                            model.status_key = StatusKey::SignInFailed;
                        }
                    },
                    HttpResult::Err(_) => {
                        if !in_signup {
                            end_social_login(model);
                        }
                        model.status_key = StatusKey::NoConnection;
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
                // O render vai junto, como no `Login`: sem ele o shell só via o visitante
                // quando a árvore chegava do servidor, e até lá o toque parecia morto. Com
                // ele o app já troca de tela, e o mapa mostra que está atualizando.
                self.claim_queue(model, GUEST_QUEUE_OWNER, &[""])
                    .and(self.update(Event::FetchNodes, model))
                    .and(render::render())
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
                // O cofre já tem o token novo (ou recusou, e não há o que esperar).
                model.refresh_in_flight = false;
                // Quem pediu refresh nessa janela foi dispensado em `TokenRead` e não volta
                // a passar por `RefreshCompleted`: o "retomando sessão" dele e a linha da
                // abertura que ele reabriu fecham aqui, como o refresh que já voltou.
                if model.status_key == StatusKey::ResumingSession {
                    model.status_key = StatusKey::Silent;
                }
                if model.boot.active && model.boot.session_running() {
                    model.boot.session = Some(boot_line(BootCheck::Session, BootVerdict::Ok, BootDetail::TokenRenewed, 0));
                    model.boot.sync = None;
                    model.boot.awaiting_offline_choice = false;
                }
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

                // Quem vem do login espera a trilha nesta tela: ela é buscada daqui, e
                // não da árvore quando ela aparece, que era o que a fazia aparecer antes
                // do conteúdo.
                let queue = if model.entering_session {
                    queue.and(enter_watchdog(model)).and(self.update(Event::FetchNodes, model))
                } else {
                    queue
                };

                // Conta antiga entrando num aparelho novo não passa pela splash com
                // sessão: os termos são conferidos aqui (ADR 0020). O bloqueio de quem
                // estava antes neste aparelho não vale para esta conta; o desta mesma conta
                // continua até o servidor dizer outra coisa, para uma busca que falhe não
                // soltar quem saiu da tela de aceite e entrou de novo.
                if model.terms_pending_owner != model.user_id {
                    model.terms_pending = None;
                    model.terms_notice = false;
                }
                let queue = queue.and(self.update(Event::FetchTermsPending, model));

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
                    // O placar guardado traz a linha de quem saiu. A resposta que ainda
                    // estiver no ar não o regrava: ninguém mais a espera.
                    .and({
                        model.leaderboard_in_flight = None;
                        model.nickname_flow = crate::leaderboard::NicknameFlow::default();
                        Command::request_from_shell(KeyValueOperation::Delete { key: crate::leaderboard::LEADERBOARD_KEY.into() }).then_send(|_| Event::Ping)
                    })
                    // A trilha escolhida é de quem saiu.
                    .and(Command::request_from_shell(KeyValueOperation::Delete { key: SELECTED_TRACK_KEY.into() }).then_send(|_| Event::Ping))
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
                    leaderboard: model.leaderboard.take(),
                    profile: std::mem::take(&mut model.profile),
                });
                model.leaderboard_disk_read_for.clear();
                model.leaderboard_disk_pending = false;
                model.leaderboard_in_flight = None;
                model.leaderboard_failed = false;
                model.nickname_flow = crate::leaderboard::NicknameFlow::default();

                model.is_guest = false;
                model.session_offline = false;
                model.session_expires_at = 0;
                model.entering_session = false;
                model.review_prompt_pending = false;
                // A trilha paga é da conta que saiu. O disco fica, sob o id dela; a
                // memória não, senão quem entra depois neste aparelho abriria a trilha.
                model.licenses.clear();
                model.track_content.clear();
                model.track_index.clear();
                model.track_downloads_owner.clear();
                model.paid_track_xp.clear();
                model.clock_high_water = 0;
                // O catálogo fica, porque é o mesmo para todos; o que ele dizia desta
                // conta (comprada, revogada e por quê), não. Nem a trilha escolhida, a
                // oferta dispensada, a compra na tela ou a restauração.
                for track in model.tracks.iter_mut() {
                    track.owned = false;
                    track.revoked_reason.clear();
                }
                model.selected_track.clear();
                model.offer_dismissed.clear();
                model.purchase_flow = None;
                model.purchase_intent.clear();
                model.purchase_retries.clear();
                model.license_retried.clear();
                model.restore = RestoreProgress::default();
                model.track_index_loaded_for.clear();
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
                model.profile = snapshot.profile;
                model.leaderboard = snapshot.leaderboard;
                // O placar volta para o disco, como o resto que a saída apagou.
                let board = match &model.leaderboard {
                    Some(cache) => Command::request_from_shell(KeyValueOperation::Set {
                        key: crate::leaderboard::LEADERBOARD_KEY.to_string(),
                        value: serde_json::to_vec(cache).unwrap_or_default(),
                    })
                    .then_send(|_| Event::Ping),
                    None => Command::done(),
                };
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
                // Os desafios fechados voltaram com o instantâneo; a licença, não. Eles
                // saem, e voltam quando as trilhas desta conta forem relidas do disco.
                merge_track_challenges(model);
                let downloads = self.update(Event::LoadTrackDownloads, model);
                token
                    .and(account)
                    .and(board)
                    .and(Command::request_from_shell(store_queue(model)).then_send(|_| Event::Ping))
                    .and(downloads)
                    .and(render::render())
            }

            Event::LogoutUndone(_) => render::render(),

            Event::LeaderboardOpened => {
                if model.access_token.is_none() && !model.session_offline {
                    return render::render();
                }
                let owner = model.user_id.clone();
                let disk = if model.leaderboard_disk_read_for == owner {
                    Command::done()
                } else {
                    model.leaderboard_disk_read_for = owner.clone();
                    model.leaderboard_disk_pending = true;
                    let o = owner.clone();
                    Command::request_from_shell(KeyValueOperation::Get { key: crate::leaderboard::LEADERBOARD_KEY.to_string() })
                        .then_send(move |result| Event::LeaderboardRestored { owner: o.clone(), result })
                };
                let fetch = if model.leaderboard_in_flight.as_deref() == Some(owner.as_str()) {
                    Command::done()
                } else {
                    fetch_leaderboard(model)
                };
                disk.and(fetch).and(render::render())
            }

            Event::LeaderboardRefresh => {
                // Um pedido por vez: dois no ar podiam voltar fora de ordem.
                if model.leaderboard_in_flight.as_deref() == Some(model.user_id.as_str()) {
                    return render::render();
                }
                fetch_leaderboard(model).and(render::render())
            }

            Event::LeaderboardRestored { owner, result } => {
                if owner != model.user_id {
                    return Command::done();
                }
                model.leaderboard_disk_pending = false;
                let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result else {
                    return render::render();
                };
                match serde_json::from_slice::<crate::leaderboard::LeaderboardCache>(&bytes) {
                    // O guardado é de outra conta (a sessão venceu sem sair): fora.
                    Ok(cache) if cache.owner != model.user_id => {
                        Command::request_from_shell(KeyValueOperation::Delete { key: crate::leaderboard::LEADERBOARD_KEY.into() })
                            .then_send(|_| Event::Ping)
                    }
                    Ok(cache) => {
                        // A rede pode ter chegado antes do disco: fica o mais novo.
                        let newer = model
                            .leaderboard
                            .as_ref()
                            .map_or(true, |current| cache.board.generated_at > current.board.generated_at);
                        if newer {
                            model.leaderboard = Some(cache);
                        }
                        render::render()
                    }
                    Err(_) => render::render(),
                }
            }

            Event::LeaderboardFetched { owner, result } => {
                if model.leaderboard_in_flight.as_ref() != Some(&owner) {
                    return Command::done();
                }
                model.leaderboard_in_flight = None;
                if owner != model.user_id {
                    return Command::done();
                }
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        let Ok(board) = serde_json::from_slice::<crate::leaderboard::Board>(&response.body) else {
                            model.leaderboard_failed = true;
                            return render::render();
                        };
                        model.leaderboard_failed = false;
                        // O placar também diz o nome da conta: o Perfil acompanha. Um
                        // apelido recém-salvo não volta a "jogador #N" por uma resposta
                        // que saiu antes dele; o /progress é quem corrige de vez.
                        if board.me.anon_number > 0 {
                            if !model.profile.belongs_to(&owner) {
                                model.profile = crate::leaderboard::ProfileIdentity::default();
                            }
                            model.profile.owner = owner.clone();
                            model.profile.anon_number = board.me.anon_number;
                            if board.me.nickname.is_some() {
                                model.profile.nickname = board.me.nickname.clone();
                                model.profile.nickname_locked = true;
                            }
                        }
                        let cache = crate::leaderboard::LeaderboardCache { owner, board };
                        let store = Command::request_from_shell(KeyValueOperation::Set {
                            key: crate::leaderboard::LEADERBOARD_KEY.to_string(),
                            value: serde_json::to_vec(&cache).unwrap_or_default(),
                        })
                        .then_send(|_| Event::Ping);
                        model.leaderboard = Some(cache);
                        store.and(render::render())
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.pending_retry_event = Some(Event::LeaderboardRefresh);
                        self.update(Event::AttemptRefresh, model)
                    }
                    // Sem rede, 429 ou erro: a lista guardada fica, marcada como velha.
                    _ => {
                        model.leaderboard_failed = true;
                        render::render()
                    }
                }
            }

            Event::NicknameStarted => {
                reset_nickname_flow(model);
                render::render()
            }

            Event::NicknameChecked(raw) => {
                model.nickname_flow.owner = model.user_id.clone();
                match crate::leaderboard::normalize_nickname(&raw) {
                    Some(draft) => {
                        model.nickname_flow.draft = draft;
                        model.nickname_flow.error = StatusKey::Silent;
                        model.nickname_flow.step = crate::domain::NicknameStep::Confirm;
                    }
                    None => {
                        model.nickname_flow.error = StatusKey::NicknameInvalid;
                        model.nickname_flow.step = crate::domain::NicknameStep::Input;
                    }
                }
                render::render()
            }

            Event::NicknameBack => {
                model.nickname_flow.step = crate::domain::NicknameStep::Input;
                render::render()
            }

            Event::NicknameSubmitted(raw) => {
                let mine = model.nickname_flow.owner == model.user_id;
                if model.nickname_flow.submitting && mine {
                    return Command::done();
                }
                // A repetição depois do 401 só vale se a confirmação ainda está na tela:
                // fechada a folha, o pedido velho não sai sozinho.
                if model.nickname_flow.retry {
                    model.nickname_flow.retry = false;
                    if !mine || model.nickname_flow.step != crate::domain::NicknameStep::Confirm {
                        return Command::done();
                    }
                }
                model.nickname_flow.owner = model.user_id.clone();
                // O Core confere de novo, sem contar que o shell passou pelo "Continuar".
                let Some(draft) = crate::leaderboard::normalize_nickname(&raw) else {
                    model.nickname_flow.error = StatusKey::NicknameInvalid;
                    model.nickname_flow.step = crate::domain::NicknameStep::Input;
                    return render::render();
                };
                let profile_locked = model.profile.belongs_to(&model.user_id)
                    && (model.profile.nickname.is_some() || model.profile.nickname_locked);
                if profile_locked {
                    model.nickname_flow.error = StatusKey::NicknameLocked;
                    model.nickname_flow.step = crate::domain::NicknameStep::Input;
                    return render::render();
                }
                if model.access_token.is_none() {
                    model.nickname_flow.error = StatusKey::NoConnection;
                    model.nickname_flow.step = crate::domain::NicknameStep::Input;
                    return render::render();
                }
                model.nickname_flow.submitting = true;
                model.nickname_flow.draft = draft.clone();
                let request = HttpRequest {
                    method: "PUT".to_string(),
                    url: "/api/v1/profile/nickname".to_string(),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: serde_json::json!({ "nickname": draft }).to_string().into_bytes(),
                };
                let owner = model.user_id.clone();
                Command::request_from_shell(request)
                    .then_send(move |result| Event::NicknameSaved { owner: owner.clone(), draft: draft.clone(), result })
                    .and(render::render())
            }

            Event::NicknameSaved { owner, draft, result } => {
                // Só a resposta que a folha espera: a que chega depois de sair (que zera a
                // folha) ou de outra conta fica de fora.
                if owner != model.user_id || model.nickname_flow.owner != owner || !model.nickname_flow.submitting {
                    return Command::done();
                }
                model.nickname_flow.submitting = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct Saved { nickname: String }
                        let nickname = serde_json::from_slice::<Saved>(&response.body)
                            .map(|s| s.nickname)
                            .unwrap_or(draft);
                        if !model.profile.belongs_to(&owner) {
                            model.profile = crate::leaderboard::ProfileIdentity::default();
                        }
                        model.profile.owner = owner.clone();
                        model.profile.nickname = Some(nickname.clone());
                        model.profile.nickname_locked = true;
                        model.nickname_flow.draft = nickname.clone();
                        model.nickname_flow.error = StatusKey::Silent;
                        model.nickname_flow.step = crate::domain::NicknameStep::Done;
                        // A lista guardada já mostra o nome novo, sem esperar o próximo GET.
                        let store = match model.leaderboard.as_mut() {
                            Some(cache) => {
                                cache.board.me.nickname = Some(nickname.clone());
                                for row in cache.board.rows.iter_mut().filter(|r| r.is_me) {
                                    row.nickname = Some(nickname.clone());
                                }
                                Command::request_from_shell(KeyValueOperation::Set {
                                    key: crate::leaderboard::LEADERBOARD_KEY.to_string(),
                                    value: serde_json::to_vec(cache).unwrap_or_default(),
                                })
                                .then_send(|_| Event::Ping)
                            }
                            None => Command::done(),
                        };
                        return store.and(save_offline_snapshot(model)).and(render::render());
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.nickname_flow.retry = true;
                        model.pending_retry_event = Some(Event::NicknameSubmitted(draft));
                        return self.update(Event::AttemptRefresh, model);
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        model.nickname_flow.error = StatusKey::RateLimited;
                    }
                    HttpResult::Ok(response) => {
                        let code = api_code(&response.body);
                        // Um segundo envio que chega depois do primeiro ter gravado: não é
                        // erro, o apelido já é da conta.
                        if code.as_deref() == Some("nickname_locked") && model.profile.nickname.is_some() {
                            model.nickname_flow.error = StatusKey::Silent;
                            model.nickname_flow.step = crate::domain::NicknameStep::Done;
                            return render::render();
                        }
                        model.nickname_flow.error = match code.as_deref() {
                            Some("nickname_invalid") => StatusKey::NicknameInvalid,
                            Some("nickname_reserved") => StatusKey::NicknameReserved,
                            Some("nickname_taken") => StatusKey::NicknameTaken,
                            Some("nickname_locked") => {
                                model.profile.owner = owner.clone();
                                model.profile.nickname_locked = true;
                                StatusKey::NicknameLocked
                            }
                            _ => StatusKey::ServerUnreadable,
                        };
                    }
                    HttpResult::Err(_) => {
                        model.nickname_flow.error = StatusKey::NoConnection;
                    }
                }
                model.nickname_flow.step = crate::domain::NicknameStep::Input;
                render::render()
            }

            Event::NicknameFlowClosed => {
                reset_nickname_flow(model);
                render::render()
            }

            Event::Tick { now } => {
                model.now = now;
                // A validade da trilha paga pode ter mudado com a hora.
                raise_clock(model, now).and(render::render())
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
                        model.paid_track_xp = snap.paid_track_xp;
                        model.tracks = snap.tracks;
                        if snap.anon_number > 0 {
                            // Sem dono: o retrato é sempre da conta do aparelho.
                            model.profile = crate::leaderboard::ProfileIdentity {
                                owner: String::new(),
                                anon_number: snap.anon_number,
                                nickname: snap.nickname,
                                nickname_locked: snap.nickname_locked,
                            };
                        }
                        credit_queued_answers(model);
                        let snap_locale = if snap.locale.is_empty() { "pt-BR".to_string() } else { snap.locale };
                        // Retrato em outra língua (o app mudou de língua sem rede): o XP e
                        // os desafios pagos valem, o texto não. Fica a semente, que já
                        // entrou na língua certa, até a próxima busca com rede.
                        if !snap.nodes.is_empty() && snap_locale == served_locale(&model.locale) {
                            model.nodes = snap.nodes;
                            model.challenges = snap.challenges;
                            merge_track_challenges(model);
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
                        // Um refresh por vez. Dois com o mesmo token são reuso para o
                        // servidor, e reuso derruba todas as sessões da conta; o que está
                        // no ar repete o que ficou em espera quando voltar.
                        if model.refresh_in_flight {
                            return Command::done();
                        }
                        model.refresh_in_flight = true;
                        return Command::request_from_shell(request).then_send(Event::RefreshCompleted);
                    }
                }
                // Instead of showing an error on the login screen, we just remain silent
                model.status_key = StatusKey::Silent;
                model.access_token = None;
                model.is_authenticating = false;
                model.entering_session = false;

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
                                //
                                // O refresh segue "no ar" até o token novo estar no cofre: o
                                // shell roda cada operação de cofre por conta própria, e um 401
                                // que chegasse agora leria o token velho, já rodado. Para o
                                // servidor isso é reuso, e reuso derruba todas as sessões.
                                model.refresh_in_flight = true;
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

                // As compras que bateram em 401 esperam numa fila própria: a restauração
                // manda várias de uma vez, e `pending_retry_event` guarda uma só.
                let mut retries = Command::done();
                for event in std::mem::take(&mut model.purchase_retries) {
                    retries = retries.and(self.update(event, model));
                }
                // O 401 que disparou o refresh deixou um evento em espera.
                if let Some(pending) = model.pending_retry_event.take() {
                    return retries.and(self.update(pending, model));
                }
                retries.and(self.update(Event::FetchProgress, model))
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
                            #[serde(default)]
                            paid_track_xp: std::collections::HashMap<String, i32>,
                            // O nome do placar. Servidor anterior ao placar não manda.
                            #[serde(default)]
                            anon_number: i32,
                            #[serde(default)]
                            nickname: Option<String>,
                            #[serde(default)]
                            nickname_locked: bool,
                        }

                        if let Ok(stats) = serde_json::from_slice::<Stats>(&response.body) {
                            model.global_xp = stats.global_xp;
                            model.bugs_found = stats.bugs_found;
                            model.dry_runs_completed = stats.dry_runs_completed;
                            model.paid_challenges = stats.paid_challenge_ids;
                            model.paid_track_xp = stats.paid_track_xp;
                            if stats.anon_number > 0 {
                                model.profile = crate::leaderboard::ProfileIdentity {
                                    owner: model.user_id.clone(),
                                    anon_number: stats.anon_number,
                                    nickname: stats.nickname,
                                    nickname_locked: stats.nickname_locked,
                                };
                            }
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
                            // O corpo de `/api/v1/nodes` é `{"nodes": [...], "origins": [...]}`
                            // desde o ADR 0011: o cartão de origem viaja junto, como
                            // conteúdo da trilha.
                            #[derive(Deserialize)]
                            struct NodesResponse {
                                nodes: Vec<crate::domain::SkillNode>,
                                #[serde(default)]
                                origins: Vec<crate::domain::OriginCard>,
                            }

                            if let Ok(parsed) = serde_json::from_slice::<NodesResponse>(&response.body) {
                                // Substitui a lista inteira, e não completa a que
                                // estava: servidor que devolve menos nós — um nó
                                // removido — tem de encolher a trilha, não conviver
                                // com sobra da semente.
                                model.nodes = parsed.nodes;
                                model.origins = parsed.origins;
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
                            // Sem árvore nova, quem está entrando entra com a que houver.
                            model.entering_session = false;
                            render::render()
                        }
                    }
                    // Sem rede o app fica com o que já tem — semente do bundle, retrato
                    // guardado, ou nada. Trocar por uma trilha fictícia era mentir para
                    // o jogador e gerar evento de sync com node_id que não existe.
                    HttpResult::Err(_) => {
                        model.status = "Offline: mantendo a trilha que já estava".to_string();
                        model.entering_session = false;
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
                                merge_track_challenges(model);
                                model.trail_from_bundle = false;
                                model.content_locale = content_language(&response, &model.locale);
                                model.status = "Challenges loaded".to_string();
                                // Nós e desafios confirmados pelo servidor: é o momento
                                // de guardar o retrato que vai servir sem rede — e de
                                // trazer o catálogo e as trilhas baixadas desta conta.
                                let snapshot = save_offline_snapshot(model);
                                let tracks = self.update(Event::FetchTracks, model);
                                return snapshot.and(tracks).and(self.update(Event::LoadTrackDownloads, model));
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
                model.entering_session = false;
                render::render()
            }

            Event::SetDeviceId(device_id) => {
                model.device_id = device_id;
                Command::done()
            }

            Event::FetchTracks => {
                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: "/api/v1/tracks".to_string(),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: vec![],
                };
                Command::request_from_shell(request).then_send(Event::TracksFetched)
            }

            // Sem 401 com refresh aqui, nem na licença e no pacote: são buscas de fundo,
            // e a próxima abertura tenta de novo. O refresh fica com quem o jogador vê.
            Event::TracksFetched(result) => {
                // O catálogo é o último pedaço que muda a tela: com ele, a árvore e o
                // cabeçalho aparecem juntos. Deu errado ou não, a entrada acaba aqui.
                let entered = finish_entering(model);
                if let HttpResult::Ok(response) = result {
                    if response.status == 200 {
                        if let Ok(tracks) = serde_json::from_slice::<Vec<crate::domain::Track>>(&response.body) {
                            model.tracks = tracks;
                            // Comprada e ainda não neste aparelho: o login em outro aparelho
                            // libera a trilha (spec, critério 2), sem esperar um toque. Só
                            // com o índice do aparelho já lido; antes disso, é a leitura
                            // dele que pede, e os dois juntos baixariam em dobro.
                            let snapshot = entered.and(save_offline_snapshot(model));
                            if model.track_index_loaded_for != model.user_id || model.user_id.is_empty() {
                                return snapshot;
                            }
                            return snapshot.and(self.fetch_missing_licenses(model));
                        }
                    }
                }
                render::render()
            }

            Event::SubmitPurchase { jws, transaction_id, product_id, restore, provider, purchase_token } => {
                if model.access_token.is_none() || model.is_guest {
                    model.status_key = StatusKey::PurchaseNeedsAccount;
                    return render::render();
                }
                model.purchase_in_flight = true;
                model.status_key = StatusKey::PurchaseConfirming;
                // A compra que o jogador acabou de pedir ganha a tela do passo a passo; a
                // restauração conta no resumo dela; a que a loja reentrega na abertura
                // segue em silêncio.
                if !restore && !product_id.is_empty() && model.purchase_intent == product_id {
                    model.purchase_intent.clear();
                    let track_id = track_for_product(model, &product_id);
                    model.purchase_flow = Some((track_id, crate::domain::PurchaseStage::Validating, StatusKey::Silent));
                }
                // Cada loja prova a compra do seu jeito; o servidor escolhe pelo `provider`.
                let body = if provider == PROVIDER_GOOGLE_PLAY {
                    serde_json::json!({ "provider": provider, "product_id": product_id, "purchase_token": purchase_token })
                } else {
                    serde_json::json!({ "jws": jws })
                };
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: if restore { "/api/v1/purchases/restore" } else { "/api/v1/purchases" }.to_string(),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: serde_json::to_vec(&body).unwrap_or_default(),
                };
                Command::request_from_shell(request)
                    .then_send(move |result| Event::PurchaseSubmitted {
                        jws,
                        transaction_id,
                        product_id,
                        restore,
                        provider,
                        purchase_token,
                        result,
                    })
                    .and(render::render())
            }

            Event::PurchaseSubmitted { jws, transaction_id, product_id, restore, provider, purchase_token, result } => {
                model.purchase_in_flight = false;
                if restore && !matches!(&result, HttpResult::Ok(r) if r.status == 401) {
                    model.restore.done += 1;
                }
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        #[derive(Deserialize)]
                        struct Granted {
                            track_id: String,
                        }
                        model.status_key = StatusKey::PurchaseConfirmed;
                        if !restore {
                            model.purchases_to_finish.push(transaction_id);
                        }
                        if let Ok(granted) = serde_json::from_slice::<Granted>(&response.body) {
                            if let Some(track) = model.tracks.iter_mut().find(|t| t.id == granted.track_id) {
                                track.owned = true;
                                track.revoked_reason.clear();
                            }
                            if restore && !model.restore.restored_tracks.contains(&granted.track_id) {
                                model.restore.restored_tracks.push(granted.track_id.clone());
                            }
                            if let Some(flow) = model.purchase_flow.as_mut() {
                                if !restore {
                                    flow.0 = granted.track_id.clone();
                                    flow.1 = crate::domain::PurchaseStage::Licensing;
                                }
                            }
                            return self
                                .update(Event::FetchLicense { track_id: granted.track_id }, model)
                                .and(render::render());
                        }
                        // Confirmada sem dizer a trilha: não há o que baixar daqui. A
                        // próxima abertura com rede revalida pelo catálogo.
                        fail_purchase(model, "", StatusKey::TrackDownloadFailed);
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.purchase_retries.push(Event::SubmitPurchase {
                            jws,
                            transaction_id,
                            product_id,
                            restore,
                            provider,
                            purchase_token,
                        });
                        return self.update(Event::AttemptRefresh, model);
                    }
                    HttpResult::Ok(response) => {
                        let code = api_code(&response.body);
                        // Pagamento pendente no Google Play (boleto, dinheiro): a compra
                        // existe, só não foi paga. Não é recusa nem conta alheia, e a
                        // loja entrega de novo quando o pagamento cair.
                        let pending = code.as_deref() == Some("purchase_pending");
                        model.status_key = match code.as_deref() {
                            Some("purchase_owned_by_other_account") => StatusKey::PurchaseOwnedByOtherAccount,
                            Some("purchase_account_mismatch") => StatusKey::PurchaseAccountMismatch,
                            Some("purchase_revoked") => StatusKey::PurchaseRevoked,
                            Some("purchase_pending") => StatusKey::PurchasePending,
                            Some("store_unavailable") => StatusKey::StoreUnavailable,
                            _ => StatusKey::PurchaseFailed,
                        };
                        if restore && response.status == 409 && !pending {
                            model.restore.other_account += 1;
                        }
                        // Recusa definitiva (403, 409, e o 400 de produto desconhecido ou
                        // transação inválida): o servidor já decidiu, e deixar a transação
                        // aberta faria a loja entregá-la de novo a cada abertura, para
                        // sempre. Não consumível volta por "Restaurar compras" se for o caso.
                        // Erro do servidor, e o pendente, ficam abertos e tentam de novo.
                        let permanent = (matches!(response.status, 403 | 409) && !pending)
                            || (response.status == 400
                                && matches!(code.as_deref(), Some("unknown_product" | "purchase_invalid")));
                        if !restore && permanent {
                            model.purchases_to_finish.push(transaction_id);
                        }
                        if !restore {
                            if let Some(flow) = model.purchase_flow.as_mut() {
                                flow.1 = crate::domain::PurchaseStage::Failed;
                                flow.2 = model.status_key.clone();
                            }
                        }
                    }
                    HttpResult::Err(_) => {
                        model.status_key = StatusKey::PurchaseFailed;
                        if !restore {
                            if let Some(flow) = model.purchase_flow.as_mut() {
                                flow.1 = crate::domain::PurchaseStage::Failed;
                                flow.2 = StatusKey::PurchaseFailed;
                            }
                        }
                    }
                }
                render::render()
            }

            Event::PurchaseFinished { transaction_id } => {
                model.purchases_to_finish.retain(|t| t != &transaction_id);
                render::render()
            }

            Event::ClosePurchaseFlow => {
                // Fechar com a trilha pronta é "abrir a trilha": a árvore passa a ela.
                // Antes disso, a compra segue em segundo plano: licença e pacote chegam
                // sem a tela.
                if let Some((track_id, crate::domain::PurchaseStage::Ready, _)) = model.purchase_flow.take() {
                    return self.update(Event::SelectTrack { track_id }, model);
                }
                render::render()
            }

            Event::RestoreStarted { count } => {
                model.restore = RestoreProgress { started: true, total: count, ..Default::default() };
                render::render()
            }

            Event::PurchaseIntent { product_id } => {
                model.purchase_intent = product_id;
                Command::done()
            }

            Event::DismissRestoreResult => {
                model.restore = RestoreProgress::default();
                render::render()
            }

            Event::SelectTrack { track_id } => {
                model.selected_track = track_id.clone();
                Command::request_from_shell(KeyValueOperation::Set {
                    key: SELECTED_TRACK_KEY.to_string(),
                    value: track_id.into_bytes(),
                })
                .then_send(|_| Event::Ping)
                .and(render::render())
            }

            Event::DismissSampleOffer { track_id } => {
                if !model.offer_dismissed.contains(&track_id) {
                    model.offer_dismissed.push(track_id);
                }
                render::render()
            }

            Event::CatalogOpened => {
                let today = local_day(model);
                if model.catalog_seen_day == today {
                    return Command::done();
                }
                model.catalog_seen_day = today;
                Command::request_from_shell(KeyValueOperation::Set {
                    key: CATALOG_SEEN_KEY.to_string(),
                    value: today.to_string().into_bytes(),
                })
                .then_send(|_| Event::Ping)
                .and(render::render())
            }

            Event::SetUtcOffset { seconds } => {
                model.utc_offset = seconds;
                Command::done()
            }

            Event::CompleteOnboarding => {
                model.onboarding_done = true;
                Command::request_from_shell(KeyValueOperation::Set {
                    key: ONBOARDING_KEY.to_string(),
                    value: b"1".to_vec(),
                })
                .then_send(|_| Event::Ping)
                .and(render::render())
            }

            Event::RestorePreferences => {
                let read = |key: &'static str| {
                    Command::request_from_shell(KeyValueOperation::Get { key: key.to_string() })
                        .then_send(move |result| Event::PreferenceRestored { key: key.to_string(), result })
                };
                read(SELECTED_TRACK_KEY).and(read(CATALOG_SEEN_KEY)).and(read(ONBOARDING_KEY))
            }

            Event::PreferenceRestored { key, result } => {
                let value = match result {
                    KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } => {
                        String::from_utf8(bytes).unwrap_or_default()
                    }
                    _ => String::new(),
                };
                match key.as_str() {
                    SELECTED_TRACK_KEY => {
                        // Uma escolha feita nesta abertura, antes da leitura, vale mais.
                        if model.selected_track.is_empty() {
                            model.selected_track = value;
                        }
                    }
                    CATALOG_SEEN_KEY => model.catalog_seen_day = value.parse().unwrap_or(0),
                    ONBOARDING_KEY => {
                        model.onboarding_done = model.onboarding_done || value == "1";
                        // O onboarding é a última leitura pedida: com ela, dá para decidir.
                        model.prefs_loaded = true;
                    }
                    _ => {}
                }
                render::render()
            }

            Event::FetchLicense { track_id } => {
                // O id vira caminho da URL: só a forma de um UUID passa.
                if model.device_id.is_empty() || !track_id.chars().all(|c| c.is_ascii_hexdigit() || c == '-') {
                    fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                    return render::render();
                }
                // Abriu sem rede e a rede voltou: a sessão vale, só o token ainda não foi
                // renovado. Recusar aqui deixava o "Baixar" mudo até fechar o app.
                if model.access_token.is_none() {
                    if !model.session_offline {
                        fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                        return render::render();
                    }
                    return retry_license_after_refresh(self, model, track_id);
                }
                let mut headers = auth_headers(&model.access_token, &model.locale);
                headers.push(crux_http::protocol::HttpHeader {
                    name: "X-Device-ID".to_string(),
                    value: model.device_id.clone(),
                });
                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: format!("/api/v1/tracks/{track_id}/license"),
                    headers,
                    body: vec![],
                };
                let user_id = model.user_id.clone();
                Command::request_from_shell(request)
                    .then_send(move |result| Event::LicenseFetched { user_id, track_id, result })
            }

            Event::LicenseFetched { user_id, track_id, result } => {
                // Resposta de um pedido de outra conta, que saiu no meio do caminho.
                if user_id != model.user_id {
                    return Command::done();
                }
                let HttpResult::Ok(response) = result else {
                    // Sem rede: a licença guardada segue valendo até o prazo dela.
                    fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                    return render::render();
                };
                match response.status {
                    200 => {
                        model.license_retried.retain(|t| *t != track_id);
                        let Ok(license) = serde_json::from_slice::<crate::domain::TrackLicense>(&response.body) else {
                            fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                            return render::render();
                        };
                        if license.track_id != track_id || model.user_id.is_empty() {
                            fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                            return render::render();
                        }
                        let needs_package = model
                            .track_content
                            .get(&track_id)
                            .is_none_or(|c| c.content_version != license.content_version);
                        let stored = Command::request_from_shell(KeyValueOperation::Set {
                            key: crate::tracks::license_key(&model.user_id, &track_id),
                            value: serde_json::to_vec(&license).unwrap_or_default(),
                        })
                        .then_send(|_| Event::Ping);
                        let issued_at = license.issued_at;
                        model.licenses.insert(track_id.clone(), license);
                        if !model.track_index.contains(&track_id) {
                            model.track_index.push(track_id.clone());
                        }
                        if let Some(track) = model.tracks.iter_mut().find(|t| t.id == track_id) {
                            track.owned = true;
                        }
                        merge_track_challenges(model);
                        advance_purchase(
                            model,
                            &track_id,
                            if needs_package { crate::domain::PurchaseStage::Downloading } else { crate::domain::PurchaseStage::Ready },
                        );
                        // A hora da emissão é do servidor: é piso confiável para o relógio.
                        let mut cmd = stored.and(store_track_index(model)).and(raise_clock(model, issued_at));
                        if needs_package {
                            cmd = cmd.and(self.update(Event::FetchPackage { track_id }, model));
                        }
                        cmd.and(render::render())
                    }
                    // Revogada: chave e pacote saem do aparelho, o XP fica (spec, seção 7).
                    403 if api_code(&response.body).as_deref() == Some("entitlement_required") => {
                        if model.track_index.contains(&track_id) || model.licenses.contains_key(&track_id) {
                            model.status_key = StatusKey::TrackRevoked;
                        }
                        if let Some(track) = model.tracks.iter_mut().find(|t| t.id == track_id) {
                            track.owned = false;
                        }
                        if let Some(flow) = model.purchase_flow.as_mut() {
                            if flow.0 == track_id {
                                flow.1 = crate::domain::PurchaseStage::Failed;
                                flow.2 = StatusKey::TrackRevoked;
                            }
                        }
                        forget_track(model, &track_id).and(render::render())
                    }
                    // O token venceu com o app aberto: renova e pede de novo, uma vez.
                    401 if !model.license_retried.contains(&track_id) => {
                        model.license_retried.push(track_id.clone());
                        retry_license_after_refresh(self, model, track_id)
                    }
                    _ => {
                        fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                        render::render()
                    }
                }
            }

            Event::FetchPackage { track_id } => {
                let request = HttpRequest {
                    method: "GET".to_string(),
                    url: format!("/api/v1/tracks/{track_id}/package"),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: vec![],
                };
                let user_id = model.user_id.clone();
                Command::request_from_shell(request)
                    .then_send(move |result| Event::PackageFetched { user_id, track_id, result })
            }

            Event::PackageFetched { user_id, track_id, result } => {
                if user_id != model.user_id {
                    return Command::done();
                }
                let HttpResult::Ok(response) = result else {
                    fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                    return render::render();
                };
                if response.status == 403 {
                    fail_purchase(model, &track_id, StatusKey::TrackRevoked);
                    return forget_track(model, &track_id).and(render::render());
                }
                if response.status != 200 || model.user_id.is_empty() {
                    fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                    return render::render();
                }
                let Some(license) = model.licenses.get(&track_id) else {
                    fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                    return render::render();
                };
                // Só guarda o que abre: pacote que não abre com a licença não serve para nada.
                match crate::tracks::open_package(license, &response.body) {
                    Ok(content) => {
                        model.track_content.insert(
                            track_id.clone(),
                            TrackContent {
                                content_version: content.content_version,
                                bytes: response.body.len() as u64,
                                challenges: content.challenges,
                            },
                        );
                        merge_track_challenges(model);
                        advance_purchase(model, &track_id, crate::domain::PurchaseStage::Ready);
                        Command::request_from_shell(KeyValueOperation::Set {
                            key: crate::tracks::package_key(&model.user_id, &track_id),
                            value: response.body,
                        })
                        .then_send(|_| Event::Ping)
                        .and(render::render())
                    }
                    Err(e) => {
                        model.status = format!("Track package does not open: {e:?}");
                        fail_purchase(model, &track_id, StatusKey::TrackDownloadFailed);
                        render::render()
                    }
                }
            }

            Event::LoadTrackDownloads => {
                if model.user_id.is_empty() || model.is_guest || model.track_downloads_owner == model.user_id {
                    return Command::done();
                }
                model.track_downloads_owner = model.user_id.clone();
                let user_id = model.user_id.clone();
                Command::request_from_shell(KeyValueOperation::Get { key: crate::tracks::index_key(&user_id) })
                    .then_send(move |result| Event::TrackIndexRead { user_id, result })
            }

            Event::TrackIndexRead { user_id, result } => {
                if user_id != model.user_id {
                    return Command::done();
                }
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result {
                    for id in serde_json::from_slice::<Vec<String>>(&bytes).unwrap_or_default() {
                        if !model.track_index.contains(&id) {
                            model.track_index.push(id);
                        }
                    }
                }
                let clock_user = user_id.clone();
                let mut cmd = Command::request_from_shell(KeyValueOperation::Get { key: crate::tracks::clock_key(&user_id) })
                    .then_send(move |result| Event::ClockRead { user_id: clock_user, result });
                for track_id in model.track_index.clone() {
                    let (user, track) = (user_id.clone(), track_id.clone());
                    cmd = cmd.and(
                        Command::request_from_shell(KeyValueOperation::Get { key: crate::tracks::license_key(&user_id, &track_id) })
                            .then_send(move |result| Event::LicenseRead { user_id: user, track_id: track, result }),
                    );
                    // Com rede, cada abertura revalida (spec, seção 6).
                    cmd = cmd.and(self.update(Event::FetchLicense { track_id }, model));
                }
                model.track_index_loaded_for = user_id;
                cmd.and(self.fetch_missing_licenses(model))
            }

            Event::ClockRead { user_id, result } => {
                if user_id != model.user_id {
                    return Command::done();
                }
                if let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result {
                    if let Some(seen) = String::from_utf8(bytes).ok().and_then(|s| s.parse::<i64>().ok()) {
                        model.clock_high_water = model.clock_high_water.max(seen);
                    }
                }
                render::render()
            }

            Event::LicenseRead { user_id, track_id, result } => {
                if user_id != model.user_id {
                    return Command::done();
                }
                let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result else {
                    return Command::done();
                };
                let Ok(license) = serde_json::from_slice::<crate::domain::TrackLicense>(&bytes) else {
                    return Command::done();
                };
                if license.track_id != track_id {
                    return Command::done();
                }
                // A revalidação pode ter chegado antes do disco: fica a mais nova.
                let newer = model.licenses.get(&track_id).is_none_or(|l| l.issued_at < license.issued_at);
                if newer {
                    model.licenses.insert(track_id.clone(), license);
                }
                Command::request_from_shell(KeyValueOperation::Get { key: crate::tracks::package_key(&user_id, &track_id) })
                    .then_send(move |result| Event::PackageRead { user_id, track_id, result })
            }

            Event::PackageRead { user_id, track_id, result } => {
                if user_id != model.user_id || model.track_content.contains_key(&track_id) {
                    return render::render();
                }
                let KeyValueResult::Ok { response: KeyValueResponse::Get { value: crux_kv::Value::Bytes(bytes) } } = result else {
                    return render::render();
                };
                if let Some(license) = model.licenses.get(&track_id) {
                    if let Ok(content) = crate::tracks::open_package(license, &bytes) {
                        model.track_content.insert(
                            track_id,
                            TrackContent { content_version: content.content_version, bytes: bytes.len() as u64, challenges: content.challenges },
                        );
                        merge_track_challenges(model);
                    }
                }
                render::render()
            }

            Event::DeleteTrackDownload { track_id } => forget_track(model, &track_id).and(render::render()),

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
                    // Código errado demais para o e-mail (`otp_locked`): o servidor não
                    // manda código novo por um dia. Contagem de um dia no botão não
                    // ajuda ninguém; a mensagem diz o que houve.
                    HttpResult::Ok(response) if response.status == 429
                        && api_code(&response.body).as_deref() == Some("otp_locked") => {
                        model.status_key = StatusKey::CodeLocked;
                    }
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
                    "client": signup_client(model),
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
                            model.entering_session = arriving_from_login(model);
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
            Event::DeleteAccount { password } => {
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
                    body: serde_json::json!({ "password": password }).to_string().into_bytes(),
                };
                Command::request_from_shell(request)
                    .then_send(Event::AccountDeleted)
                    .and(render::render())
            }
            Event::DeleteAccountWithProvider { provider, id_token, nonce, authorization_code } => {
                if model.access_token.is_none() {
                    model.status_key = StatusKey::NoConnection;
                    return render::render();
                }
                model.is_authenticating = true;
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/users/me/delete".to_string(),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: serde_json::json!({
                        "provider": provider, "id_token": id_token, "nonce": nonce,
                        "authorization_code": authorization_code,
                    })
                    .to_string()
                    .into_bytes(),
                };
                Command::request_from_shell(request)
                    .then_send(Event::AccountDeleted)
                    .and(render::render())
            }
            Event::DeleteAccountWithGitHub { code, code_verifier, nonce } => {
                if model.access_token.is_none() {
                    model.status_key = StatusKey::NoConnection;
                    return render::render();
                }
                // Uma troca por vez: o nonce da outra seria trocado por este no meio.
                if model.github_nonce.is_some() {
                    return Command::done();
                }
                model.is_authenticating = true;
                let mut request = github_exchange_request(model, &code, &code_verifier, &nonce, "delete");
                // A troca de exclusão pede a sessão, como a exclusão.
                request.headers = auth_headers(&model.access_token, &model.locale);
                model.github_nonce = Some(nonce);
                Command::request_from_shell(request)
                    .then_send(Event::GitHubDeleteExchanged)
                    .and(render::render())
            }
            Event::GitHubDeleteExchanged(result) => {
                model.is_authenticating = false;
                let Some(nonce) = model.github_nonce.take() else {
                    return Command::done();
                };
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        match serde_json::from_slice::<GitHubExchange>(&response.body) {
                            // O access token passa só por aqui, a caminho do servidor, que o
                            // usa para revogar e não o devolve.
                            Ok(ex) if !ex.ticket.is_empty() && !ex.access_token.is_empty() => {
                                return self.update(
                                    Event::DeleteAccountWithProvider {
                                        provider: "github".into(),
                                        id_token: ex.ticket,
                                        nonce,
                                        authorization_code: ex.access_token,
                                    },
                                    model,
                                );
                            }
                            _ => model.status_key = StatusKey::ServerUnreadable,
                        }
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    HttpResult::Ok(response) => {
                        model.status_key = match api_code(&response.body).as_deref() {
                            Some("provider_disabled") => StatusKey::SocialProviderDisabled,
                            _ => StatusKey::WrongCredentials,
                        };
                    }
                    HttpResult::Err(_) => {
                        model.status_key = StatusKey::NoConnection;
                    }
                }
                render::render()
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
                        // A conta saiu: o bloqueio dos termos era dela.
                        model.terms_pending = None;
                        model.terms_notice = false;
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
                        model.leaderboard = None;
                        model.leaderboard_disk_read_for.clear();
                        model.leaderboard_disk_pending = false;
                        model.leaderboard_in_flight = None;
                        model.leaderboard_failed = false;
                        model.profile = crate::leaderboard::ProfileIdentity::default();
                        model.nickname_flow = crate::leaderboard::NicknameFlow::default();
                        let queue = queue_key(&model.queue_owner);
                        return delete("refresh_token")
                            .and(delete("account_email"))
                            .and(delete("account_user_id"))
                            .and(delete("session_expires_at"))
                            .and(delete(&queue))
                            .and(delete("offline_snapshot"))
                            .and(delete(crate::leaderboard::LEADERBOARD_KEY))
                            // O aparelho para de mandar eventos com o id da conta apagada;
                            // os que já estão no PostHog somem pela retenção de 30 dias.
                            .and(Command::request_from_shell(TelemetryOperation::Reset).then_send(|_| Event::TelemetrySent))
                            .and(self.claim_queue(model, GUEST_QUEUE_OWNER, &[""]))
                            .and(render::render());
                    }
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.status_key = StatusKey::WrongCredentials;
                    }
                    // A conta também entra pela Apple: a senha não basta, porque só a
                    // confirmação pela Apple traz o código que revoga o acesso.
                    HttpResult::Ok(response) if response.status == 409
                        && api_code(&response.body).as_deref() == Some("provider_reauth_required") => {
                        model.status_key = StatusKey::ProviderReauthRequired;
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
                            model.entering_session = arriving_from_login(model);
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
                // A fila sobe em lotes: o servidor recusa sync acima de 500 eventos, e
                // quem jogou muito tempo sem rede passaria disso. A resposta de um lote
                // dispara o seguinte, e o resto da fila já está encadeado a partir do
                // último enviado.
                let batch: Vec<GameEvent> = model.pending_events.iter().take(SYNC_BATCH).cloned().collect();
                model.sync_sent_ids = batch.iter().map(|e| e.id.clone()).collect();
                model.sync_owner = model.queue_owner.clone();

                let payload = SyncPayload {
                    user_id: model.user_id.clone(),
                    events: batch,
                };
                
                let body_bytes = serde_json::to_vec(&payload).unwrap_or_default();
                
                let request = HttpRequest {
                    method: "POST".to_string(),
                    url: "/api/v1/sync".to_string(),
                    headers: auth_headers(&model.access_token, &model.locale),
                    body: body_bytes,
                };
                
                // O render vai junto, como no `Login`: sem ele o `is_syncing` só chegava ao
                // shell com a resposta, já desligado, e "Sincronizar e sair" não mostrava
                // que estava esperando a rede.
                Command::request_from_shell(request)
                    .then_send(Event::SyncCompleted)
                    .and(render::render())
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
                            // O `SyncNow` pôs o "sincronizando" na tela; fora da abertura,
                            // ninguém mais renderiza para tirá-lo.
                            return flush.and(boot).and(render::render());
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
                // A tela só oferece o nó aberto, mas a trava é aqui: licença vencida com
                // o desafio ainda na memória não abre partida.
                if model.nodes.iter().any(|n| n.id == node_id && !node_open(model, n)) {
                    model.match_state = None;
                    model.status = "Node requires a valid license".to_string();
                    return render::render();
                }
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

            Event::MatchPickTradeoff { value } => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.pick_tradeoff(value);
                }
                render::render()
            }

            Event::MatchClearBenefit => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.tradeoff_benefit = None;
                }
                render::render()
            }

            Event::MatchClearDrawback => {
                if let Some(ref mut ms) = model.match_state {
                    ms.selection.tradeoff_drawback = None;
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
                        credit_track_xp(&model.nodes, &mut model.paid_track_xp, &node_id);
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
                // Sem rede, é aqui que se sabe de quem é a sessão: as trilhas baixadas
                // desta conta abrem pela licença guardada, até o prazo dela.
                let downloads = self.update(Event::LoadTrackDownloads, model);
                self.claim_queue(model, &owner, &[""]).and(downloads)
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

            Event::EnterWatchdogElapsed { attempt } if attempt != model.entering_attempt => Command::done(),
            // A trilha não chegou no prazo: abre com o que há. O que ainda estiver no ar
            // chega depois, como em qualquer busca de fundo.
            Event::EnterWatchdogElapsed { .. } => finish_entering(model),
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
                // Os termos não responderam a tempo: o app entra, e a resposta que chegar
                // depois ainda põe a tela de aceite na frente, se houver.
                BootCheck::Terms => self.settle_boot_terms(model, BootVerdict::Skipped, BootDetail::TermsDeferred),
            },

            Event::SetClientInfo { app_version, platform } => {
                model.client_app_version = app_version;
                model.client_platform = platform;
                Command::done()
            }

            Event::FetchTermsPending => {
                if model.access_token.is_none() {
                    return Command::done();
                }
                fetch_terms_pending(model)
            }

            // Resposta de outra conta (saiu uma, entrou outra no meio): não decide nada aqui.
            Event::TermsPendingFetched { owner, .. } if owner != model.user_id => Command::done(),

            Event::TermsPendingFetched { result, .. } => {
                let detail = match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        match serde_json::from_slice::<TermsPending>(&response.body) {
                            Ok(pending) if pending.documents.is_empty() => {
                                model.terms_pending = None;
                                BootDetail::TermsCurrent
                            }
                            Ok(pending) if pending.blocking => {
                                model.terms_pending = Some(pending);
                                model.terms_pending_owner = model.user_id.clone();
                                BootDetail::TermsChanged
                            }
                            // Só não relevantes: a faixa aparece, e o aceite é gravado ao
                            // aparecer, como pede o design. Fechar a faixa também conta.
                            Ok(pending) => {
                                model.terms_pending = None;
                                model.terms_notice = true;
                                let accept = Command::request_from_shell(terms_accept_request(model, &pending, "notice"))
                                    .then_send(Event::TermsNoticeAccepted);
                                return accept.and(self.settle_boot_terms(model, BootVerdict::Ok, BootDetail::TermsNotice));
                            }
                            Err(_) => BootDetail::TermsDeferred,
                        }
                    }
                    // Fora da abertura, o token venceu: renova e pergunta de novo, em vez de
                    // deixar a conta sem conferir até a próxima abertura.
                    HttpResult::Ok(response) if response.status == 401 && !model.boot.terms_running() => {
                        model.pending_retry_event = Some(Event::FetchTermsPending);
                        return self.update(Event::AttemptRefresh, model);
                    }
                    // Qualquer outra resposta, ou sem rede: confere na próxima abertura. O
                    // bloqueio que já estava na memória continua.
                    _ => BootDetail::TermsDeferred,
                };
                let verdict = if detail == BootDetail::TermsDeferred { BootVerdict::Skipped } else { BootVerdict::Ok };
                self.settle_boot_terms(model, verdict, detail)
            }

            Event::AcceptTerms => {
                let Some(pending) = model.terms_pending.clone() else {
                    return Command::done();
                };
                if model.terms_accepting || !pending.blocking {
                    return Command::done();
                }
                model.terms_accepting = true;
                model.status_key = StatusKey::Silent;
                Command::request_from_shell(terms_accept_request(model, &pending, "reaccept"))
                    .then_send(Event::TermsAccepted)
                    .and(render::render())
            }

            Event::TermsAccepted(result) => {
                model.terms_accepting = false;
                match result {
                    HttpResult::Ok(response) if response.status == 200 => {
                        model.terms_pending = None;
                        model.status_key = StatusKey::Silent;
                    }
                    // A tela estava velha (versão nova no meio, ou o aceite já tinha ido):
                    // busca de novo, e o que o servidor disser agora é o que vale.
                    HttpResult::Ok(response)
                        if response.status == 409 && api_code(&response.body).as_deref() == Some("legal_version_outdated") =>
                    {
                        return self.update(Event::FetchTermsPending, model).and(render::render());
                    }
                    // Quem ficou na tela além do prazo do token: renova e manda o mesmo aceite,
                    // em vez de deixar o botão mudo até fechar o app.
                    HttpResult::Ok(response) if response.status == 401 => {
                        model.pending_retry_event = Some(Event::AcceptTerms);
                        return self.update(Event::AttemptRefresh, model);
                    }
                    HttpResult::Ok(response) if response.status == 429 => {
                        return rate_limited(model, &response, false);
                    }
                    HttpResult::Ok(_) => model.status_key = StatusKey::ServerUnreadable,
                    HttpResult::Err(_) => model.status_key = StatusKey::NoConnection,
                }
                render::render()
            }

            // Se o aceite da faixa não chegou, a próxima abertura mostra a faixa de novo.
            Event::TermsNoticeAccepted(_) => Command::done(),

            Event::DismissTermsNotice => {
                model.terms_notice = false;
                render::render()
            }

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
                        if model.origins.is_empty() {
                            model.origins = trail.origins;
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
        };
        let command = handle();
        match observed {
            Some(log) => command.and(log),
            None => command,
        }
    }

        fn view(&self, model: &Self::Model) -> Self::ViewModel {
        // O nome do placar só aparece para a conta de quem ele é.
        let profile = Some(&model.profile).filter(|p| p.belongs_to(&model.user_id));
        let mut computed_nodes = model.nodes.clone();
        
        // Calculate DAG status
        // O paywall vem antes do portão: o nó pago que o jogador toca abre a compra, não o
        // "faltam N XP". Comprar não pula o portão (spec, seção 8), que conta só o XP da
        // trilha do nó.
        for node in computed_nodes.iter_mut() {
            node.status = if !node_open(model, node) {
                crate::domain::NodeStatus::PaywallLocked
            } else if track_xp(model, &node.track_id) < node.required_xp {
                crate::domain::NodeStatus::Locked
            } else {
                // Conquistado quando algum filho já abriu pelo XP da trilha dele. Filho
                // atrás do paywall não abriu: sem isto, a amostra aparecia conquistada
                // sem nenhum problema resolvido, só porque o nó pago tem portão zero.
                let has_unlocked_child = model.nodes.iter().any(|child| {
                    child.prerequisites.contains(&node.id)
                        && node_open(model, child)
                        && track_xp(model, &child.track_id) >= child.required_xp
                });
                if has_unlocked_child {
                    crate::domain::NodeStatus::Completed
                } else {
                    crate::domain::NodeStatus::Active
                }
            };
        }

        // Problema a problema, na ordem das letras da partida — a mesma ordem em que
        // `StartMatch` as distribui.
        for node in computed_nodes.iter_mut() {
            node.problems_solved = model.challenges.iter()
                .filter(|c| c.node_id == node.id)
                .map(|c| model.paid_challenges.contains(&c.id))
                .collect();
        }

        // O catálogo sai dos nós já calculados: "4 de 7 nós" conta conquistados.
        let tracks: Vec<crate::domain::TrackView> =
            model.tracks.iter().map(|t| track_view(model, t, &computed_nodes)).collect();
        let current = effective_track(model);
        let current_track = tracks.iter().find(|t| t.id == current).cloned().unwrap_or_else(|| crate::domain::TrackView {
            id: current.clone(),
            is_free: true,
            selected: true,
            ..Default::default()
        });

        // A árvore mostra uma trilha por vez. Sem isto, a trilha paga desenhava por cima
        // da principal: a posição na grade é única por trilha, não no mapa inteiro.
        //
        // Nenhum nó da trilha escolhida (semente de antes de `track_id`, com o catálogo
        // já no retrato): fica o que não é de trilha paga, em vez de uma árvore vazia.
        if computed_nodes.iter().any(|n| n.track_id == current) {
            computed_nodes.retain(|n| n.track_id == current);
        } else {
            computed_nodes.retain(|n| !is_paid_track(model, &n.track_id));
        }

        // "N balões no ar" conta nós conquistados, não problemas aceitos.
        let balloons_up = computed_nodes
            .iter()
            .filter(|n| n.status == crate::domain::NodeStatus::Completed)
            .count() as i32;

        let purchase_flow = model
            .purchase_flow
            .as_ref()
            .map(|(track_id, stage, failure)| crate::domain::PurchaseFlowView {
                stage: *stage,
                track_id: track_id.clone(),
                failure: failure.clone(),
            })
            .unwrap_or_default();
        let restore_result = crate::domain::RestoreResultView {
            active: model.restore.started,
            finished: model.restore.started && model.restore.done >= model.restore.total,
            total: model.restore.total,
            restored: model.restore.restored_tracks.len() as u32,
            other_account: model.restore.other_account,
            restored_names: model
                .restore
                .restored_tracks
                .iter()
                .filter_map(|id| model.tracks.iter().find(|t| &t.id == id).map(|t| t.name.clone()))
                .collect(),
        };
        let catalog_badge = catalog_badge(model, &tracks);
        let show_onboarding = model.prefs_loaded
            && !model.onboarding_done
            && (model.access_token.is_some() || model.session_offline || model.is_guest)
            && !model.entering_session
            && !model.boot.active;

        ViewModel {
            status: model.status_key.clone(),
            pending_sync_count: model.pending_events.len() as u32,
            is_syncing: model.is_syncing,
            is_fetching: model.is_fetching,
            // Quem está entrando continua no login, com o botão esperando, até a trilha
            // chegar: o shell só vê a sessão quando há o que mostrar nela.
            is_authenticating: model.is_authenticating || model.entering_session,
            has_access_token: model.access_token.is_some() && !model.entering_session,
            has_session: (model.access_token.is_some() || model.session_offline) && !model.entering_session,
            is_offline_session: model.session_offline,
            trail_from_bundle: model.trail_from_bundle,
            trail_generated_at: model.trail_generated_at.clone(),
            match_left: model.match_left,
            is_guest: model.is_guest,
            locale: model.locale.clone(),
            challenges: model.challenges.clone(),
            nodes: computed_nodes,
            otp_email: model.otp_email.clone(),
            // Como o nome abaixo: depois de sair, vem do instantâneo. O perfil ainda está
            // descendo quando a saída chega, e sem isto ele descia sem e-mail, com o
            // avatar do cabeçalho virando "?".
            account_email: model
                .logout_undo
                .as_ref()
                .map(|s| s.email.clone())
                .unwrap_or_else(|| model.account_email.clone()),
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
                .map(|ms| ms.to_view_model(resolve_origin_card(model).as_ref()))
                .unwrap_or_default(),
            auth_cooldown_seconds: model.auth_cooldown.remaining(model.now),
            resend_cooldown_seconds: model
                .resend_cooldown
                .remaining(model.now)
                .max(model.auth_cooldown.remaining(model.now)),
            legal_versions_ready: legal_versions_complete(&model.legal_versions),
            min_age: if model.min_age == 0 { DEFAULT_MIN_AGE } else { model.min_age },
            deletion_purge_after: model.deletion_purge_after,
            account_restored_notice: model.account_restored_notice,
            social_signup_required: model.social_signup_required,
            analytics_enabled: !model.analytics_disabled,
            boot: boot_view(&model.boot),
            terms_update: terms_update_view(model),
            terms_notice: model.terms_notice && model.access_token.is_some(),
            resume_email: model.resume_email.clone(),
            account_user_id: if model.is_guest { String::new() } else { model.user_id.clone() },
            tracks,
            current_track,
            catalog_badge,
            show_onboarding,
            purchase_flow,
            restore_result,
            sample_offer: sample_offer(model),
            purchases_to_finish: model.purchases_to_finish.clone(),
            purchase_in_flight: model.purchase_in_flight,
            leaderboard: crate::leaderboard::view(&crate::leaderboard::BoardContext {
                signed_in: !model.is_guest && (model.access_token.is_some() || model.session_offline),
                cache: model.leaderboard.as_ref().filter(|c| c.owner == model.user_id),
                in_flight: model.leaderboard_in_flight.as_deref() == Some(model.user_id.as_str()),
                disk_pending: model.leaderboard_disk_pending,
                failed: model.leaderboard_failed,
                session_offline: model.session_offline,
                now: model.now,
            }),
            profile_anon_number: profile.map_or(0, |p| p.anon_number),
            profile_nickname: profile.and_then(|p| p.nickname.clone()),
            can_choose_nickname: !model.is_guest
                && profile.is_some_and(|p| p.anon_number > 0 && p.nickname.is_none() && !p.nickname_locked),
            nickname_flow: if model.nickname_flow.owner == model.user_id {
                crate::domain::NicknameFlowView {
                    step: model.nickname_flow.step.clone(),
                    draft: model.nickname_flow.draft.clone(),
                    submitting: model.nickname_flow.submitting,
                    error: model.nickname_flow.error.clone(),
                }
            } else {
                crate::domain::NicknameFlowView::default()
            },
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
        // Sem o render junto do request, "Sincronizar e sair" não mostra que está esperando.
        let effects: Vec<_> = cmd.effects().collect();
        assert!(effects.iter().any(|e| matches!(e, Effect::Render(_))), "a espera tem de chegar à tela");
        assert!(app.view(&model).is_syncing);
        assert!(effects.iter().any(|e| matches!(e, Effect::Http(r) if r.operation.url == "/api/v1/sync")));

        let failure = HttpResult::Ok(crux_http::protocol::HttpResponse {
            status: 500, headers: vec![], body: vec![],
        });
        let _ = app.update(Event::SyncCompleted(failure), &mut model);

        assert_eq!(model.pending_events.len(), 1, "a fila que não subiu fica");
        assert!(model.access_token.is_some(), "e a sessão continua de pé");
        assert!(!model.logout_after_sync);
    }

    /// O sync de fundo que deu certo tira o "sincronizando" da tela.
    ///
    /// Com o render no `SyncNow`, o shell passou a ver o envio começar; sem render na
    /// resposta, fora da abertura, ele ficava "sincronizando" até outro render qualquer.
    #[test]
    fn test_a_background_sync_that_succeeds_renders_the_end() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = USER_B.into();
        model.pending_events = vec![answer("a1", GameEvent::GENESIS)];

        let _ = app.update(Event::SyncNow, &mut model);
        let mut cmd = app.update(Event::SyncCompleted(http(200, serde_json::json!({ "new_top": "abc" }))), &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Render(_))));
        assert!(!app.view(&model).is_syncing);
    }

    /// Leva um login por senha até o fim da cadeia do cofre, onde a trilha é pedida.
    fn login_until_session_stored(app: &LogNApp, model: &mut Model) -> Command<Effect, Event> {
        let _ = app.update(Event::Login { email: "a@example.com".into(), password: "senha".into() }, model);
        let _ = app.update(
            Event::LoginCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "ref", "user_id": USER_B,
            }))),
            model,
        );
        let _ = app.update(Event::TokenStored(kv_empty()), model);
        let _ = app.update(Event::AccountEmailStored(kv_empty()), model);
        app.update(Event::SessionExpiryStored(kv_empty()), model)
    }

    /// O app abre depois do login uma vez só, com a trilha e o catálogo já na tela.
    ///
    /// Abria com o token, antes do conteúdo, e cada resposta redesenhava a árvore: a
    /// semente com a tarja, a trilha do servidor, o cabeçalho quando o catálogo chegava.
    /// Depois de um logout, a primeira tela era "não foi possível carregar o mapa".
    #[test]
    fn test_login_opens_the_app_once_the_trail_arrived() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut cmd = login_until_session_stored(&app, &mut model);
        let urls: Vec<String> = cmd
            .effects()
            .filter_map(|e| match e {
                Effect::Http(r) => Some(r.operation.url.clone()),
                _ => None,
            })
            .collect();
        assert!(urls.contains(&"/api/v1/nodes".to_string()), "a trilha é pedida pelo login, não pela árvore");

        let waiting = |model: &Model| {
            let view = app.view(model);
            !view.has_session && !view.has_access_token && view.is_authenticating
        };
        assert!(waiting(&model), "o login segue esperando, com o botão girando");

        let _ = app.update(Event::ProgressFetched(http(200, serde_json::json!({
            "global_xp": 0, "bugs_found": 0, "dry_runs_completed": 0,
        }))), &mut model);
        let _ = app.update(Event::NodesFetched(http(200, serde_json::json!({ "nodes": [], "origins": [] }))), &mut model);
        let _ = app.update(Event::ChallengesFetched(http(200, serde_json::json!([]))), &mut model);
        assert!(waiting(&model), "sem o catálogo o cabeçalho ainda mudaria depois");

        let mut cmd = app.update(Event::TracksFetched(http(200, serde_json::json!([]))), &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Render(_))));
        let view = app.view(&model);
        assert!(view.has_session && view.has_access_token && !view.is_authenticating);
    }

    /// Sem a trilha, o login não prende ninguém: a falha abre o app com o que houver, e
    /// o prazo abre quando a resposta não vem.
    #[test]
    fn test_login_opens_the_app_when_the_trail_does_not_come() {
        let app = LogNApp::default();

        let mut model = Model::default();
        let _ = login_until_session_stored(&app, &mut model);
        let _ = app.update(Event::NodesFetched(HttpResult::Err(crux_http::HttpError::Io("offline".into()))), &mut model);
        assert!(app.view(&model).has_session, "falha de rede abre o app");

        let mut model = Model::default();
        let _ = login_until_session_stored(&app, &mut model);
        let attempt = model.entering_attempt;
        let _ = app.update(Event::EnterWatchdogElapsed { attempt: attempt - 1 }, &mut model);
        assert!(!app.view(&model).has_session, "o prazo de uma entrada anterior não corta esta");
        let mut cmd = app.update(Event::EnterWatchdogElapsed { attempt }, &mut model);
        assert!(cmd.effects().any(|e| matches!(e, Effect::Render(_))));
        assert!(app.view(&model).has_session, "o prazo acabou: abre com o que há");
    }

    /// Quem já está no app e troca a senha não volta à tela de login esperando a trilha.
    #[test]
    fn test_password_reset_inside_the_app_does_not_leave_the_app() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("old".into());
        model.user_id = USER_B.into();

        let _ = app.update(
            Event::ResetPasswordCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "ref", "user_id": USER_B,
            }))),
            &mut model,
        );
        assert!(!model.entering_session);
        assert!(app.view(&model).has_session);
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
            track_id: String::new(),
            requires_purchase: false,
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
        // O perfil ainda está descendo quando a saída chega: ele desce com o e-mail.
        assert!(model.account_email.is_empty());
        assert_eq!(app.view(&model).account_email, "jogador@example.com");

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
            track_id: String::new(),
            requires_purchase: false,
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
            track_id: String::new(),
            requires_purchase: false,
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
                status: 200, headers, body: br#"{"nodes":[],"origins":[]}"#.to_vec(),
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
                status: 200, headers: vec![], body: br#"{"nodes":[],"origins":[]}"#.to_vec(),
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
        model.origins = vec![crate::domain::OriginCard {
            id: "FARIAS".into(),
            name: "Nome de teste".into(),
            role: "Papel de teste".into(),
            body: "Corpo de teste.".into(),
        }];

        let node = "10000000-0000-0000-0000-000000000001";
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);

        // Primeira leitura: para o relógio e fica registrada.
        let _ = app.update(Event::OpenOriginSheet, &mut model);
        assert_eq!(model.origin_sheet, "FARIAS");
        assert!(model.match_state.as_ref().unwrap().is_paused, "a primeira leitura para o relógio");
        assert_eq!(model.origins_seen, vec!["FARIAS".to_string()]);
        let cartao = app.view(&model).match_view.origin_sheet.expect("cartão achado por id");
        assert_eq!(cartao.name, "Nome de teste", "o ViewModel carrega o cartão, não só o id");

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
        model.origins = vec![crate::domain::OriginCard {
            id: "FARIAS".into(),
            name: "Nome de teste".into(),
            role: "Papel de teste".into(),
            body: "Corpo de teste.".into(),
        }];

        let node = "10000000-0000-0000-0000-000000000001";
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let _ = app.update(Event::OpenOriginSheet, &mut model);

        assert_eq!(model.origin_sheet, "FARIAS");
        assert!(
            !model.match_state.as_ref().unwrap().is_paused,
            "já lida numa sessão anterior: abre sem parar o relógio"
        );
    }

    /// Origem que a semente ou o servidor não conhecem mais — id mudou, conteúdo
    /// atrasado — nunca trava a partida: o cartão abre com o próprio id no lugar do
    /// nome, e o resto vazio.
    #[test]
    fn test_origin_sheet_falls_back_to_the_id_when_the_card_is_unknown() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let mut ch = seeded_challenge(
            "ch_004",
            "COMPLEXITY_MATCH",
            vec!["O(n)".into(), "O(1)".into()],
            vec!["O(n)".into(), "O(1)".into()],
            "Irrelevante para este teste.",
        );
        ch.origin = "DESCONHECIDA".into();
        model.challenges = vec![ch];
        // model.origins fica vazio de propósito: nem servidor nem semente trazem
        // o cartão desta origem.

        let node = "10000000-0000-0000-0000-000000000001";
        let _ = app.update(Event::StartMatch { node_id: node.into() }, &mut model);
        let _ = app.update(Event::OpenOriginSheet, &mut model);

        let cartao = app.view(&model).match_view.origin_sheet.expect("abre mesmo sem cartão conhecido");
        assert_eq!(cartao.name, "DESCONHECIDA", "sem texto conhecido, o id vira o nome");
        assert!(cartao.role.is_empty());
        assert!(cartao.body.is_empty());
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

    /// TRADEOFF_MATCH: o toque preenche a próxima casa, benefício antes de desvantagem,
    /// e o gabarito é [benefício, desvantagem] posição a posição.
    #[test]
    fn test_tradeoff_fills_the_next_slot_and_judges_in_order() {
        let app = LogNApp::default();
        let options = vec!["Opção A".to_string(), "Opção B".into(), "Opção C".into()];
        let answer = vec!["Opção A".to_string(), "Opção B".into()];
        let start = |model: &mut Model| {
            model.challenges = vec![seeded_challenge("ch_t06", "TRADEOFF_MATCH", options.clone(), answer.clone(), "")];
            let _ = app.update(Event::StartMatch { node_id: "10000000-0000-0000-0000-000000000001".into() }, model);
        };

        let mut model = Model::default();
        start(&mut model);
        assert_eq!(model.match_state.as_ref().unwrap().problems[0].seconds, 75);

        let _ = app.update(Event::MatchPickTradeoff { value: "Opção A".into() }, &mut model);
        let _ = app.update(Event::MatchPickTradeoff { value: "Opção A".into() }, &mut model);
        let view = app.view(&model).match_view;
        assert_eq!(view.tradeoff_benefit, "Opção A");
        assert_eq!(view.tradeoff_drawback, "", "a mesma opção não entra nas duas casas");

        let _ = app.update(Event::MatchPickTradeoff { value: "Opção B".into() }, &mut model);
        let _ = app.update(Event::MatchPickTradeoff { value: "Opção C".into() }, &mut model);
        let view = app.view(&model).match_view;
        assert_eq!((view.tradeoff_benefit.as_str(), view.tradeoff_drawback.as_str()), ("Opção A", "Opção B"),
            "com as duas casas cheias, o toque não troca nada");

        // Tirar o benefício deixa a desvantagem e devolve o próximo toque ao benefício.
        let _ = app.update(Event::MatchClearBenefit, &mut model);
        let _ = app.update(Event::MatchPickTradeoff { value: "Opção C".into() }, &mut model);
        let view = app.view(&model).match_view;
        assert_eq!((view.tradeoff_benefit.as_str(), view.tradeoff_drawback.as_str()), ("Opção C", "Opção B"));
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);
        assert_eq!(app.view(&model).match_view.last_verdict, "WA");

        // As duas certas, mas trocadas de casa, é erro.
        let mut model = Model::default();
        start(&mut model);
        let _ = app.update(Event::MatchPickTradeoff { value: "Opção B".into() }, &mut model);
        let _ = app.update(Event::MatchPickTradeoff { value: "Opção A".into() }, &mut model);
        let _ = app.update(Event::MatchSubmit { timestamp: 1_700_000_000 }, &mut model);
        assert_eq!(app.view(&model).match_view.last_verdict, "WA", "trocar benefício e desvantagem é erro");

        let mut model = Model::default();
        start(&mut model);
        let _ = app.update(Event::MatchPickTradeoff { value: "Opção A".into() }, &mut model);
        let _ = app.update(Event::MatchClearDrawback, &mut model);
        let _ = app.update(Event::MatchPickTradeoff { value: "Opção B".into() }, &mut model);
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
        assert_eq!(display_name_from_email("jogador@example.com"), "Jogador");
        assert_eq!(display_name_from_email("jogador.exemplo@example.com"), "Jogador");
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

        let mut cmd = app.update(Event::Login { email: "test@x.com".into(), password: "senha".into() }, &mut model);
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
        assert_eq!(view.boot.progress, 16);
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
        assert_eq!(app.view(&model).boot.progress, 50);

        let mut cmd = app.update(
            Event::SyncCompleted(http(200, serde_json::json!({ "new_top": "abc" }))),
            &mut model,
        );
        // Depois do sync, os termos: a fila subiu antes de o aceite ser perguntado.
        assert!(
            http_requests(&mut cmd).iter().any(|r| r.url == "/api/v1/legal/pending"),
            "com o sync fechado, a abertura pergunta pelos termos"
        );
        let view = app.view(&model);
        assert!(view.boot.in_progress);
        assert_eq!(view.boot.lines[1], boot_line(BootCheck::Sync, BootVerdict::Ok, BootDetail::Sent, 2));
        assert_eq!(view.boot.lines[2], boot_line(BootCheck::Terms, BootVerdict::Running, BootDetail::TermsChecking, 0));

        let _ = app.update(terms_for_a(http(200, serde_json::json!({ "documents": [] }))), &mut model);
        let view = app.view(&model);
        assert!(!view.boot.in_progress, "a última linha fechou: a splash sai");
        assert_eq!(view.boot.lines[2], boot_line(BootCheck::Terms, BootVerdict::Ok, BootDetail::TermsCurrent, 0));
        assert_eq!(view.boot.progress, 100);
        assert!(view.has_session && view.terms_update.is_none());
    }

    #[test]
    fn test_boot_with_an_empty_queue_has_nothing_to_send() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_online(&app, &mut model, USER_A);
        let _ = app.update(Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_empty() }, &mut model);
        let mut cmd = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, &mut model);

        let urls: Vec<String> = http_requests(&mut cmd).into_iter().map(|r| r.url).collect();
        assert_eq!(urls, vec!["/api/v1/legal/pending".to_string()], "fila vazia não vai ao sync; os termos, sim");
        let view = app.view(&model);
        assert_eq!(view.boot.lines[1], boot_line(BootCheck::Sync, BootVerdict::Ok, BootDetail::NothingToSend, 0));
        let _ = app.update(terms_for_a(http(200, serde_json::json!({ "documents": [] }))), &mut model);
        assert!(!app.view(&model).boot.in_progress);
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

    /// A fila longa sobe em lotes de SYNC_BATCH. Mandada inteira, ela passava do teto de
    /// eventos do servidor e não subia nunca mais.
    #[test]
    fn test_a_long_queue_goes_up_in_batches() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = USER_A.into();
        model.queue_owner = USER_A.into();
        model.queue_loaded = true;
        let mut previous = GameEvent::GENESIS.to_string();
        for i in 0..(SYNC_BATCH * 2 + 50) {
            let event = answer(&format!("a{i}"), &previous);
            previous = event.current_hash.clone();
            model.pending_events.push(event);
        }
        model.last_hash = previous.clone();

        let sent_body = |effects: &[Effect]| -> SyncPayload {
            let body = effects.iter().find_map(|e| match e {
                Effect::Http(r) if r.operation.url == "/api/v1/sync" => Some(r.operation.body.clone()),
                _ => None,
            }).expect("um pedido de sync");
            serde_json::from_slice(&body).unwrap()
        };

        let mut cmd = app.update(Event::SyncNow, &mut model);
        let first = sent_body(&cmd.effects().collect::<Vec<_>>());
        assert_eq!(first.events.len(), SYNC_BATCH);
        assert_eq!(first.events[0].id, "a0");

        let top = first.events.last().unwrap().current_hash.clone();
        let mut cmd = app.update(Event::SyncCompleted(http(200, serde_json::json!({ "new_top": top }))), &mut model);
        let second = sent_body(&cmd.effects().collect::<Vec<_>>());
        assert_eq!(second.events.len(), SYNC_BATCH);
        assert_eq!(second.events[0].previous_hash, top, "o lote seguinte continua de onde o servidor parou");

        let top = second.events.last().unwrap().current_hash.clone();
        let mut cmd = app.update(Event::SyncCompleted(http(200, serde_json::json!({ "new_top": top }))), &mut model);
        let third = sent_body(&cmd.effects().collect::<Vec<_>>());
        assert_eq!(third.events.len(), 50);

        let top = third.events.last().unwrap().current_hash.clone();
        let _ = app.update(Event::SyncCompleted(http(200, serde_json::json!({ "new_top": top }))), &mut model);
        assert!(model.pending_events.is_empty());
        assert_eq!(model.last_hash, previous);
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
        assert_eq!(view.boot.lines[1], boot_line(BootCheck::Sync, BootVerdict::Warn, BootDetail::StillSending, 1));
        assert!(model.is_syncing, "o envio segue");
        // Os termos também têm teto: sem resposta, o app entra e confere depois.
        let _ = app.update(Event::BootWatchdogElapsed { check: BootCheck::Terms, attempt: 1 }, &mut model);
        let view = app.view(&model);
        assert!(!view.boot.in_progress);
        assert_eq!(view.boot.lines[2], boot_line(BootCheck::Terms, BootVerdict::Skipped, BootDetail::TermsDeferred, 0));
    }

    /// A resposta dos termos para a conta A, a que as aberturas dos testes usam.
    fn terms_for_a(result: HttpResult) -> Event {
        Event::TermsPendingFetched { owner: USER_A.into(), result }
    }

    fn pending_terms(blocking: bool) -> HttpResult {
        http(200, serde_json::json!({
            "blocking": blocking,
            "documents": [
                { "kind": "terms", "version": 4, "locale": "pt-BR", "effective_at": "2026-11-01",
                  "sha256": "aa", "accepted_version": 2, "accepted_effective_at": "2026-09-28" },
                { "kind": "privacy", "version": 4, "locale": "pt-BR", "effective_at": "2026-11-01",
                  "sha256": "bb", "accepted_version": 2, "accepted_effective_at": "2026-09-28" }
            ],
            "changes": [
                { "id": "terms:3:secao-a", "kind": "terms", "version": 3, "change": "added", "section": "secao-a", "summary": "Mudança A." },
                { "id": "terms:4:secao-b", "kind": "terms", "version": 4, "change": "removed", "section": "secao-b", "summary": "Mudança B." },
                { "id": "privacy:4:secao-c", "kind": "privacy", "version": 4, "change": "changed", "section": "secao-c", "summary": "Mudança C." }
            ]
        }))
    }

    /// Abre até a linha dos termos, com a sessão e a fila vazia.
    fn boot_to_terms(app: &LogNApp, model: &mut Model) {
        boot_online(app, model, USER_A);
        let _ = app.update(Event::OfflineQueueRestored { owner: USER_A.into(), result: kv_empty() }, model);
        let _ = app.update(Event::QueueToAdoptRead { from: String::new(), result: kv_empty() }, model);
        assert!(model.boot.terms_running());
    }

    #[test]
    fn material_terms_block_the_app_and_accepting_sends_the_proof() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::SetClientInfo { app_version: "1.8.0".into(), platform: "ios".into() }, &mut model);
        boot_to_terms(&app, &mut model);

        let _ = app.update(terms_for_a(pending_terms(true)), &mut model);
        let view = app.view(&model);
        assert!(!view.boot.in_progress, "a splash sai, e a tela de aceite fica na frente");
        assert_eq!(view.boot.lines[2].detail, BootDetail::TermsChanged);
        let gate = view.terms_update.expect("versão relevante cobre o app");
        assert_eq!((gate.from_version, gate.from_date.as_str()), (2, "2026-09-28"));
        assert_eq!((gate.to_version, gate.to_date.as_str()), (4, "2026-11-01"));
        assert_eq!(gate.versions_skipped, 2, "as duas versões puladas somam no diff");
        assert_eq!(gate.changes.len(), 3);
        assert_eq!(gate.changes[1].change, crate::domain::TermsChangeKind::Removed);
        assert_eq!(gate.terms_sections, vec!["secao-a".to_string(), "secao-b".to_string()]);
        assert_eq!(gate.privacy_sections, vec!["secao-c".to_string()]);

        let mut cmd = app.update(Event::AcceptTerms, &mut model);
        let reqs = http_requests(&mut cmd);
        assert_eq!(reqs.len(), 1);
        assert_eq!(reqs[0].url, "/api/v1/legal/accept");
        let body = body_of(&reqs[0]);
        assert_eq!(body["source"], "reaccept");
        assert_eq!(body["documents"].as_array().unwrap().len(), 2);
        assert_eq!(body["documents"][0]["sha256"], "aa");
        assert_eq!(body["documents"][0]["from_version"], 2);
        assert_eq!(body["shown_changes"].as_array().unwrap().len(), 3);
        assert_eq!(body["client"]["app"], "1.8.0");
        assert_eq!(body["client"]["platform"], "ios");
        assert!(app.view(&model).terms_update.unwrap().accepting);

        // O segundo toque não manda outro aceite.
        let mut again = app.update(Event::AcceptTerms, &mut model);
        assert!(http_requests(&mut again).is_empty());

        let _ = app.update(Event::TermsAccepted(http(200, serde_json::json!({}))), &mut model);
        assert!(app.view(&model).terms_update.is_none(), "aceito, o app abre");
    }

    #[test]
    fn stale_terms_screen_fetches_again() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_to_terms(&app, &mut model);
        let _ = app.update(terms_for_a(pending_terms(true)), &mut model);
        let _ = app.update(Event::AcceptTerms, &mut model);
        let mut cmd = app.update(Event::TermsAccepted(api_error(409, "legal_version_outdated")), &mut model);
        assert!(http_requests(&mut cmd).iter().any(|r| r.url == "/api/v1/legal/pending"));
        assert!(app.view(&model).terms_update.is_some(), "o bloqueio fica até o servidor dizer outra coisa");

        // Sem rede no aceite: o bloqueio fica, e o erro aparece.
        let _ = app.update(Event::AcceptTerms, &mut model);
        let _ = app.update(Event::TermsAccepted(HttpResult::Err(crux_http::HttpError::Timeout)), &mut model);
        assert!(app.view(&model).terms_update.is_some());
        assert_eq!(model.status_key, StatusKey::NoConnection);
    }

    #[test]
    fn minor_terms_changes_show_a_notice_and_record_it() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_to_terms(&app, &mut model);
        let mut cmd = app.update(terms_for_a(pending_terms(false)), &mut model);
        let reqs = http_requests(&mut cmd);
        assert_eq!(reqs.len(), 1, "o aceite da faixa é gravado quando ela aparece");
        assert_eq!(body_of(&reqs[0])["source"], "notice");
        let view = app.view(&model);
        assert!(!view.boot.in_progress && view.terms_update.is_none() && view.terms_notice);
        assert_eq!(view.boot.lines[2].detail, BootDetail::TermsNotice);

        let _ = app.update(Event::DismissTermsNotice, &mut model);
        assert!(!app.view(&model).terms_notice);
    }

    #[test]
    fn terms_are_deferred_without_a_network_session() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_to_terms(&app, &mut model);
        let _ = app.update(terms_for_a(HttpResult::Err(crux_http::HttpError::Io("offline".into()))), &mut model);
        let view = app.view(&model);
        assert!(!view.boot.in_progress && view.terms_update.is_none());
        assert_eq!(view.boot.lines[2], boot_line(BootCheck::Terms, BootVerdict::Skipped, BootDetail::TermsDeferred, 0));

        // A resposta que chega depois de o app abrir ainda põe o bloqueio na frente.
        let _ = app.update(terms_for_a(pending_terms(true)), &mut model);
        assert!(app.view(&model).terms_update.is_some());
    }

    #[test]
    fn the_terms_block_belongs_to_the_session() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_to_terms(&app, &mut model);
        let _ = app.update(terms_for_a(pending_terms(true)), &mut model);

        // Sem sessão, a tela some (o login vem na frente); o desfazer devolve os dois.
        let token = model.access_token.take();
        assert!(app.view(&model).terms_update.is_none());
        model.access_token = token;
        assert!(app.view(&model).terms_update.is_some());

        // A mesma conta saindo pela tela de aceite e entrando de novo: o bloqueio fica até
        // o servidor responder, e uma resposta que falhe não a solta.
        let mut cmd = app.update(Event::SessionExpiryStored(kv_empty()), &mut model);
        assert!(http_requests(&mut cmd).iter().any(|r| r.url == "/api/v1/legal/pending"));
        let _ = app.update(terms_for_a(HttpResult::Err(crux_http::HttpError::Timeout)), &mut model);
        assert!(app.view(&model).terms_update.is_some(), "busca que falhou não solta o bloqueio");

        // Outra conta entrando neste aparelho busca a dela, sem herdar este bloqueio.
        model.user_id = USER_B.into();
        model.entering_session = true;
        let mut cmd = app.update(Event::SessionExpiryStored(kv_empty()), &mut model);
        assert!(model.terms_pending.is_none());
        assert!(http_requests(&mut cmd).iter().any(|r| r.url == "/api/v1/legal/pending"));

        // A resposta atrasada da conta A não decide o bloqueio da B.
        let _ = app.update(terms_for_a(pending_terms(true)), &mut model);
        assert!(model.terms_pending.is_none(), "resposta de outra conta não bloqueia esta");
    }

    #[test]
    fn a_stale_token_on_accept_renews_and_retries() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_to_terms(&app, &mut model);
        let _ = app.update(terms_for_a(pending_terms(true)), &mut model);
        let _ = app.update(Event::AcceptTerms, &mut model);
        let _ = app.update(Event::TermsAccepted(api_error(401, "unauthenticated")), &mut model);
        assert!(matches!(model.pending_retry_event, Some(Event::AcceptTerms)), "o aceite volta depois do refresh");
        assert!(!model.terms_accepting);
    }

    #[test]
    fn terms_are_deferred_on_an_offline_session() {
        let app = LogNApp::default();
        let mut model = Model::default();
        boot_online(&app, &mut model, USER_A);
        // Sessão aberta sem rede, dentro do prazo: sem token de acesso não há como
        // perguntar, e a linha fecha como adiada.
        model.access_token = None;
        model.boot.sync = None;
        let mut cmd = app.finish_boot(&mut model);
        assert!(http_requests(&mut cmd).is_empty());
        assert_eq!(model.boot.terms, Some(boot_line(BootCheck::Terms, BootVerdict::Skipped, BootDetail::TermsDeferred, 0)));
        assert!(!model.boot.active);
    }
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

        let _ = app.update(Event::Login { email: "b@x.com".into(), password: "senha".into() }, &mut model);
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
            Event::Login { email: "a@x.com".into(), password: "senha-forte".into() },
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

    /// Os bloqueios por e-mail não travam botão: o de login deixa o login social e o
    /// código abertos, e o de código dura um dia, que contagem nenhuma ajuda a esperar.
    #[test]
    fn test_per_email_locks_say_what_happened_without_freezing_the_buttons() {
        let app = LogNApp::default();

        let mut model = Model::default();
        model.is_authenticating = true;
        let _ = app.update(Event::LoginCompleted(api_error(429, "login_locked")), &mut model);
        assert_eq!(model.status_key, StatusKey::LoginLocked);
        assert!(!model.is_authenticating);
        assert_eq!(app.view(&model).auth_cooldown_seconds, 0, "login travado não trava o resto da conta");
        assert_eq!(app.view(&model).resend_cooldown_seconds, 0);

        let mut model = Model::default();
        let _ = app.update(Event::OTPRequested(api_error(429, "otp_locked")), &mut model);
        assert_eq!(model.status_key, StatusKey::CodeLocked);
        assert_eq!(app.view(&model).auth_cooldown_seconds, 0);
        assert_eq!(app.view(&model).resend_cooldown_seconds, 0);
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

    /// O cadastro diz que app gravou o aceite, como o reaceite. Sem plataforma do shell,
    /// o client vai vazio: meio client o servidor recusaria, e o cadastro travaria.
    #[test]
    fn test_signup_bodies_carry_the_client() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.legal_versions = vec![("terms".into(), 1), ("privacy".into(), 1)];
        let register = |model: &mut Model| {
            let mut cmd = app.update(Event::Register {
                email: "a@example.com".into(), password: "senha-forte".into(), otp: "123456".into(),
                age_confirmed: true, legal_accepted: true,
            }, model);
            register_body(&mut cmd)["client"].clone()
        };
        assert_eq!(register(&mut model), serde_json::json!({}));

        let _ = app.update(Event::SetClientInfo { app_version: "0.1.0".into(), platform: "android".into() }, &mut model);
        model.is_authenticating = false;
        assert_eq!(register(&mut model), serde_json::json!({ "app": "0.1.0", "platform": "android" }));

        let cred = SocialCredential { provider: "google".into(), id_token: "t".into(), nonce: "n".into() };
        let body: serde_json::Value = serde_json::from_slice(&social_login_body(&model, &cred, Some((true, true)))).unwrap();
        assert_eq!(body["client"]["platform"], "android");
        let first: serde_json::Value = serde_json::from_slice(&social_login_body(&model, &cred, None)).unwrap();
        assert!(first.get("client").is_none(), "a primeira tentativa não é cadastro");

        // Versão que o servidor recusaria vai vazia: a plataforma ainda é gravada.
        let _ = app.update(Event::SetClientInfo { app_version: "0.1.0 (build_7)".into(), platform: "android".into() }, &mut model);
        model.is_authenticating = false;
        assert_eq!(register(&mut model), serde_json::json!({ "app": "", "platform": "android" }));
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

        let mut cmd = app.update(Event::DeleteAccount { password: "senha-forte".into() }, &mut model);
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

    // --- Trilha paga. "Trilha T" é inventada: a amostra "Nó A" e o fechado "Nó B". ---

    const TRACK: &str = "aaaaaaaa-0000-4000-8000-000000000001";
    const FREE_NODE: &str = "10000000-0000-0000-0000-000000000001";
    const SAMPLE_NODE: &str = "aaaaaaaa-0000-4000-8000-0000000000a1";
    const CLOSED_NODE: &str = "aaaaaaaa-0000-4000-8000-0000000000b1";
    const DEVICE: &str = "dddddddd-0000-4000-8000-000000000001";
    const NOW: i64 = 1_800_000_000;

    fn tree_node(id: &str, track: &str, requires_purchase: bool, required_xp: i32) -> crate::domain::SkillNode {
        crate::domain::SkillNode {
            id: id.into(),
            name: "Nó".into(),
            description: String::new(),
            row: 0,
            column: 0,
            required_xp,
            prerequisites: vec![],
            track_id: track.into(),
            requires_purchase,
            topic: String::new(),
            status: Default::default(),
            problems_solved: vec![],
        }
    }

    fn on_node(id: &str, node: &str) -> Challenge {
        let mut c = seeded_challenge(id, "SPOT_THE_BUG", vec![], vec![], "e");
        c.node_id = node.into();
        c
    }

    fn paid_model() -> (LogNApp, Model) {
        let mut model = Model::default();
        model.access_token = Some("tok".into());
        model.user_id = USER_A.into();
        model.device_id = DEVICE.into();
        model.now = NOW;
        model.locale = "pt-BR".into();
        model.nodes = vec![
            tree_node(FREE_NODE, "free", false, 0),
            tree_node(SAMPLE_NODE, TRACK, false, 0),
            tree_node(CLOSED_NODE, TRACK, true, 50),
        ];
        model.challenges = vec![on_node("ch_t01", FREE_NODE), on_node("ch_t02", SAMPLE_NODE)];
        model.tracks = vec![crate::domain::Track {
            id: TRACK.into(),
            slug: "t".into(),
            product_id: "com.example.logn.track.t".into(),
            name: "Trilha T".into(),
            content_version: 1,
            ..Default::default()
        }];
        (LogNApp::default(), model)
    }

    fn test_license() -> crate::domain::TrackLicense {
        crate::domain::TrackLicense {
            track_id: TRACK.into(),
            key_hex: "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff".into(),
            content_version: 1,
            issued_at: NOW - 10,
            valid_until: NOW + 30 * 86_400,
        }
    }

    fn test_package() -> Vec<u8> {
        let content = serde_json::json!({
            "track_id": TRACK,
            "content_version": 1,
            "challenges": { "pt-BR": [on_node("ch_t03", CLOSED_NODE)], "en": [], "es": [] },
        });
        crate::tracks::testing::seal_package(&test_license(), &content.to_string())
    }

    /// O estado do nó na árvore da trilha dele: a árvore mostra uma trilha por vez.
    fn status_of(model: &Model, node: &str) -> crate::domain::NodeStatus {
        let mut model = model.clone();
        model.selected_track = model.nodes.iter().find(|n| n.id == node).unwrap().track_id.clone();
        LogNApp::default().view(&model).nodes.into_iter().find(|n| n.id == node).unwrap().status
    }

    fn http_requests(cmd: &mut Command<Effect, Event>) -> Vec<HttpRequest> {
        drain(cmd).1
    }

    /// Cofre e HTTP de uma vez: `effects()` esvazia o comando.
    fn drain(cmd: &mut Command<Effect, Event>) -> (Vec<KeyValueOperation>, Vec<HttpRequest>) {
        let (mut kv, mut http) = (vec![], vec![]);
        for e in cmd.effects() {
            match e {
                Effect::SecureStore(r) => kv.push(r.operation.clone()),
                Effect::Http(r) => http.push(r.operation.clone()),
                _ => {}
            }
        }
        (kv, http)
    }

    fn set_keys(ops: &[KeyValueOperation]) -> Vec<String> {
        ops.iter()
            .filter_map(|op| match op {
                KeyValueOperation::Set { key, .. } => Some(key.clone()),
                _ => None,
            })
            .collect()
    }

    /// Licença e pacote já no modelo, como depois de comprar e baixar.
    fn licensed(app: &LogNApp, model: &mut Model) {
        let _ = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(200, serde_json::to_value(test_license()).unwrap()) },
            model,
        );
        let _ = app.update(
            Event::PackageFetched {
                user_id: USER_A.into(),
                track_id: TRACK.into(),
                result: HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body: test_package() }),
            },
            model,
        );
    }

    #[test]
    fn test_a_paid_node_opens_only_with_a_valid_license() {
        use crate::domain::NodeStatus;
        let (_, mut model) = paid_model();
        // O fechado depende da amostra e tem portão zero, como na trilha de verdade.
        model.nodes[2].prerequisites = vec![SAMPLE_NODE.into()];
        model.nodes[2].required_xp = 0;
        assert_eq!(status_of(&model, CLOSED_NODE), NodeStatus::PaywallLocked, "sem compra");
        assert_eq!(status_of(&model, SAMPLE_NODE), NodeStatus::Active,
            "a amostra abre para todos, e o filho no paywall não a conta como conquistada");
        model.nodes[2].required_xp = 50;

        model.licenses.insert(TRACK.into(), test_license());
        assert_eq!(status_of(&model, CLOSED_NODE), NodeStatus::Locked, "comprar não pula o portão");
        model.paid_track_xp.insert(TRACK.into(), 50);
        assert_eq!(status_of(&model, CLOSED_NODE), NodeStatus::Active);

        model.now = test_license().valid_until;
        assert_eq!(status_of(&model, CLOSED_NODE), NodeStatus::PaywallLocked, "31º dia sem rede");
        model.now = test_license().issued_at - crate::tracks::LICENSE_CLOCK_SKEW_SECS - 1;
        assert_eq!(status_of(&model, CLOSED_NODE), NodeStatus::PaywallLocked, "relógio atrasado de propósito");
    }

    /// O XP de uma trilha paga não destrava nó da gratuita, e vice-versa (critério 7).
    #[test]
    fn test_the_gate_counts_only_the_xp_of_the_node_track() {
        use crate::domain::NodeStatus;
        let (_, mut model) = paid_model();
        model.nodes.push(tree_node("10000000-0000-0000-0000-000000000002", "free", false, 100));
        model.licenses.insert(TRACK.into(), test_license());
        let free_gate = "10000000-0000-0000-0000-000000000002";

        model.global_xp = 100;
        assert_eq!(status_of(&model, free_gate), NodeStatus::Active);
        assert_eq!(status_of(&model, CLOSED_NODE), NodeStatus::Locked, "XP da gratuita não abre a paga");

        model.paid_track_xp.insert(TRACK.into(), 60);
        assert_eq!(status_of(&model, CLOSED_NODE), NodeStatus::Active);
        assert_eq!(status_of(&model, free_gate), NodeStatus::Locked, "XP da paga não abre a gratuita");
        assert_eq!(LogNApp::default().view(&model).global_xp, 100, "o nível segue somando os dois");
    }

    #[test]
    fn test_an_accepted_answer_credits_the_track_of_its_node() {
        let (_, mut model) = paid_model();
        credit_track_xp(&model.nodes, &mut model.paid_track_xp, CLOSED_NODE);
        credit_track_xp(&model.nodes, &mut model.paid_track_xp, SAMPLE_NODE);
        credit_track_xp(&model.nodes, &mut model.paid_track_xp, FREE_NODE);
        assert_eq!(model.paid_track_xp.get(TRACK), Some(&(2 * match_engine::XP_PER_ACCEPTED)));
        assert_eq!(model.paid_track_xp.len(), 1, "a gratuita não entra no mapa");
    }

    #[test]
    fn test_a_confirmed_purchase_is_finished_and_licensed() {
        let (app, mut model) = paid_model();
        let mut cmd = app.update(
            Event::SubmitPurchase {
                jws: "signed".into(),
                transaction_id: "tx1".into(),
                product_id: "com.example.logn.track.t".into(),
                restore: false,
                provider: String::new(),
                purchase_token: String::new(),
            },
            &mut model,
        );
        let sent = http_requests(&mut cmd);
        assert_eq!(sent.len(), 1);
        assert_eq!((sent[0].method.as_str(), sent[0].url.as_str()), ("POST", "/api/v1/purchases"));
        assert_eq!(serde_json::from_slice::<serde_json::Value>(&sent[0].body).unwrap()["jws"], "signed");
        assert!(model.purchase_in_flight);
        assert!(model.purchases_to_finish.is_empty(), "a loja só finaliza depois do servidor");

        let mut cmd = app.update(
            Event::PurchaseSubmitted {
                jws: "signed".into(),
                transaction_id: "tx1".into(),
                product_id: "com.example.logn.track.t".into(),
                restore: false,
                provider: String::new(),
                purchase_token: String::new(),
                result: http(200, serde_json::json!({ "track_id": TRACK })),
            },
            &mut model,
        );
        assert_eq!(model.purchases_to_finish, vec!["tx1".to_string()]);
        assert_eq!(model.status_key, StatusKey::PurchaseConfirmed);
        let license = http_requests(&mut cmd);
        assert_eq!(license[0].url, format!("/api/v1/tracks/{TRACK}/license"));
        assert!(license[0].headers.iter().any(|h| h.name == "X-Device-ID" && h.value == DEVICE),
            "todo pedido de licença leva o aparelho");

        let _ = app.update(Event::PurchaseFinished { transaction_id: "tx1".into() }, &mut model);
        assert!(model.purchases_to_finish.is_empty());
    }

    #[test]
    fn test_a_refused_purchase_stops_coming_back_and_a_failed_one_retries() {
        let (app, mut model) = paid_model();
        let submitted = |tx: &str, restore: bool, result: HttpResult| Event::PurchaseSubmitted {
            jws: "signed".into(),
            transaction_id: tx.into(),
            product_id: "com.example.logn.track.t".into(),
            restore,
            provider: String::new(),
            purchase_token: String::new(),
            result,
        };

        let _ = app.update(submitted("tx1", false, http(409, serde_json::json!({ "code": "purchase_owned_by_other_account" }))), &mut model);
        assert_eq!(model.status_key, StatusKey::PurchaseOwnedByOtherAccount);
        assert_eq!(model.purchases_to_finish, vec!["tx1".to_string()], "recusa definitiva fecha a transação");

        let _ = app.update(submitted("tx2", false, http(500, serde_json::json!({ "code": "internal" }))), &mut model);
        assert_eq!(model.status_key, StatusKey::PurchaseFailed);
        assert!(!model.purchases_to_finish.contains(&"tx2".to_string()), "erro do servidor tenta de novo");

        let _ = app.update(submitted("tx3", true, http(200, serde_json::json!({ "track_id": TRACK }))), &mut model);
        assert!(!model.purchases_to_finish.contains(&"tx3".to_string()), "restauração já foi finalizada");
    }

    /// Comprou em outro aparelho: o catálogo diz que é da conta, e a licença vem sozinha.
    #[test]
    fn test_a_track_bought_elsewhere_downloads_after_login() {
        let (app, mut model) = paid_model();
        let mut owned = serde_json::to_value(crate::domain::Track {
            id: TRACK.into(), slug: "t".into(), product_id: "p".into(), name: "Trilha T".into(), ..Default::default()
        }).unwrap();
        owned["owned"] = serde_json::Value::Bool(true);
        // Antes de o índice do aparelho ser lido, o catálogo não pede: a leitura pede.
        let mut cmd = app.update(Event::TracksFetched(http(200, serde_json::json!([owned]))), &mut model);
        assert!(http_requests(&mut cmd).is_empty(), "sem índice lido, não baixa em dobro");
        let mut cmd = app.update(Event::TrackIndexRead { user_id: USER_A.into(), result: kv_empty() }, &mut model);
        assert!(http_requests(&mut cmd).iter().any(|r| r.url == format!("/api/v1/tracks/{TRACK}/license")));

        model.track_index.clear();
        let mut cmd = app.update(Event::TracksFetched(http(200, serde_json::json!([owned]))), &mut model);
        assert!(http_requests(&mut cmd).iter().any(|r| r.url == format!("/api/v1/tracks/{TRACK}/license")),
            "com índice lido, o catálogo pede o que falta");

        // Já baixada: a revalidação é da abertura, não do catálogo.
        model.track_index.push(TRACK.into());
        let mut cmd = app.update(Event::TracksFetched(http(200, serde_json::json!([owned]))), &mut model);
        assert!(http_requests(&mut cmd).is_empty());
    }

    /// O catálogo do teste: a principal e a Trilha T.
    fn with_catalog(model: &mut Model) {
        model.tracks = vec![
            crate::domain::Track { id: "free".into(), kind: "free".into(), name: "Problem Solving".into(), ..Default::default() },
            crate::domain::Track {
                id: TRACK.into(),
                kind: "paid".into(),
                product_id: "com.example.logn.track.t".into(),
                name: "Trilha T".into(),
                node_count: 2,
                problem_count: 5,
                ..Default::default()
            },
        ];
    }

    /// A árvore mostra uma trilha por vez: a principal até o jogador escolher outra.
    #[test]
    fn test_the_tree_shows_one_track_at_a_time() {
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        let ids = |m: &Model| LogNApp::default().view(m).nodes.into_iter().map(|n| n.id).collect::<Vec<_>>();

        assert_eq!(ids(&model), vec![FREE_NODE.to_string()]);
        assert!(app.view(&model).current_track.is_free);

        let mut cmd = app.update(Event::SelectTrack { track_id: TRACK.into() }, &mut model);
        assert!(set_keys(&kv_ops(&mut cmd)).contains(&"selected_track".to_string()), "a escolha fica no aparelho");
        assert_eq!(ids(&model), vec![SAMPLE_NODE.to_string(), CLOSED_NODE.to_string()]);
        assert_eq!(app.view(&model).current_track.name, "Trilha T");

        // Trilha que o app não conhece mais (saiu do catálogo e da árvore): volta à principal.
        model.selected_track = "gone".into();
        assert_eq!(ids(&model), vec![FREE_NODE.to_string()]);
    }

    #[test]
    fn test_the_offline_ladder_counts_days_left() {
        use crate::domain::OfflineState;
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        let state = |m: &Model| {
            let v = LogNApp::default().view(m);
            let t = v.tracks.into_iter().find(|t| t.id == TRACK).unwrap();
            (t.offline, t.offline_days_left, t.days_since_contact)
        };
        assert_eq!(state(&model).0, OfflineState::NoLicense);

        let mut license = test_license();
        license.issued_at = NOW - 26 * 86_400;
        license.valid_until = NOW + 4 * 86_400;
        model.licenses.insert(TRACK.into(), license.clone());
        assert_eq!(state(&model), (OfflineState::Silent, 4, 26));

        model.now = NOW + 86_400;
        assert_eq!(state(&model), (OfflineState::Soon, 3, 27));
        model.now = NOW + 3 * 86_400 + 60;
        assert_eq!(state(&model).0, OfflineState::Today);
        model.now = license.valid_until;
        assert_eq!(state(&model).0, OfflineState::Expired);
        let _ = app;
    }

    /// O selo de "N dias" some no dia em que o catálogo abriu; "hoje" não some.
    #[test]
    fn test_the_catalog_badge_quiets_down_for_the_day() {
        use crate::domain::OfflineState;
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        let mut license = test_license();
        license.valid_until = NOW + 2 * 86_400 + 10;
        model.licenses.insert(TRACK.into(), license);

        assert_eq!(app.view(&model).catalog_badge.state, OfflineState::Soon);
        assert_eq!(app.view(&model).catalog_badge.days_left, 2);
        let _ = app.update(Event::CatalogOpened, &mut model);
        assert_eq!(app.view(&model).catalog_badge.state, OfflineState::NoLicense, "visto hoje");

        model.now = NOW + 2 * 86_400;
        assert_eq!(app.view(&model).catalog_badge.state, OfflineState::Today, "último dia não some");
    }

    #[test]
    fn test_the_sample_offer_follows_a_mastered_sample() {
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        model.nodes[2].prerequisites = vec![SAMPLE_NODE.into()];
        let _ = app.update(Event::StartMatch { node_id: SAMPLE_NODE.into() }, &mut model);
        model.match_state.as_mut().unwrap().is_active = false;

        assert!(!app.view(&model).sample_offer.active, "sem dominar a amostra, sem oferta");
        model.paid_challenges.push("ch_t02".into());
        let offer = app.view(&model).sample_offer;
        assert!(offer.active);
        assert_eq!((offer.next_node_index, offer.remaining_nodes, offer.remaining_problems), (2, 1, 4));

        let _ = app.update(Event::DismissSampleOffer { track_id: TRACK.into() }, &mut model);
        assert!(!app.view(&model).sample_offer.active, "\"Agora não\" vale pela sessão");

        model.offer_dismissed.clear();
        model.tracks[1].owned = true;
        assert!(!app.view(&model).sample_offer.active, "quem comprou não vê oferta");
    }

    /// F3: a compra anda pelos passos até a trilha abrir sem rede.
    #[test]
    fn test_the_purchase_flow_walks_its_steps() {
        use crate::domain::PurchaseStage;
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        let stage = |m: &Model| LogNApp::default().view(m).purchase_flow.stage;

        // Reentregue pela loja na abertura, sem o jogador pedir: segue em silêncio.
        let _ = app.update(apple_submit("tx0", false), &mut model);
        assert_eq!(stage(&model), PurchaseStage::Idle);

        let _ = app.update(Event::PurchaseIntent { product_id: "com.example.logn.track.t".into() }, &mut model);
        let _ = app.update(apple_submit("tx1", false), &mut model);
        assert_eq!(stage(&model), PurchaseStage::Validating);
        assert_eq!(app.view(&model).purchase_flow.track_id, TRACK);

        let _ = app.update(apple_submitted("tx1", false, http(200, serde_json::json!({ "track_id": TRACK }))), &mut model);
        assert_eq!(stage(&model), PurchaseStage::Licensing);
        let _ = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(200, serde_json::to_value(test_license()).unwrap()) },
            &mut model,
        );
        assert_eq!(stage(&model), PurchaseStage::Downloading);
        let _ = app.update(
            Event::PackageFetched {
                user_id: USER_A.into(),
                track_id: TRACK.into(),
                result: HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body: test_package() }),
            },
            &mut model,
        );
        assert_eq!(stage(&model), PurchaseStage::Ready);

        let _ = app.update(Event::ClosePurchaseFlow, &mut model);
        assert_eq!(stage(&model), PurchaseStage::Idle);
        assert_eq!(model.selected_track, TRACK, "\"Abrir\" leva à trilha comprada");

        // Recusa: o passo a passo para, com o motivo.
        let _ = app.update(Event::PurchaseIntent { product_id: "com.example.logn.track.t".into() }, &mut model);
        let _ = app.update(apple_submit("tx2", false), &mut model);
        let _ = app.update(apple_submitted("tx2", false, http(403, serde_json::json!({ "code": "purchase_revoked" }))), &mut model);
        let flow = app.view(&model).purchase_flow;
        assert_eq!((flow.stage, flow.failure), (PurchaseStage::Failed, StatusKey::PurchaseRevoked));
    }

    /// A compra da App Store como o iOS a manda: prova no `jws`, sem `provider`.
    fn apple_submit(tx: &str, restore: bool) -> Event {
        Event::SubmitPurchase {
            jws: "j".into(),
            transaction_id: tx.into(),
            product_id: "com.example.logn.track.t".into(),
            restore,
            provider: String::new(),
            purchase_token: String::new(),
        }
    }

    fn apple_submitted(tx: &str, restore: bool, result: HttpResult) -> Event {
        Event::PurchaseSubmitted {
            jws: "j".into(),
            transaction_id: tx.into(),
            product_id: "com.example.logn.track.t".into(),
            restore,
            provider: String::new(),
            purchase_token: String::new(),
            result,
        }
    }

    /// A compra do Google Play como o Android a manda: o token é a prova e o id.
    fn play_submit(token: &str, restore: bool) -> Event {
        Event::SubmitPurchase {
            jws: String::new(),
            transaction_id: token.into(),
            product_id: "com.example.logn.track.t".into(),
            restore,
            provider: PROVIDER_GOOGLE_PLAY.into(),
            purchase_token: token.into(),
        }
    }

    fn play_submitted(token: &str, restore: bool, result: HttpResult) -> Event {
        Event::PurchaseSubmitted {
            jws: String::new(),
            transaction_id: token.into(),
            product_id: "com.example.logn.track.t".into(),
            restore,
            provider: PROVIDER_GOOGLE_PLAY.into(),
            purchase_token: token.into(),
            result,
        }
    }

    #[test]
    fn test_a_play_purchase_sends_the_token_and_the_product() {
        let (app, mut model) = paid_model();
        let mut cmd = app.update(play_submit("tok-1", false), &mut model);
        let sent = http_requests(&mut cmd);
        assert_eq!(sent[0].url, "/api/v1/purchases");
        let body = serde_json::from_slice::<serde_json::Value>(&sent[0].body).unwrap();
        assert_eq!(body["provider"], "google_play");
        assert_eq!(body["product_id"], "com.example.logn.track.t");
        assert_eq!(body["purchase_token"], "tok-1");
        assert!(body.get("jws").is_none(), "a prova do Play é o token, não um JWS");

        let _ = app.update(play_submitted("tok-1", false, http(200, serde_json::json!({ "track_id": TRACK }))), &mut model);
        assert_eq!(model.purchases_to_finish, vec!["tok-1".to_string()]);
        assert_eq!(model.status_key, StatusKey::PurchaseConfirmed);
    }

    #[test]
    fn test_a_pending_play_payment_stays_open_and_is_not_another_account() {
        let (app, mut model) = paid_model();
        let _ = app.update(Event::PurchaseIntent { product_id: "com.example.logn.track.t".into() }, &mut model);
        let _ = app.update(play_submit("tok-2", false), &mut model);
        let _ = app.update(play_submitted("tok-2", false, http(409, serde_json::json!({ "code": "purchase_pending" }))), &mut model);
        assert_eq!(model.status_key, StatusKey::PurchasePending);
        assert!(model.purchases_to_finish.is_empty(), "pendente não é recusa: a loja entrega de novo quando pagar");
        let flow = app.view(&model).purchase_flow;
        assert_eq!(flow.failure, StatusKey::PurchasePending);

        // Na restauração, o pendente não conta como compra de outra conta.
        let _ = app.update(Event::RestoreStarted { count: 1 }, &mut model);
        let _ = app.update(play_submitted("tok-2", true, http(409, serde_json::json!({ "code": "purchase_pending" }))), &mut model);
        assert_eq!(model.restore.other_account, 0);
    }

    #[test]
    fn test_store_unavailable_keeps_the_purchase_for_a_retry() {
        let (app, mut model) = paid_model();
        let _ = app.update(play_submit("tok-3", false), &mut model);
        let _ = app.update(play_submitted("tok-3", false, http(503, serde_json::json!({ "code": "store_unavailable" }))), &mut model);
        assert_eq!(model.status_key, StatusKey::StoreUnavailable);
        assert!(model.purchases_to_finish.is_empty());
    }

    #[test]
    fn test_a_play_purchase_retried_after_401_keeps_its_token() {
        let (app, mut model) = paid_model();
        let _ = app.update(play_submit("tok-4", false), &mut model);
        let _ = app.update(play_submitted("tok-4", false, http(401, serde_json::json!({}))), &mut model);
        match model.purchase_retries.first() {
            Some(Event::SubmitPurchase { provider, purchase_token, .. }) => {
                assert_eq!((provider.as_str(), purchase_token.as_str()), ("google_play", "tok-4"));
            }
            other => panic!("esperava a compra do Play na fila de repetição, veio {other:?}"),
        }
    }

    /// A rede cai depois da cobrança: a tela para com o motivo, em vez de "Aguarde" para
    /// sempre, e a compra sem pedido na tela não trava nada.
    #[test]
    fn test_the_purchase_flow_stops_when_the_download_fails() {
        use crate::domain::PurchaseStage;
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        model.purchase_flow = Some((TRACK.into(), PurchaseStage::Licensing, StatusKey::Silent));
        let _ = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: HttpResult::Err(crux_http::HttpError::Io("offline".into())) },
            &mut model,
        );
        let flow = app.view(&model).purchase_flow;
        assert_eq!((flow.stage, flow.failure), (PurchaseStage::Failed, StatusKey::TrackDownloadFailed));

        model.purchase_flow = Some((TRACK.into(), PurchaseStage::Downloading, StatusKey::Silent));
        model.licenses.insert(TRACK.into(), test_license());
        let _ = app.update(
            Event::PackageFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(500, serde_json::json!({})) },
            &mut model,
        );
        assert_eq!(app.view(&model).purchase_flow.stage, PurchaseStage::Failed);

        // Fechar antes de pronta: some a tela, a compra segue em segundo plano.
        model.purchase_flow = Some((TRACK.into(), PurchaseStage::Downloading, StatusKey::Silent));
        let _ = app.update(Event::ClosePurchaseFlow, &mut model);
        assert_eq!(app.view(&model).purchase_flow.stage, PurchaseStage::Idle);
    }

    /// A restauração manda várias compras de uma vez; com o token vencido, cada uma volta
    /// com 401. Um refresh só — dois com o mesmo token derrubariam a conta — e todas
    /// voltam a ser mandadas depois dele.
    #[test]
    fn test_concurrent_401s_refresh_once_and_retry_all() {
        let (app, mut model) = paid_model();
        let submitted = |tx: &str| Event::PurchaseSubmitted {
            jws: "j".into(),
            transaction_id: tx.into(),
            product_id: String::new(),
            restore: true,
            provider: String::new(),
            purchase_token: String::new(),
            result: http(401, serde_json::json!({ "code": "unauthenticated" })),
        };
        let _ = app.update(submitted("tx1"), &mut model);
        let _ = app.update(submitted("tx2"), &mut model);
        assert_eq!(model.purchase_retries.len(), 2);

        let token = kv_bytes(b"refresh".to_vec());
        let mut first = app.update(Event::TokenRead(token.clone()), &mut model);
        assert!(http_requests(&mut first).iter().any(|r| r.url == "/api/v1/auth/refresh"));
        let mut second = app.update(Event::TokenRead(token), &mut model);
        assert!(http_requests(&mut second).is_empty(), "um refresh por vez");

        let mut cmd = app.update(Event::AccountEmailRead(kv_empty()), &mut model);
        let resent = http_requests(&mut cmd).iter().filter(|r| r.url == "/api/v1/purchases/restore").count();
        assert_eq!(resent, 2, "as duas compras voltam a ser mandadas");
    }

    /// O refresh voltou com token rodado, mas o cofre ainda não gravou: um 401 atrasado
    /// nessa janela leria o token velho, e mandá-lo é reuso, que derruba a conta.
    #[test]
    fn test_a_late_401_does_not_refresh_before_the_rotated_token_is_stored() {
        let (app, mut model) = paid_model();
        let _ = app.update(Event::TokenRead(kv_bytes(b"velho".to_vec())), &mut model);
        let _ = app.update(
            Event::RefreshCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "rodado", "user_id": USER_A,
                "refresh_expires_at": 1_792_600_000i64,
            }))),
            &mut model,
        );
        assert!(model.refresh_in_flight, "segue no ar até o cofre gravar");

        // O 401 de um pedido que saiu antes: entra na fila e pede refresh.
        let _ = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(401, serde_json::json!({})) },
            &mut model,
        );
        let mut cmd = app.update(Event::TokenRead(kv_bytes(b"velho".to_vec())), &mut model);
        assert!(http_requests(&mut cmd).is_empty(), "o token velho não sai enquanto o novo não está no cofre");

        let stored = KeyValueResult::Ok { response: KeyValueResponse::Set { previous: crux_kv::Value::None } };
        let mut cmd = app.update(Event::RotatedTokenStored(stored), &mut model);
        assert!(!model.refresh_in_flight);
        assert_ne!(model.status_key, StatusKey::ResumingSession, "o pedido dispensado não deixa o status preso");
        let _ = drain(&mut cmd);
        let mut cmd = app.update(Event::AttemptRefreshDone, &mut model);
        let _ = drain(&mut cmd);
        let mut cmd = app.update(Event::AccountEmailRead(kv_empty()), &mut model);
        assert!(
            http_requests(&mut cmd).iter().any(|r| r.url == format!("/api/v1/tracks/{TRACK}/license")),
            "o pedido da fila sai com o token novo"
        );
    }

    /// Tentar de novo a abertura dentro da janela não manda o token velho e não deixa a
    /// splash esperando o watchdog: a linha da sessão fecha quando o cofre grava.
    #[test]
    fn test_a_boot_retry_inside_the_rotation_window_closes_when_the_token_is_stored() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::Tick { now: 1_790_000_000 }, &mut model);
        boot_online(&app, &mut model, USER_A);
        let mut cmd = app.update(Event::StartBoot, &mut model);
        assert!(!kv_ops(&mut cmd).iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "refresh_token")));
        assert!(model.boot.session_running());

        let stored = KeyValueResult::Ok { response: KeyValueResponse::Set { previous: crux_kv::Value::None } };
        let _ = app.update(Event::RotatedTokenStored(stored), &mut model);
        assert!(!model.boot.session_running(), "a linha da sessão fechou");
    }

    #[test]
    fn test_signing_out_forgets_what_the_catalog_said_about_the_account() {
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        model.tracks[1].owned = true;
        model.tracks[1].revoked_reason = "account_sharing".into();
        model.selected_track = TRACK.into();
        let _ = app.update(
            Event::TokenCleared(KeyValueResult::Ok { response: KeyValueResponse::Delete { previous: crux_kv::Value::None } }),
            &mut model,
        );
        assert!(!model.tracks[1].owned && model.tracks[1].revoked_reason.is_empty());
        assert!(model.selected_track.is_empty());
    }

    #[test]
    fn test_restoring_nothing_says_so() {
        let (app, mut model) = paid_model();
        let _ = app.update(Event::RestoreStarted { count: 0 }, &mut model);
        let result = app.view(&model).restore_result;
        assert!(result.active && result.finished && result.total == 0);
    }

    #[test]
    fn test_restore_reports_what_came_back() {
        let (app, mut model) = paid_model();
        with_catalog(&mut model);
        let _ = app.update(Event::RestoreStarted { count: 2 }, &mut model);
        let submitted = |tx: &str, result: HttpResult| Event::PurchaseSubmitted {
            jws: "j".into(),
            transaction_id: tx.into(),
            product_id: String::new(),
            restore: true,
            provider: String::new(),
            purchase_token: String::new(),
            result,
        };
        let _ = app.update(submitted("tx1", http(200, serde_json::json!({ "track_id": TRACK }))), &mut model);
        assert!(!app.view(&model).restore_result.finished);
        let _ = app.update(submitted("tx2", http(409, serde_json::json!({ "code": "purchase_owned_by_other_account" }))), &mut model);

        let result = app.view(&model).restore_result;
        assert!(result.active && result.finished);
        assert_eq!((result.total, result.restored, result.other_account), (2, 1, 1));
        assert_eq!(result.restored_names, vec!["Trilha T".to_string()]);
        assert!(model.purchases_to_finish.is_empty(), "restauração não finaliza nada na loja");

        let _ = app.update(Event::DismissRestoreResult, &mut model);
        assert!(!app.view(&model).restore_result.active);
    }

    #[test]
    fn test_onboarding_shows_once_per_device() {
        let (app, mut model) = paid_model();
        assert!(!app.view(&model).show_onboarding, "antes de ler as preferências, não");

        let restored = |key: &str, value: &[u8]| Event::PreferenceRestored { key: key.into(), result: kv_bytes(value.to_vec()) };
        let _ = app.update(restored("selected_track", TRACK.as_bytes()), &mut model);
        let _ = app.update(Event::PreferenceRestored { key: "onboarding_done".into(), result: kv_empty() }, &mut model);
        assert_eq!(model.selected_track, TRACK, "a trilha escolhida volta");
        assert!(app.view(&model).show_onboarding);

        let mut cmd = app.update(Event::CompleteOnboarding, &mut model);
        assert!(set_keys(&kv_ops(&mut cmd)).contains(&"onboarding_done".to_string()));
        assert!(!app.view(&model).show_onboarding);

        let (app, mut model) = paid_model();
        let _ = app.update(restored("onboarding_done", b"1"), &mut model);
        assert!(!app.view(&model).show_onboarding, "quem já passou não vê de novo");
    }

    #[test]
    fn test_playing_as_guest_shows_the_game_right_away() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(Event::ContinueAsGuest, &mut model);
        let effects: Vec<_> = cmd.effects().collect();
        // A tela muda no toque, sem esperar a árvore chegar do servidor.
        assert!(effects.iter().any(|e| matches!(e, Effect::Render(_))), "o toque tem de chegar à tela");
        assert!(effects.iter().any(|e| matches!(e, Effect::Http(r) if r.operation.url == "/api/v1/nodes")));
        let view = app.view(&model);
        assert!(view.is_guest && view.is_fetching, "o mapa mostra que está atualizando");
    }

    #[test]
    fn test_a_guest_does_not_buy() {
        let (app, mut model) = paid_model();
        model.access_token = None;
        model.is_guest = true;
        let mut cmd = app.update(
            Event::SubmitPurchase {
                jws: "signed".into(),
                transaction_id: "tx1".into(),
                product_id: String::new(),
                restore: false,
                provider: String::new(),
                purchase_token: String::new(),
            },
            &mut model,
        );
        assert!(http_requests(&mut cmd).is_empty());
        assert_eq!(model.status_key, StatusKey::PurchaseNeedsAccount);
    }

    #[test]
    fn test_the_license_goes_to_the_keychain_and_the_package_opens() {
        let (app, mut model) = paid_model();
        let mut cmd = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(200, serde_json::to_value(test_license()).unwrap()) },
            &mut model,
        );
        let (kv, sent) = drain(&mut cmd);
        let keys = set_keys(&kv);
        assert!(keys.contains(&format!("track_key:{USER_A}:{TRACK}")), "{keys:?}");
        assert!(keys.contains(&format!("track_index:{USER_A}")));
        assert!(sent.iter().any(|r| r.url == format!("/api/v1/tracks/{TRACK}/package")));

        let mut cmd = app.update(
            Event::PackageFetched {
                user_id: USER_A.into(),
                track_id: TRACK.into(),
                result: HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body: test_package() }),
            },
            &mut model,
        );
        assert!(set_keys(&kv_ops(&mut cmd)).contains(&format!("track_package:{USER_A}:{TRACK}")));
        assert!(model.challenges.iter().any(|c| c.id == "ch_t03"), "o fechado entra na trilha");
        assert_ne!(status_of(&model, CLOSED_NODE), crate::domain::NodeStatus::PaywallLocked);

        let view = app.view(&model);
        let track = &view.tracks[0];
        assert!(track.owned && track.downloaded);
        assert_eq!(track.offline, crate::domain::OfflineState::Silent);
        assert_eq!(track.offline_days_left, 30);

        // O retrato em claro não leva o fechado nem a chave.
        let mut snap = save_offline_snapshot(&model);
        let Some(KeyValueOperation::Set { value, .. }) = kv_ops(&mut snap).into_iter().next() else {
            panic!("o retrato é gravado");
        };
        let text = String::from_utf8(value).unwrap();
        assert!(!text.contains("ch_t03") && !text.contains(&test_license().key_hex), "{text}");
    }

    fn reads_refresh_token(kv: &[KeyValueOperation]) -> bool {
        kv.iter().any(|op| matches!(op, KeyValueOperation::Get { key } if key == "refresh_token"))
    }

    #[test]
    fn test_download_on_an_offline_session_waits_for_the_refresh() {
        let (app, mut model) = paid_model();
        model.access_token = None;
        model.session_offline = true;
        for _ in 0..2 {
            let mut cmd = app.update(Event::FetchLicense { track_id: TRACK.into() }, &mut model);
            let (kv, sent) = drain(&mut cmd);
            assert!(sent.is_empty(), "sem token, a licença não sai: {sent:?}");
            assert!(reads_refresh_token(&kv), "renova a sessão");
        }
        let queued = model.purchase_retries.iter().filter(|e| matches!(e, Event::FetchLicense { .. })).count();
        assert_eq!(queued, 1, "dois toques, um pedido na fila");
    }

    #[test]
    fn test_a_license_refused_for_the_token_is_asked_again_after_the_refresh() {
        let (app, mut model) = paid_model();
        let mut cmd = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(401, serde_json::json!({})) },
            &mut model,
        );
        assert!(reads_refresh_token(&drain(&mut cmd).0));
        assert!(model.purchase_retries.iter().any(|e| matches!(e, Event::FetchLicense { track_id } if track_id == TRACK)));

        // O refresh terminou: a fila sai, e a licença é pedida de novo.
        let mut cmd = app.update(Event::AccountEmailRead(kv_empty()), &mut model);
        let (_, sent) = drain(&mut cmd);
        assert!(sent.iter().any(|r| r.url == format!("/api/v1/tracks/{TRACK}/license")), "{sent:?}");
        assert!(model.purchase_retries.is_empty());
    }

    #[test]
    fn test_a_second_401_on_the_license_fails_instead_of_refreshing_again() {
        let (app, mut model) = paid_model();
        let refused = |model: &mut Model| {
            let mut cmd = app.update(
                Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(401, serde_json::json!({})) },
                model,
            );
            reads_refresh_token(&drain(&mut cmd).0)
        };
        assert!(refused(&mut model), "o primeiro renova");
        let _ = app.update(Event::AccountEmailRead(kv_empty()), &mut model);
        assert!(!refused(&mut model), "o segundo não gira outro refresh");
        assert!(model.purchase_retries.is_empty());

        // Uma licença que chega limpa a marca: o token que vencer depois tem a sua vez.
        let _ = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(200, serde_json::to_value(test_license()).unwrap()) },
            &mut model,
        );
        assert!(refused(&mut model));
    }

    #[test]
    fn test_download_without_a_session_does_not_queue() {
        let (app, mut model) = paid_model();
        model.access_token = None;
        model.session_offline = false;
        let mut cmd = app.update(Event::FetchLicense { track_id: TRACK.into() }, &mut model);
        let (kv, sent) = drain(&mut cmd);
        assert!(sent.is_empty() && !reads_refresh_token(&kv));
        assert!(model.purchase_retries.is_empty());
    }

    #[test]
    fn test_a_package_that_does_not_open_is_not_kept() {
        let (app, mut model) = paid_model();
        model.licenses.insert(TRACK.into(), test_license());
        let mut tampered = test_package();
        tampered[20] ^= 1;
        let mut cmd = app.update(
            Event::PackageFetched {
                user_id: USER_A.into(),
                track_id: TRACK.into(),
                result: HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body: tampered }),
            },
            &mut model,
        );
        assert!(set_keys(&kv_ops(&mut cmd)).is_empty());
        assert!(!model.challenges.iter().any(|c| c.id == "ch_t03"));
    }

    /// Reembolso na loja: na abertura seguinte com rede, chave e pacote saem (critério 5).
    #[test]
    fn test_a_revoked_license_wipes_the_track() {
        let (app, mut model) = paid_model();
        licensed(&app, &mut model);
        model.global_xp = 150;

        let mut cmd = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(403, serde_json::json!({ "code": "entitlement_required" })) },
            &mut model,
        );
        let deleted: Vec<String> = kv_ops(&mut cmd)
            .into_iter()
            .filter_map(|op| match op {
                KeyValueOperation::Delete { key } => Some(key),
                _ => None,
            })
            .collect();
        assert!(deleted.contains(&format!("track_key:{USER_A}:{TRACK}")));
        assert!(deleted.contains(&format!("track_package:{USER_A}:{TRACK}")));
        assert!(!model.challenges.iter().any(|c| c.id == "ch_t03"));
        assert_eq!(model.status_key, StatusKey::TrackRevoked);
        assert_eq!(status_of(&model, CLOSED_NODE), crate::domain::NodeStatus::PaywallLocked);
        assert_eq!(model.global_xp, 150, "o XP ganho fica");
    }

    #[test]
    fn test_the_downloads_come_back_from_the_device_offline() {
        let (app, mut model) = paid_model();
        model.access_token = None;

        let mut cmd = app.update(Event::LoadTrackDownloads, &mut model);
        assert!(matches!(kv_ops(&mut cmd).as_slice(), [KeyValueOperation::Get { key }] if *key == format!("track_index:{USER_A}")));

        let mut cmd = app.update(
            Event::TrackIndexRead { user_id: USER_A.into(), result: kv_bytes(serde_json::to_vec(&[TRACK]).unwrap()) },
            &mut model,
        );
        let (kv, sent) = drain(&mut cmd);
        assert!(sent.is_empty(), "sem rede não revalida");
        let read: Vec<String> = kv
            .iter()
            .filter_map(|op| match op {
                KeyValueOperation::Get { key } => Some(key.clone()),
                _ => None,
            })
            .collect();
        assert_eq!(read, vec![crate::tracks::clock_key(USER_A), format!("track_key:{USER_A}:{TRACK}")]);

        let _ = app.update(
            Event::LicenseRead { user_id: USER_A.into(), track_id: TRACK.into(), result: kv_bytes(serde_json::to_vec(&test_license()).unwrap()) },
            &mut model,
        );
        let _ = app.update(
            Event::PackageRead { user_id: USER_A.into(), track_id: TRACK.into(), result: kv_bytes(test_package()) },
            &mut model,
        );
        assert!(model.challenges.iter().any(|c| c.id == "ch_t03"));

        // A leitura de outra conta, que chegou atrasada, não entra.
        let (app, mut model) = paid_model();
        let _ = app.update(
            Event::LicenseRead { user_id: USER_B.into(), track_id: TRACK.into(), result: kv_bytes(serde_json::to_vec(&test_license()).unwrap()) },
            &mut model,
        );
        assert!(model.licenses.is_empty());
    }

    /// A saiu com o pedido de licença no ar, B entrou: a resposta é de A e não entra em B.
    #[test]
    fn test_a_license_asked_by_another_account_is_dropped() {
        let (app, mut model) = paid_model();
        model.user_id = USER_B.into();
        let mut cmd = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: TRACK.into(), result: http(200, serde_json::to_value(test_license()).unwrap()) },
            &mut model,
        );
        assert!(kv_ops(&mut cmd).is_empty(), "nada gravado sob a conta nova");
        assert!(model.licenses.is_empty());

        model.licenses.insert(TRACK.into(), test_license());
        let _ = app.update(
            Event::PackageFetched {
                user_id: USER_A.into(),
                track_id: TRACK.into(),
                result: HttpResult::Ok(crux_http::protocol::HttpResponse { status: 200, headers: vec![], body: test_package() }),
            },
            &mut model,
        );
        assert!(model.track_content.is_empty());
    }

    /// Venceu com o app vendo a hora: atrasar o relógio depois não ressuscita a licença.
    #[test]
    fn test_turning_the_clock_back_does_not_revive_a_license() {
        let (app, mut model) = paid_model();
        licensed(&app, &mut model);
        assert!(track_open(&model, TRACK));

        let mut cmd = app.update(Event::Tick { now: test_license().valid_until + 1 }, &mut model);
        assert!(set_keys(&kv_ops(&mut cmd)).contains(&crate::tracks::clock_key(USER_A)), "a marca vai para o Keychain");
        assert!(!track_open(&model, TRACK));

        let _ = app.update(Event::Tick { now: NOW }, &mut model);
        assert!(!track_open(&model, TRACK), "relógio de volta para dentro do prazo");

        // A marca guardada volta na abertura seguinte.
        let (app, mut model) = paid_model();
        model.licenses.insert(TRACK.into(), test_license());
        let _ = app.update(
            Event::ClockRead { user_id: USER_A.into(), result: kv_bytes((test_license().valid_until + 1).to_string().into_bytes()) },
            &mut model,
        );
        assert!(!track_open(&model, TRACK));
    }

    #[test]
    fn test_a_closed_node_does_not_start_without_a_license() {
        let (app, mut model) = paid_model();
        model.challenges.push(on_node("ch_t03", CLOSED_NODE));
        let _ = app.update(Event::StartMatch { node_id: CLOSED_NODE.into() }, &mut model);
        assert!(model.match_state.is_none());

        licensed(&app, &mut model);
        let _ = app.update(Event::StartMatch { node_id: CLOSED_NODE.into() }, &mut model);
        assert!(model.match_state.is_some());
    }

    #[test]
    fn test_signing_out_forgets_the_tracks() {
        let (app, mut model) = paid_model();
        licensed(&app, &mut model);
        let _ = app.update(
            Event::TokenCleared(KeyValueResult::Ok { response: KeyValueResponse::Delete { previous: crux_kv::Value::None } }),
            &mut model,
        );
        assert!(model.licenses.is_empty() && model.track_content.is_empty() && model.track_index.is_empty());

        // Desfazer a saída devolve a conta, e as trilhas voltam do disco, não da memória.
        let _ = app.update(Event::UndoLogout, &mut model);
        assert!(!model.challenges.iter().any(|c| c.id == "ch_t03"));
    }

    fn social_login(app: &LogNApp, model: &mut Model) -> Vec<HttpRequest> {
        let mut cmd = app.update(
            Event::SocialLogin { provider: "google".into(), id_token: "idtok".into(), nonce: "nonce-cru".into() },
            model,
        );
        http_requests(&mut cmd)
    }

    fn body_of(req: &HttpRequest) -> serde_json::Value {
        serde_json::from_slice(&req.body).unwrap()
    }

    #[test]
    fn social_login_sends_the_token_and_opens_the_session() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let reqs = social_login(&app, &mut model);
        assert_eq!(reqs.len(), 1);
        assert_eq!(reqs[0].url, "/api/v1/auth/social");
        let body = body_of(&reqs[0]);
        assert_eq!(body["provider"], "google");
        assert_eq!(body["id_token"], "idtok");
        assert_eq!(body["nonce"], "nonce-cru");
        // A primeira tentativa não aceita termo nenhum em nome de ninguém.
        assert!(body.get("legal_acceptances").is_none() && body.get("age_confirmed").is_none());
        assert!(model.is_authenticating);

        let mut cmd = app.update(
            Event::SocialLoginCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "ref", "user_id": USER_A,
                "refresh_expires_at": 1_900_000_000, "email": "pessoa@example.com",
            }))),
            &mut model,
        );
        assert_eq!(model.access_token.as_deref(), Some("acc"));
        assert_eq!(model.user_id, USER_A);
        // O app não digitou e-mail: a sessão fica com o que o servidor disse.
        assert_eq!(model.account_email, "pessoa@example.com");
        assert!(model.social_pending.is_none(), "o token não fica na memória depois do login");
        assert!(!model.is_authenticating);
        assert!(matches!(kv_ops(&mut cmd).as_slice(),
            [KeyValueOperation::Set { key, .. }] if key == "refresh_token"));
    }

    #[test]
    fn social_signup_asks_for_age_and_terms_and_reuses_the_token() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.legal_versions = vec![("terms".into(), 3), ("privacy".into(), 2)];
        model.legal_country = "BR".into();

        social_login(&app, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(409, "signup_required")), &mut model);
        assert!(app.view(&model).social_signup_required);
        assert_eq!(model.status_key, StatusKey::Silent, "pedir termos não é erro");
        assert!(model.social_pending.is_some());

        // Caixa errada na tela: o erro fica na tela e o login continua de pé.
        let _ = app.update(Event::CompleteSocialSignup { age_confirmed: true, legal_accepted: false }, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(400, "legal_acceptance_required")), &mut model);
        assert!(model.social_signup_required && model.social_pending.is_some());
        assert_eq!(model.status_key, StatusKey::AccountFailed);

        let mut cmd = app.update(Event::CompleteSocialSignup { age_confirmed: true, legal_accepted: true }, &mut model);
        let reqs = http_requests(&mut cmd);
        let body = body_of(&reqs[0]);
        assert_eq!(body["id_token"], "idtok", "o mesmo token volta com os aceites");
        assert_eq!(body["age_confirmed"], true);
        assert_eq!(body["country"], "BR");
        assert_eq!(body["legal_acceptances"].as_array().unwrap().len(), 2);
        assert_eq!(body["legal_acceptances"][0]["version"], 3);

        let _ = app.update(
            Event::SocialLoginCompleted(http(200, serde_json::json!({
                "access_token": "acc", "refresh_token": "ref", "user_id": USER_A, "email": "nova@example.com",
            }))),
            &mut model,
        );
        assert!(!app.view(&model).social_signup_required && model.social_pending.is_none());
    }

    #[test]
    fn cancelling_the_social_signup_drops_the_token() {
        let app = LogNApp::default();
        let mut model = Model::default();
        social_login(&app, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(409, "signup_required")), &mut model);
        let _ = app.update(Event::CancelSocialSignup, &mut model);
        assert!(model.social_pending.is_none() && !model.social_signup_required);

        // Sem login pendente, completar não manda nada.
        let mut cmd = app.update(Event::CompleteSocialSignup { age_confirmed: true, legal_accepted: true }, &mut model);
        assert!(http_requests(&mut cmd).is_empty());
    }

    #[test]
    fn social_login_errors_end_the_login_with_their_own_key() {
        let app = LogNApp::default();
        for (status, code, key) in [
            (401, "social_token_invalid", StatusKey::SocialSignInFailed),
            (403, "social_email_unverified", StatusKey::SocialEmailUnverified),
            (503, "provider_disabled", StatusKey::SocialProviderDisabled),
            (500, "internal", StatusKey::SignInFailed),
        ] {
            let mut model = Model::default();
            social_login(&app, &mut model);
            let _ = app.update(Event::SocialLoginCompleted(api_error(status, code)), &mut model);
            assert_eq!(model.status_key, key, "{code}");
            assert!(model.social_pending.is_none() && !model.is_authenticating, "{code}");
            assert!(model.access_token.is_none(), "{code}");
        }
    }

    #[test]
    fn social_signup_survives_what_the_person_can_retry() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.legal_versions = vec![("terms".into(), 1), ("privacy".into(), 1)];
        social_login(&app, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(409, "signup_required")), &mut model);

        // Sem rede no "Criar conta": o toque seguinte manda de novo o mesmo token.
        let _ = app.update(Event::CompleteSocialSignup { age_confirmed: true, legal_accepted: true }, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(HttpResult::Err(crux_http::HttpError::Io("offline".into()))), &mut model);
        assert!(model.social_signup_required && model.social_pending.is_some());
        assert_eq!(model.status_key, StatusKey::NoConnection);

        // Versão nova dos termos no meio: busca a vigente e a tela continua.
        let _ = app.update(Event::CompleteSocialSignup { age_confirmed: true, legal_accepted: true }, &mut model);
        let mut cmd = app.update(Event::SocialLoginCompleted(api_error(409, "legal_version_outdated")), &mut model);
        assert!(http_requests(&mut cmd).iter().any(|r| r.url.starts_with("/api/v1/legal/current")));
        assert!(model.social_signup_required && model.social_pending.is_some());

        // 429 na tela: a contagem trava o botão, e o login continua de pé.
        let _ = app.update(Event::CompleteSocialSignup { age_confirmed: true, legal_accepted: true }, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(429, "rate_limited")), &mut model);
        assert!(model.social_pending.is_some());
    }

    #[test]
    fn a_429_outside_the_signup_ends_the_social_login() {
        let app = LogNApp::default();
        let mut model = Model::default();
        social_login(&app, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(429, "rate_limited")), &mut model);
        assert!(model.social_pending.is_none(), "fora da tela de termos, o 429 acaba o login");
        assert_eq!(model.status_key, StatusKey::RateLimited);
    }

    #[test]
    fn a_late_cancel_keeps_the_error_on_screen() {
        let app = LogNApp::default();
        let mut model = Model::default();
        social_login(&app, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(409, "signup_required")), &mut model);
        let _ = app.update(Event::CompleteSocialSignup { age_confirmed: true, legal_accepted: true }, &mut model);
        let _ = app.update(Event::SocialLoginCompleted(api_error(401, "social_token_invalid")), &mut model);
        assert!(!model.social_signup_required);

        // A tela fechou por causa do erro, e o shell avisa o fechamento depois.
        let _ = app.update(Event::CancelSocialSignup, &mut model);
        assert_eq!(model.status_key, StatusKey::SocialSignInFailed);
    }

    #[test]
    fn signup_required_after_a_cancel_opens_nothing() {
        let app = LogNApp::default();
        let mut model = Model::default();
        social_login(&app, &mut model);
        model.social_pending = None;
        let _ = app.update(Event::SocialLoginCompleted(api_error(409, "signup_required")), &mut model);
        assert!(!app.view(&model).social_signup_required);
    }

    #[test]
    fn a_failure_on_the_device_says_so() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let _ = app.update(Event::SocialLoginFailed, &mut model);
        assert_eq!(model.status_key, StatusKey::SocialSignInFailed);
        assert!(!model.is_authenticating);
    }

    #[test]
    fn delete_account_with_provider_sends_the_fresh_token() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("acc".into());
        let mut cmd = app.update(
            Event::DeleteAccountWithProvider {
                provider: "apple".into(), id_token: "novo".into(), nonce: "n".into(),
                authorization_code: "code-1".into(),
            },
            &mut model,
        );
        let reqs = http_requests(&mut cmd);
        assert_eq!(reqs[0].url, "/api/v1/users/me/delete");
        let body = body_of(&reqs[0]);
        assert_eq!(body["id_token"], "novo");
        assert_eq!(body["provider"], "apple");
        // O código vai junto: é com ele que o servidor revoga o acesso na Apple.
        assert_eq!(body["authorization_code"], "code-1");
        assert!(body.get("password").is_none());
    }

    fn github_code(app: &LogNApp, model: &mut Model) -> Vec<HttpRequest> {
        let mut cmd = app.update(
            Event::GitHubCodeReceived { code: "cod".into(), code_verifier: "ver".into(), nonce: "nonce-cru".into() },
            model,
        );
        http_requests(&mut cmd)
    }

    #[test]
    fn github_login_trades_the_code_for_a_ticket_and_logs_in_with_it() {
        let app = LogNApp::default();
        let mut model = Model::default();

        let reqs = github_code(&app, &mut model);
        assert_eq!(reqs.len(), 1);
        assert_eq!(reqs[0].url, "/api/v1/auth/github/exchange");
        let body = body_of(&reqs[0]);
        assert_eq!(body["code"], "cod");
        assert_eq!(body["code_verifier"], "ver");
        assert_eq!(body["purpose"], "login");
        // Na troca o nonce vai só como hash; o cru espera o pedido do bilhete.
        let hash: String = Sha256::digest(b"nonce-cru").iter().map(|b| format!("{:02x}", b)).collect();
        assert_eq!(body["nonce_hash"], hash);
        assert!(!String::from_utf8_lossy(&reqs[0].body).contains("\"nonce-cru\""));
        assert!(model.is_authenticating);

        let mut cmd = app.update(Event::GitHubExchanged(http(200, serde_json::json!({ "ticket": "bilhete" }))), &mut model);
        let reqs = http_requests(&mut cmd);
        assert_eq!(reqs.len(), 1);
        assert_eq!(reqs[0].url, "/api/v1/auth/social");
        let body = body_of(&reqs[0]);
        assert_eq!(body["provider"], "github");
        assert_eq!(body["id_token"], "bilhete");
        assert_eq!(body["nonce"], "nonce-cru");
        assert!(model.github_nonce.is_none(), "o nonce não fica na memória depois da troca");
        assert!(model.social_pending.is_some(), "o bilhete fica para o signup_required");
    }

    #[test]
    fn github_exchange_refused_does_not_log_in() {
        let cases = [
            (api_error(401, "social_token_invalid"), StatusKey::SocialSignInFailed),
            (api_error(503, "provider_disabled"), StatusKey::SocialProviderDisabled),
            (http(200, serde_json::json!({ "ticket": "" })), StatusKey::ServerUnreadable),
            (HttpResult::Err(crux_http::HttpError::Timeout), StatusKey::NoConnection),
        ];
        for (result, want) in cases {
            let app = LogNApp::default();
            let mut model = Model::default();
            github_code(&app, &mut model);
            let mut cmd = app.update(Event::GitHubExchanged(result), &mut model);
            assert!(http_requests(&mut cmd).is_empty(), "sem bilhete não há login");
            assert_eq!(model.status_key, want);
            assert!(!model.is_authenticating && model.github_nonce.is_none());
        }
    }

    #[test]
    fn late_github_exchange_opens_nothing() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(Event::GitHubExchanged(http(200, serde_json::json!({ "ticket": "bilhete" }))), &mut model);
        assert!(http_requests(&mut cmd).is_empty(), "troca sem login pendente não entra em conta nenhuma");
    }

    #[test]
    fn github_deletion_sends_the_ticket_and_the_token_to_revoke() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("acc".into());
        let mut cmd = app.update(
            Event::DeleteAccountWithGitHub { code: "cod".into(), code_verifier: "ver".into(), nonce: "nonce-cru".into() },
            &mut model,
        );
        let reqs = http_requests(&mut cmd);
        assert_eq!(reqs[0].url, "/api/v1/auth/github/exchange");
        assert_eq!(body_of(&reqs[0])["purpose"], "delete");
        assert!(
            reqs[0].headers.iter().any(|h| h.name.eq_ignore_ascii_case("authorization") && h.value == "Bearer acc"),
            "a troca de exclusão vai com a sessão"
        );

        // Outra troca no meio não troca o nonce desta.
        let mut again = app.update(
            Event::GitHubCodeReceived { code: "c2".into(), code_verifier: "v2".into(), nonce: "outro".into() },
            &mut model,
        );
        assert!(http_requests(&mut again).is_empty());
        assert_eq!(model.github_nonce.as_deref(), Some("nonce-cru"));

        let mut cmd = app.update(
            Event::GitHubDeleteExchanged(http(200, serde_json::json!({ "ticket": "bilhete", "access_token": "gho_x" }))),
            &mut model,
        );
        let reqs = http_requests(&mut cmd);
        assert_eq!(reqs[0].url, "/api/v1/users/me/delete");
        let body = body_of(&reqs[0]);
        assert_eq!(body["provider"], "github");
        assert_eq!(body["id_token"], "bilhete");
        assert_eq!(body["nonce"], "nonce-cru");
        assert_eq!(body["authorization_code"], "gho_x");
        assert!(body.get("password").is_none());
    }

    #[test]
    fn github_deletion_without_the_token_does_not_delete() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("acc".into());
        let _ = app.update(
            Event::DeleteAccountWithGitHub { code: "cod".into(), code_verifier: "ver".into(), nonce: "n".into() },
            &mut model,
        );
        // Sem o token o servidor não teria com que revogar: não chega a pedir.
        let mut cmd = app.update(Event::GitHubDeleteExchanged(http(200, serde_json::json!({ "ticket": "bilhete" }))), &mut model);
        assert!(http_requests(&mut cmd).is_empty());
        assert_eq!(model.status_key, StatusKey::ServerUnreadable);

        let _ = app.update(
            Event::DeleteAccountWithGitHub { code: "cod".into(), code_verifier: "ver".into(), nonce: "n".into() },
            &mut model,
        );
        let mut cmd = app.update(Event::GitHubDeleteExchanged(api_error(401, "social_token_invalid")), &mut model);
        assert!(http_requests(&mut cmd).is_empty());
        assert_eq!(model.status_key, StatusKey::WrongCredentials);
    }

    #[test]
    fn deleting_an_apple_linked_account_by_password_asks_for_apple() {
        let app = LogNApp::default();
        let mut model = Model::default();
        model.access_token = Some("acc".into());
        let _ = app.update(Event::DeleteAccount { password: "senha-forte".into() }, &mut model);
        let _ = app.update(Event::AccountDeleted(api_error(409, "provider_reauth_required")), &mut model);
        assert_eq!(model.status_key, StatusKey::ProviderReauthRequired);
        assert!(model.access_token.is_some(), "a conta segue de pé");
    }

    /// Os registros de log e de erro que o comando pediu.
    fn logs_of(cmd: &mut Command<Effect, Event>) -> (Vec<crate::domain::LogOperation>, Vec<crate::domain::MonitoringOperation>) {
        let (mut logs, mut errors) = (vec![], vec![]);
        for e in cmd.effects() {
            match e {
                Effect::Log(r) => logs.push(r.operation.clone()),
                Effect::Monitoring(r) => errors.push(r.operation.clone()),
                _ => {}
            }
        }
        (logs, errors)
    }

    #[test]
    fn a_server_error_becomes_an_error_log_and_an_issue() {
        use crate::domain::{LogLevel, MonitoringOperation};
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(Event::SyncCompleted(api_error(503, "internal")), &mut model);
        let (logs, errors) = logs_of(&mut cmd);
        assert_eq!(logs.len(), 1);
        assert_eq!(logs[0].level, LogLevel::Error);
        assert_eq!(logs[0].attributes["route"], "/api/v1/sync");
        assert_eq!(logs[0].attributes["status"], "503");
        assert_eq!(logs[0].attributes["code"], "internal");
        assert!(matches!(&errors[..], [MonitoringOperation::LogError { message, .. }] if message == "server error 503 on /api/v1/sync"));
    }

    #[test]
    fn being_offline_is_information_not_an_error() {
        use crate::domain::LogLevel;
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(
            Event::NodesFetched(HttpResult::Err(crux_http::HttpError::Io("offline sh.logn/secret?token=x".into()))),
            &mut model,
        );
        let (logs, errors) = logs_of(&mut cmd);
        assert!(errors.is_empty(), "sem rede não é issue");
        assert_eq!(logs.len(), 1);
        assert_eq!(logs[0].level, LogLevel::Info);
        assert_eq!(logs[0].attributes["kind"], "io");
        // A frase do erro, que pode trazer o endereço inteiro, não vai para o log.
        let everything = format!("{} {:?}", logs[0].message, logs[0].attributes);
        assert!(!everything.contains("token") && !everything.contains("secret"));
    }

    #[test]
    fn expected_answers_do_not_log() {
        let app = LogNApp::default();
        for result in [api_error(401, "invalid_credentials"), api_error(429, "rate_limited"), http(200, serde_json::json!({}))] {
            let mut model = Model::default();
            let mut cmd = app.update(Event::LoginCompleted(result), &mut model);
            let (logs, errors) = logs_of(&mut cmd);
            assert!(logs.is_empty() && errors.is_empty(), "4xx e sucesso não viram log");
        }
    }

    #[test]
    fn the_route_hides_ids() {
        let app = LogNApp::default();
        let mut model = Model::default();
        let mut cmd = app.update(
            Event::LicenseFetched { user_id: USER_A.into(), track_id: "trilha-1".into(), result: api_error(500, "internal") },
            &mut model,
        );
        let (logs, _) = logs_of(&mut cmd);
        let everything = format!("{} {:?}", logs[0].message, logs[0].attributes);
        assert!(everything.contains("/api/v1/tracks/{id}/license"));
        assert!(!everything.contains(USER_A) && !everything.contains("trilha-1"));
    }

    // --- Placar geral de XP (docs/specs/logn_placar_spec.md) ---

    fn board_session() -> Model {
        Model { access_token: Some("a".into()), user_id: "u1".into(), ..Model::default() }
    }

    /// Entrega a resposta do placar como a de um pedido que estava no ar.
    fn deliver_board(app: &LogNApp, model: &mut Model, result: HttpResult) -> Command<Effect, Event> {
        model.leaderboard_in_flight = Some(model.user_id.clone());
        app.update(Event::LeaderboardFetched { owner: model.user_id.clone(), result }, model)
    }

    fn board_body(open: bool, me_rank: Option<i32>, nickname: Option<&str>, generated_at: i64) -> HttpResult {
        http(200, serde_json::json!({
            "open": open, "missing": if open { 0 } else { 3 }, "threshold": 10,
            "rows": if open { serde_json::json!([
                { "rank": 1, "anon_number": 4821, "nickname": null, "xp": 900, "is_me": false },
                { "rank": 2, "anon_number": 2207, "nickname": nickname, "xp": 50, "is_me": true },
            ]) } else { serde_json::json!([]) },
            "me": { "rank": me_rank, "anon_number": 2207, "nickname": nickname, "xp": 50, "hidden": false },
            "generated_at": generated_at,
        }))
    }

    #[test]
    fn test_a_guest_sees_the_call_to_sign_up_and_asks_nothing() {
        let app = LogNApp::default();
        let mut model = Model { is_guest: true, ..Model::default() };
        let mut cmd = app.update(Event::LeaderboardOpened, &mut model);
        let (kv, http) = drain(&mut cmd);
        assert!(kv.is_empty() && http.is_empty());
        assert_eq!(app.view(&model).leaderboard.state, crate::domain::LeaderboardState::SignedOut);
    }

    #[test]
    fn test_opening_the_leaderboard_reads_the_disk_and_asks_the_server() {
        let app = LogNApp::default();
        let mut model = board_session();
        let mut cmd = app.update(Event::LeaderboardOpened, &mut model);
        let (kv, http) = drain(&mut cmd);
        assert!(matches!(&kv[..], [KeyValueOperation::Get { key }] if key == "leaderboard"));
        assert_eq!(http.len(), 1);
        assert_eq!(http[0].url, "/api/v1/leaderboard");
        assert_eq!(app.view(&model).leaderboard.state, crate::domain::LeaderboardState::Loading);

        let mut cmd = app.update(Event::LeaderboardFetched { owner: "u1".into(), result: board_body(true, Some(2), None, 100) }, &mut model);
        let (kv, _) = drain(&mut cmd);
        assert!(kv.iter().any(|op| matches!(op, KeyValueOperation::Set { key, .. } if key == "leaderboard")),
            "a lista vai para o disco");
        let view = app.view(&model);
        assert_eq!(view.leaderboard.state, crate::domain::LeaderboardState::Open);
        assert_eq!(view.leaderboard.rows.len(), 2);
        assert!(view.leaderboard.me_in_rows);
        assert_eq!(view.profile_anon_number, 2207);

        // Abrir de novo não relê o disco.
        let mut cmd = app.update(Event::LeaderboardOpened, &mut model);
        let (kv, http) = drain(&mut cmd);
        assert!(kv.is_empty() && http.len() == 1);
    }

    #[test]
    fn test_a_network_error_keeps_the_stored_list_as_offline() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = deliver_board(&app, &mut model, HttpResult::Err(crux_http::HttpError::Io("x".into())));
        assert_eq!(app.view(&model).leaderboard.state, crate::domain::LeaderboardState::Unavailable);

        let _ = deliver_board(&app, &mut model, board_body(true, Some(2), None, 100));
        let _ = app.update(Event::LeaderboardRefresh, &mut model);
        let _ = deliver_board(&app, &mut model, HttpResult::Err(crux_http::HttpError::Io("x".into())));
        let view = app.view(&model).leaderboard;
        assert_eq!(view.state, crate::domain::LeaderboardState::Open);
        assert!(view.offline && view.rows.len() == 2);
    }

    #[test]
    fn test_a_refresh_that_fails_marks_the_list_offline() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = app.update(Event::LeaderboardOpened, &mut model);
        let _ = app.update(Event::LeaderboardFetched { owner: "u1".into(), result: board_body(true, Some(2), None, 100) }, &mut model);
        let mut cmd = app.update(Event::LeaderboardRefresh, &mut model);
        assert_eq!(http_requests(&mut cmd).len(), 1, "puxar para atualizar pede de novo");
        let _ = app.update(Event::LeaderboardFetched { owner: "u1".into(), result: HttpResult::Err(crux_http::HttpError::Io("ConnectException".into())) }, &mut model);
        let view = app.view(&model).leaderboard;
        assert!(view.offline, "sem rede, a lista guardada fica marcada");
        assert_eq!(view.rows.len(), 2);
    }

    #[test]
    fn test_a_closed_leaderboard_shows_how_many_are_missing() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = deliver_board(&app, &mut model, board_body(false, None, None, 100));
        let view = app.view(&model).leaderboard;
        assert_eq!(view.state, crate::domain::LeaderboardState::Closed);
        assert_eq!((view.missing, view.threshold), (3, 10));
        assert!(view.rows.is_empty());
        assert_eq!(view.me_status, crate::domain::LeaderboardMeStatus::Waiting);
    }

    #[test]
    fn test_the_leaderboard_401_renews_the_session_and_retries() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = deliver_board(&app, &mut model, api_error(401, "unauthenticated"));
        assert!(matches!(model.pending_retry_event, Some(Event::LeaderboardRefresh)));
    }

    #[test]
    fn test_another_accounts_board_never_enters() {
        let app = LogNApp::default();
        let mut model = board_session();
        // Resposta de quem pediu antes de trocar de conta.
        let _ = app.update(Event::LeaderboardFetched { owner: "u0".into(), result: board_body(true, Some(2), None, 100) }, &mut model);
        assert!(model.leaderboard.is_none());

        // No disco, de outra conta: apaga.
        let stored = serde_json::to_vec(&crate::leaderboard::LeaderboardCache { owner: "u0".into(), board: Default::default() }).unwrap();
        let mut cmd = app.update(Event::LeaderboardRestored { owner: "u1".into(), result: kv_bytes(stored) }, &mut model);
        let (kv, _) = drain(&mut cmd);
        assert!(matches!(&kv[..], [KeyValueOperation::Delete { key }] if key == "leaderboard"));
        assert!(model.leaderboard.is_none());
    }

    #[test]
    fn test_an_older_disk_board_does_not_replace_a_newer_one() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = deliver_board(&app, &mut model, board_body(true, Some(2), None, 200));
        let old = crate::leaderboard::LeaderboardCache {
            owner: "u1".into(),
            board: crate::leaderboard::Board { generated_at: 100, ..Default::default() },
        };
        let _ = app.update(Event::LeaderboardRestored { owner: "u1".into(), result: kv_bytes(serde_json::to_vec(&old).unwrap()) }, &mut model);
        assert_eq!(model.leaderboard.as_ref().unwrap().board.generated_at, 200);
    }

    #[test]
    fn test_logout_deletes_the_board_and_undo_brings_it_back() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = deliver_board(&app, &mut model, board_body(true, Some(2), None, 100));

        let mut cmd = app.update(Event::Logout, &mut model);
        let (kv, _) = drain(&mut cmd);
        assert!(kv.iter().any(|op| matches!(op, KeyValueOperation::Delete { key } if key == "leaderboard")));
        let _ = app.update(Event::TokenCleared(KeyValueResult::Ok { response: KeyValueResponse::Delete { previous: crux_kv::Value::None } }), &mut model);
        assert!(model.leaderboard.is_none());
        assert_eq!(model.profile.anon_number, 0);

        let mut cmd = app.update(Event::UndoLogout, &mut model);
        let (kv, _) = drain(&mut cmd);
        assert!(kv.iter().any(|op| matches!(op, KeyValueOperation::Set { key, .. } if key == "leaderboard")));
        assert!(model.leaderboard.is_some());
        assert_eq!(model.profile.anon_number, 2207);
    }

    #[test]
    fn test_progress_brings_the_profile_name() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = app.update(Event::ProgressFetched(http(200, serde_json::json!({
            "global_xp": 0, "bugs_found": 0, "dry_runs_completed": 0,
            "anon_number": 4821, "nickname": null, "nickname_locked": false,
        }))), &mut model);
        let view = app.view(&model);
        assert_eq!(view.profile_anon_number, 4821);
        assert!(view.profile_nickname.is_none() && view.can_choose_nickname);

        // Moderado: sem apelido e sem botão.
        let _ = app.update(Event::ProgressFetched(http(200, serde_json::json!({
            "global_xp": 0, "bugs_found": 0, "dry_runs_completed": 0,
            "anon_number": 4821, "nickname": null, "nickname_locked": true,
        }))), &mut model);
        assert!(!app.view(&model).can_choose_nickname);
    }

    #[test]
    fn test_an_invalid_nickname_never_leaves_the_device() {
        let app = LogNApp::default();
        let mut model = board_session();
        let mut cmd = app.update(Event::NicknameChecked("Ana Dev".into()), &mut model);
        assert!(http_requests(&mut cmd).is_empty());
        let flow = app.view(&model).nickname_flow;
        assert_eq!(flow.error, StatusKey::NicknameInvalid);
        assert_eq!(flow.step, crate::domain::NicknameStep::Input);

        let _ = app.update(Event::NicknameChecked("  Ana_Dev ".into()), &mut model);
        let flow = app.view(&model).nickname_flow;
        assert_eq!(flow.step, crate::domain::NicknameStep::Confirm);
        assert_eq!(flow.draft, "ana_dev");
    }

    #[test]
    fn test_a_saved_nickname_shows_without_a_new_fetch() {
        let app = LogNApp::default();
        let mut model = board_session();
        model.profile.anon_number = 2207;
        let _ = deliver_board(&app, &mut model, board_body(true, Some(2), None, 100));

        let mut cmd = app.update(Event::NicknameSubmitted("ana_dev".into()), &mut model);
        let reqs = http_requests(&mut cmd);
        assert_eq!(reqs.len(), 1);
        assert_eq!(reqs[0].method, "PUT");
        assert_eq!(reqs[0].url, "/api/v1/profile/nickname");

        let mut cmd = app.update(Event::NicknameSaved {
            owner: "u1".into(), draft: "ana_dev".into(),
            result: http(200, serde_json::json!({ "nickname": "ana_dev" })),
        }, &mut model);
        assert!(http_requests(&mut cmd).is_empty(), "sem novo GET do placar");
        let view = app.view(&model);
        assert_eq!(view.profile_nickname.as_deref(), Some("ana_dev"));
        assert!(!view.can_choose_nickname);
        assert_eq!(view.nickname_flow.step, crate::domain::NicknameStep::Done);
        let me = view.leaderboard.rows.iter().find(|r| r.is_me).unwrap();
        assert_eq!(me.nickname.as_deref(), Some("ana_dev"));
    }

    #[test]
    fn test_each_nickname_error_has_its_key() {
        let app = LogNApp::default();
        for (status, code, want) in [
            (400, "nickname_invalid", StatusKey::NicknameInvalid),
            (409, "nickname_reserved", StatusKey::NicknameReserved),
            (409, "nickname_taken", StatusKey::NicknameTaken),
            (409, "nickname_locked", StatusKey::NicknameLocked),
        ] {
            let mut model = board_session();
            model.profile.anon_number = 2207;
            let _ = app.update(Event::NicknameSubmitted("ana_dev".into()), &mut model);
            let _ = app.update(Event::NicknameSaved { owner: "u1".into(), draft: "ana_dev".into(), result: api_error(status, code) }, &mut model);
            let view = app.view(&model);
            assert_eq!(view.nickname_flow.error, want, "{code}");
            assert_eq!(view.nickname_flow.step, crate::domain::NicknameStep::Input, "{code}");
            assert_eq!(view.can_choose_nickname, code != "nickname_locked", "{code}");
        }
    }

    #[test]
    fn test_the_nickname_goes_once_even_reopening_the_sheet() {
        let app = LogNApp::default();
        let mut model = board_session();
        model.profile.anon_number = 2207;
        let mut cmd = app.update(Event::NicknameSubmitted("ana_dev".into()), &mut model);
        assert_eq!(http_requests(&mut cmd).len(), 1);
        let mut cmd = app.update(Event::NicknameSubmitted("ana_dev".into()), &mut model);
        assert!(http_requests(&mut cmd).is_empty(), "segundo toque com o PUT no ar");

        let _ = app.update(Event::NicknameFlowClosed, &mut model);
        let _ = app.update(Event::NicknameStarted, &mut model);
        let mut cmd = app.update(Event::NicknameSubmitted("outro_nome".into()), &mut model);
        assert!(http_requests(&mut cmd).is_empty(), "reabrir a folha não destrava outro envio");

        // A resposta do primeiro ainda grava.
        let _ = app.update(Event::NicknameSaved {
            owner: "u1".into(), draft: "ana_dev".into(),
            result: http(200, serde_json::json!({ "nickname": "ana_dev" })),
        }, &mut model);
        assert_eq!(app.view(&model).profile_nickname.as_deref(), Some("ana_dev"));
    }

    #[test]
    fn test_a_nickname_answer_nobody_waits_for_is_dropped() {
        let app = LogNApp::default();
        let mut model = board_session();
        model.profile.anon_number = 2207;
        let ok = || http(200, serde_json::json!({ "nickname": "ana_dev" }));

        // De outra conta.
        let _ = app.update(Event::NicknameSubmitted("ana_dev".into()), &mut model);
        let _ = app.update(Event::NicknameSaved { owner: "u0".into(), draft: "ana_dev".into(), result: ok() }, &mut model);
        assert!(model.profile.nickname.is_none());

        // Depois de sair: a folha foi zerada, e o retrato não é regravado.
        let _ = app.update(Event::Logout, &mut model);
        let mut cmd = app.update(Event::NicknameSaved { owner: "u1".into(), draft: "ana_dev".into(), result: ok() }, &mut model);
        assert!(kv_ops(&mut cmd).is_empty());
        assert!(model.profile.nickname.is_none());
    }

    #[test]
    fn test_a_nickname_retry_only_goes_with_the_confirmation_open() {
        let app = LogNApp::default();
        let mut model = board_session();
        model.profile.anon_number = 2207;
        let _ = app.update(Event::NicknameChecked("ana_dev".into()), &mut model);
        let _ = app.update(Event::NicknameSubmitted("ana_dev".into()), &mut model);
        let _ = app.update(Event::NicknameSaved { owner: "u1".into(), draft: "ana_dev".into(), result: api_error(401, "unauthenticated") }, &mut model);
        assert!(matches!(model.pending_retry_event, Some(Event::NicknameSubmitted(_))));

        // Fechou a folha antes de a sessão voltar: o envio não sai sozinho depois.
        let _ = app.update(Event::NicknameFlowClosed, &mut model);
        assert!(model.pending_retry_event.is_none());
    }

    #[test]
    fn test_no_session_asks_nothing() {
        let app = LogNApp::default();
        let mut model = Model { user_id: "u1".into(), profile: crate::leaderboard::ProfileIdentity { anon_number: 2207, ..Default::default() }, ..Model::default() };
        let mut cmd = app.update(Event::LeaderboardRefresh, &mut model);
        assert!(http_requests(&mut cmd).is_empty());
        let mut cmd = app.update(Event::NicknameSubmitted("ana_dev".into()), &mut model);
        assert!(http_requests(&mut cmd).is_empty());
        assert_eq!(app.view(&model).nickname_flow.error, StatusKey::NoConnection);
    }

    #[test]
    fn test_a_late_board_after_logout_is_not_stored() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = app.update(Event::LeaderboardOpened, &mut model);
        let _ = app.update(Event::Logout, &mut model);
        let mut cmd = app.update(Event::LeaderboardFetched { owner: "u1".into(), result: board_body(true, Some(2), None, 100) }, &mut model);
        assert!(kv_ops(&mut cmd).is_empty(), "o placar apagado na saída não volta ao disco");
        assert!(model.leaderboard.is_none());
    }

    #[test]
    fn test_another_account_in_the_session_starts_its_own_board() {
        let app = LogNApp::default();
        let mut model = board_session();
        let _ = app.update(Event::LeaderboardOpened, &mut model);
        model.profile = crate::leaderboard::ProfileIdentity { owner: "u1".into(), anon_number: 2207, ..Default::default() };

        // A sessão venceu e outra conta entrou, sem passar pela saída.
        model.user_id = "u2".into();
        let view = app.view(&model);
        assert_eq!(view.profile_anon_number, 0, "o nome de u1 não aparece para u2");
        let mut cmd = app.update(Event::LeaderboardOpened, &mut model);
        let (kv, http) = drain(&mut cmd);
        assert_eq!(http.len(), 1, "o pedido de u1 no ar não trava o de u2");
        assert!(matches!(&kv[..], [KeyValueOperation::Get { .. }]), "o disco é relido para u2");
        // A resposta de u1 chega depois e fica de fora.
        let _ = app.update(Event::LeaderboardFetched { owner: "u1".into(), result: board_body(true, Some(2), None, 100) }, &mut model);
        assert!(model.leaderboard.is_none());
        assert_eq!(app.view(&model).leaderboard.state, crate::domain::LeaderboardState::Loading,
            "u2 segue esperando o pedido dele");
        assert_eq!(model.leaderboard_in_flight.as_deref(), Some("u2"));
    }

    #[test]
    fn test_an_old_snapshot_without_the_name_still_reads() {
        let snap: OfflineSnapshot = serde_json::from_str(
            r#"{"global_xp":10,"bugs_found":0,"dry_runs_completed":0,"nodes":[],"challenges":[]}"#,
        ).unwrap();
        assert_eq!(snap.anon_number, 0);
        assert!(snap.nickname.is_none());
    }
}
