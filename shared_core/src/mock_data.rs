use crate::domain::{ScoreCell, ScoreCellState, ScoreboardRow, StandingRow};

/// Traduz a notação compacta do design numa célula do telão:
/// `+/41` aceito de primeira no minuto 41 · `+2/88` aceito com 2 tentativas erradas ·
/// `-` ou `-3` tentado sem AC · `?` submetido após o congelamento · `.` não tentado.
fn parse_score_cell(token: &str) -> ScoreCell {
    if token == "." {
        return ScoreCell { state: ScoreCellState::Untried, top: String::new(), bottom: String::new() };
    }
    if token == "?" {
        return ScoreCell { state: ScoreCellState::Frozen, top: "?".into(), bottom: "frz".into() };
    }
    if let Some(rest) = token.strip_prefix('-') {
        // O sinal exibido é o de menos tipográfico (U+2212), não o hífen.
        return ScoreCell {
            state: ScoreCellState::Failed,
            top: format!("\u{2212}{}", rest),
            bottom: String::new(),
        };
    }
    if let Some(rest) = token.strip_prefix('+') {
        let (attempts, minute) = rest.split_once('/').unwrap_or((rest, ""));
        return ScoreCell {
            state: ScoreCellState::Accepted,
            top: format!("+{}", attempts),
            bottom: minute.to_string(),
        };
    }
    ScoreCell { state: ScoreCellState::Untried, top: String::new(), bottom: String::new() }
}

/// `solved` é **derivado** das células, não copiado do mock.
///
/// Os números de SLV do design não reconciliam com as células desenhadas (a primeira
/// linha mostra 9 com 8 células aceitas, a quinta mostra 7 com 5). Num mock isso não
/// importa; numa tela, alguém conta os verdes. O congelamento é justamente o estado em
/// que não se sabe o resultado, então um `?` não entra na conta.
fn scoreboard_row(rank: i32, team: &str, uni: &str, penalty: i32, is_user: bool, cells: &str) -> ScoreboardRow {
    let cells: Vec<ScoreCell> = cells.split_whitespace().map(parse_score_cell).collect();
    let solved = cells.iter().filter(|c| c.state == ScoreCellState::Accepted).count() as i32;
    ScoreboardRow {
        rank,
        team: team.into(),
        university: uni.into(),
        solved,
        penalty,
        is_user,
        cells,
    }
}

/// Telão do ginásio. Os dados são os do mock do design system.
pub fn get_mock_scoreboard() -> Vec<ScoreboardRow> {
    vec![
        scoreboard_row(1, "Ctrl Alt Defeat", "ITA", 721, false,
            "+1/23 +/41 +2/88 +/12 +/55 +3/140 +/31 ? +/97 - . . ."),
        scoreboard_row(2, "Segmentation Vault", "USP", 844, false,
            "+/19 +/37 +1/94 +/21 +2/61 +/155 +/44 ? +4/128 . . . ."),
        scoreboard_row(3, "Nlog N Roll", "UNICAMP", 690, false,
            "+/25 +/33 +/79 +1/18 +/72 - +/38 ? +/110 . . . ."),
        scoreboard_row(4, "Dijkstra Girls", "UFMG", 812, false,
            "+2/31 +/45 +/101 +/27 +/68 -2 +/52 . +/121 . . . ."),
        scoreboard_row(5, "Heap Overflow", "UFRGS", 658, false,
            "+/28 +/40 - +/22 +/74 . +/49 ? . . . . ."),
        scoreboard_row(42, "você", "UFC", 512, true,
            "+/34 +/58 -3 +1/29 . . +/91 ? . . . . ."),
    ]
}

fn standing(rank: i32, handle: &str, uni: &str, solved: i32, penalty: i32, is_user: bool, note: &str) -> StandingRow {
    StandingRow {
        rank,
        handle: handle.into(),
        university: uni.into(),
        solved,
        penalty,
        is_user,
        note: note.into(),
    }
}

/// Ranking global. A linha do usuário fica fora da lista — ela gruda na base.
pub fn get_mock_standings_global() -> Vec<StandingRow> {
    vec![
        standing(1, "tourist_br", "ITA", 9, 721, false, ""),
        standing(2, "lupa.dp", "USP", 9, 844, false, ""),
        standing(3, "bfs_enjoyer", "UNICAMP", 8, 690, false, ""),
        standing(4, "seg.tree", "UFMG", 8, 812, false, ""),
        standing(5, "kmp.rodrigo", "UFRGS", 7, 658, false, ""),
    ]
}

/// Ranking da sede. Mesma casa do usuário, então ele aparece no topo.
pub fn get_mock_standings_home() -> Vec<StandingRow> {
    vec![
        standing(1, "bit.mask", "UFC", 6, 540, false, ""),
        standing(2, "trie.hard", "UFC", 5, 498, false, ""),
        standing(3, "dsu.rodrigo", "UFC", 4, 610, false, ""),
    ]
}

/// A linha do jogador, em ambas as abas.
pub fn get_mock_user_standing() -> StandingRow {
    standing(42, "você", "UFC", 5, 512, true, "subiu 6 nesta rodada")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parses_the_four_cell_states() {
        let accepted_first_try = parse_score_cell("+/41");
        assert_eq!(accepted_first_try.state, ScoreCellState::Accepted);
        assert_eq!(accepted_first_try.top, "+");
        assert_eq!(accepted_first_try.bottom, "41");

        let accepted_after_retries = parse_score_cell("+2/88");
        assert_eq!(accepted_after_retries.state, ScoreCellState::Accepted);
        assert_eq!(accepted_after_retries.top, "+2");
        assert_eq!(accepted_after_retries.bottom, "88");

        let failed = parse_score_cell("-3");
        assert_eq!(failed.state, ScoreCellState::Failed);
        assert_eq!(failed.top, "\u{2212}3", "o sinal é o menos tipográfico, não o hífen");

        let frozen = parse_score_cell("?");
        assert_eq!(frozen.state, ScoreCellState::Frozen);
        assert_eq!(frozen.bottom, "frz");

        let untried = parse_score_cell(".");
        assert_eq!(untried.state, ScoreCellState::Untried);
        assert!(untried.top.is_empty() && untried.bottom.is_empty());
    }

    #[test]
    fn test_every_scoreboard_row_has_thirteen_cells() {
        // A—M são treze letras: uma célula por problema, sempre.
        for row in get_mock_scoreboard() {
            assert_eq!(row.cells.len(), 13, "equipe {} tem {} células", row.team, row.cells.len());
        }
    }

    /// A tela nunca pode se contradizer: o contador tem de bater com os verdes.
    #[test]
    fn test_solved_count_matches_the_accepted_cells() {
        for row in get_mock_scoreboard() {
            let accepted = row.cells.iter().filter(|c| c.state == ScoreCellState::Accepted).count();
            assert_eq!(accepted as i32, row.solved, "equipe {}", row.team);
        }
    }

    /// Um `?` é submissão de resultado desconhecido — não conta como resolvido.
    #[test]
    fn test_frozen_cells_do_not_count_as_solved() {
        let row = scoreboard_row(1, "T", "U", 100, false, "+/10 ? ? . . . . . . . . . .");
        assert_eq!(row.solved, 1);
    }

    #[test]
    fn test_exactly_one_user_row_in_the_scoreboard() {
        assert_eq!(get_mock_scoreboard().iter().filter(|r| r.is_user).count(), 1);
    }
}
