//! O placar geral de XP (docs/specs/logn_placar_spec.md): a resposta do servidor, o
//! que fica guardado no aparelho e a tela que sai disso. Tudo aqui é puro; os efeitos
//! moram em `app.rs`.

use serde::{Deserialize, Serialize};

use crate::domain::{
    LeaderboardAgeUnit, LeaderboardMeStatus, LeaderboardRow, LeaderboardState, LeaderboardView,
};

/// Chave do placar guardado. Uma só, com o dono dentro: chave por conta deixaria a
/// lista de quem a sessão venceu sem `Logout` parada no aparelho para sempre.
pub const LEADERBOARD_KEY: &str = "leaderboard";

/// Uma linha como `GET /api/v1/leaderboard` manda.
#[derive(Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
pub struct BoardRow {
    #[serde(default)]
    pub rank: i32,
    #[serde(default)]
    pub anon_number: i32,
    #[serde(default)]
    pub nickname: Option<String>,
    #[serde(default)]
    pub xp: i32,
    #[serde(default)]
    pub is_me: bool,
}

/// A linha de quem pediu. `rank` nulo: placar fechado, sem XP ou oculto.
#[derive(Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
pub struct BoardMe {
    #[serde(default)]
    pub rank: Option<i32>,
    #[serde(default)]
    pub anon_number: i32,
    #[serde(default)]
    pub nickname: Option<String>,
    #[serde(default)]
    pub xp: i32,
    #[serde(default)]
    pub hidden: bool,
}

/// O corpo de `GET /api/v1/leaderboard`. Vai para o disco em JSON, então todo campo
/// tem `default` e nenhum muda de nome (regra de estabilidade do ROADMAP).
#[derive(Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
pub struct Board {
    #[serde(default)]
    pub open: bool,
    #[serde(default)]
    pub missing: i32,
    #[serde(default)]
    pub threshold: i32,
    #[serde(default)]
    pub rows: Vec<BoardRow>,
    #[serde(default)]
    pub me: BoardMe,
    #[serde(default)]
    pub generated_at: i64,
}

/// O placar guardado, com o dono: lido por outra conta, é descartado.
#[derive(Serialize, Deserialize, Clone, Debug, Default, PartialEq)]
pub struct LeaderboardCache {
    #[serde(default)]
    pub owner: String,
    #[serde(default)]
    pub board: Board,
}

/// O nome do placar da conta em sessão, como o servidor disse por último.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct ProfileIdentity {
    /// De que conta é. Vazio é o do retrato offline, que é sempre da conta do aparelho
    /// (a saída o apaga). Outra conta não vê este nome.
    pub owner: String,
    /// 0 enquanto o servidor não disse.
    pub anon_number: i32,
    pub nickname: Option<String>,
    /// A conta já escolheu, ou a moderação queimou a chance.
    pub nickname_locked: bool,
}

impl ProfileIdentity {
    /// O nome vale para a conta `user_id`.
    pub fn belongs_to(&self, user_id: &str) -> bool {
        self.owner.is_empty() || self.owner == user_id
    }
}

/// A folha de escolher apelido.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct NicknameFlow {
    /// De que conta é a folha: a resposta de outra não entra.
    pub owner: String,
    pub step: crate::domain::NicknameStep,
    pub draft: String,
    /// O PUT está no ar. Fechar e reabrir a folha não destrava um segundo envio.
    pub submitting: bool,
    /// O envio levou 401 e espera a sessão ser renovada para ir de novo.
    pub retry: bool,
    pub error: crate::domain::StatusKey,
}

/// O que o placar precisa saber da sessão para decidir a tela.
pub struct BoardContext<'a> {
    pub signed_in: bool,
    pub cache: Option<&'a LeaderboardCache>,
    pub in_flight: bool,
    /// A leitura do placar guardado ainda não voltou: até lá é "carregando", não "sem
    /// conexão".
    pub disk_pending: bool,
    pub failed: bool,
    pub session_offline: bool,
    pub now: i64,
}

/// A mesma regra do servidor (`NormalizeNickname`): espaços das pontas fora, minúscula
/// só de ASCII, e 3 a 20 de `a-z0-9_`. A lista de reservados fica só no servidor: duas
/// listas iam divergir.
pub fn normalize_nickname(raw: &str) -> Option<String> {
    let n: String = raw.trim().chars().map(|c| c.to_ascii_lowercase()).collect();
    let ok = (3..=20).contains(&n.len())
        && n.bytes().all(|b| b.is_ascii_lowercase() || b.is_ascii_digit() || b == b'_');
    ok.then_some(n)
}

/// Há quanto tempo a lista foi gerada, na unidade que a tela mostra.
pub fn age(now: i64, generated_at: i64) -> (LeaderboardAgeUnit, u32) {
    let secs = (now - generated_at).max(0);
    if now == 0 || generated_at == 0 || secs < 60 {
        (LeaderboardAgeUnit::JustNow, 0)
    } else if secs < 3600 {
        (LeaderboardAgeUnit::Minutes, (secs / 60) as u32)
    } else if secs < 86_400 {
        (LeaderboardAgeUnit::Hours, (secs / 3600) as u32)
    } else {
        (LeaderboardAgeUnit::Days, (secs / 86_400) as u32)
    }
}

fn row(r: &BoardRow) -> LeaderboardRow {
    LeaderboardRow {
        rank: r.rank,
        anon_number: r.anon_number,
        nickname: r.nickname.clone(),
        xp: r.xp,
        is_me: r.is_me,
    }
}

/// A tela do placar.
pub fn view(ctx: &BoardContext) -> LeaderboardView {
    if !ctx.signed_in {
        return LeaderboardView::default();
    }
    let Some(cache) = ctx.cache else {
        let state = if !ctx.disk_pending && (ctx.failed || (ctx.session_offline && !ctx.in_flight)) {
            LeaderboardState::Unavailable
        } else {
            LeaderboardState::Loading
        };
        return LeaderboardView { state, ..LeaderboardView::default() };
    };

    let board = &cache.board;
    let rows: Vec<LeaderboardRow> = if board.open { board.rows.iter().map(row).collect() } else { Vec::new() };
    let me = LeaderboardRow {
        rank: board.me.rank.unwrap_or(0),
        anon_number: board.me.anon_number,
        nickname: board.me.nickname.clone(),
        xp: board.me.xp,
        is_me: true,
    };
    let me_status = if board.me.hidden {
        LeaderboardMeStatus::Hidden
    } else if board.me.xp <= 0 {
        LeaderboardMeStatus::ZeroXp
    } else if board.open && board.me.rank.is_some() {
        LeaderboardMeStatus::Ranked
    } else {
        LeaderboardMeStatus::Waiting
    };
    let (age_unit, age_value) = age(ctx.now, board.generated_at);
    LeaderboardView {
        state: if board.open { LeaderboardState::Open } else { LeaderboardState::Closed },
        missing: board.missing.max(0) as u32,
        threshold: board.threshold.max(0) as u32,
        me_in_rows: rows.iter().any(|r| r.is_me),
        rows,
        me,
        me_status,
        age_unit,
        age_value,
        refreshing: ctx.in_flight,
        offline: ctx.failed || ctx.session_offline,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn cache(board: Board) -> LeaderboardCache {
        LeaderboardCache { owner: "u1".into(), board }
    }

    fn ctx(cache: Option<&LeaderboardCache>) -> BoardContext<'_> {
        BoardContext { signed_in: true, cache, in_flight: false, disk_pending: false, failed: false, session_offline: false, now: 1_000_000 }
    }

    #[test]
    fn normalize_nickname_follows_the_server_rule() {
        for (raw, want) in [
            ("ana_dev", Some("ana_dev")),
            ("  Ana_Dev ", Some("ana_dev")),
            ("abc", Some("abc")),
            ("ab", None),
            ("aaaaaaaaaaaaaaaaaaaaa", None),
            ("joão", None),
            ("ana dev", None),
            ("ana-dev", None),
            ("\u{212A}bfs", None),
        ] {
            assert_eq!(normalize_nickname(raw).as_deref(), want, "{raw:?}");
        }
    }

    #[test]
    fn signed_out_is_the_call_to_create_an_account() {
        let c = BoardContext { signed_in: false, ..ctx(None) };
        assert_eq!(view(&c).state, LeaderboardState::SignedOut);
    }

    #[test]
    fn nothing_stored_is_loading_then_unavailable_on_failure() {
        assert_eq!(view(&ctx(None)).state, LeaderboardState::Loading);
        let failed = BoardContext { failed: true, ..ctx(None) };
        assert_eq!(view(&failed).state, LeaderboardState::Unavailable);
        // Sem rede, mas o disco ainda vai responder: carregando, não "conecte".
        let reading = BoardContext { session_offline: true, disk_pending: true, ..ctx(None) };
        assert_eq!(view(&reading).state, LeaderboardState::Loading);
    }

    #[test]
    fn a_closed_board_shows_no_rank() {
        let c = cache(Board {
            open: false, missing: 3, threshold: 10,
            rows: vec![BoardRow { rank: 1, is_me: true, xp: 50, ..Default::default() }],
            me: BoardMe { rank: None, xp: 50, anon_number: 2207, ..Default::default() },
            generated_at: 1_000_000,
        });
        let v = view(&ctx(Some(&c)));
        assert_eq!(v.state, LeaderboardState::Closed);
        assert_eq!((v.missing, v.threshold), (3, 10));
        assert!(v.rows.is_empty());
        assert_eq!(v.me.rank, 0);
        assert_eq!(v.me_status, LeaderboardMeStatus::Waiting);
    }

    #[test]
    fn me_status_covers_the_four_cases() {
        let open = |me: BoardMe| cache(Board { open: true, threshold: 10, me, ..Default::default() });
        let ranked = open(BoardMe { rank: Some(4), xp: 50, ..Default::default() });
        let zero = open(BoardMe { rank: None, xp: 0, ..Default::default() });
        let hidden = open(BoardMe { rank: None, xp: 50, hidden: true, ..Default::default() });
        let closed = cache(Board { open: false, me: BoardMe { xp: 50, ..Default::default() }, ..Default::default() });
        assert_eq!(view(&ctx(Some(&ranked))).me_status, LeaderboardMeStatus::Ranked);
        assert_eq!(view(&ctx(Some(&zero))).me_status, LeaderboardMeStatus::ZeroXp);
        assert_eq!(view(&ctx(Some(&hidden))).me_status, LeaderboardMeStatus::Hidden);
        assert_eq!(view(&ctx(Some(&closed))).me_status, LeaderboardMeStatus::Waiting);
    }

    #[test]
    fn a_failed_refresh_keeps_the_list_and_flags_it_offline() {
        let c = cache(Board {
            open: true, threshold: 10,
            rows: vec![BoardRow { rank: 1, xp: 900, ..Default::default() }, BoardRow { rank: 2, xp: 50, is_me: true, ..Default::default() }],
            me: BoardMe { rank: Some(2), xp: 50, ..Default::default() },
            generated_at: 1_000_000 - 7200,
            ..Default::default()
        });
        let v = view(&BoardContext { failed: true, ..ctx(Some(&c)) });
        assert_eq!(v.state, LeaderboardState::Open);
        assert!(v.offline && v.me_in_rows);
        assert_eq!(v.rows.len(), 2);
        assert_eq!((v.age_unit, v.age_value), (LeaderboardAgeUnit::Hours, 2));
    }

    #[test]
    fn age_picks_the_unit() {
        assert_eq!(age(1000, 990), (LeaderboardAgeUnit::JustNow, 0));
        assert_eq!(age(10_000, 10_000 - 300), (LeaderboardAgeUnit::Minutes, 5));
        assert_eq!(age(200_000, 200_000 - 3 * 86_400), (LeaderboardAgeUnit::Days, 3));
        assert_eq!(age(0, 500), (LeaderboardAgeUnit::JustNow, 0));
    }

    #[test]
    fn a_stored_board_without_new_fields_still_reads() {
        let c: LeaderboardCache = serde_json::from_str(r#"{"owner":"u1","board":{"open":true}}"#).unwrap();
        assert!(c.board.open && c.board.rows.is_empty());
    }
}
