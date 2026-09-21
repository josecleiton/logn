use facet::Facet;
use facet_generate_attrs as fg;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Estado de uma partida (Match/Contest) no modo Arena.
/// Toda a lógica de jogo reside aqui para manter a UI "burra".
#[derive(Clone, Debug, Default)]
pub struct MatchState {
    pub is_active: bool,
    pub problems: Vec<MatchProblem>,
    pub current_index: usize,
    pub verdicts: HashMap<char, VerdictCode>,
    pub lives: i32,
    pub max_lives: i32,
    pub penalty_minutes: i32,
    pub attempts: HashMap<char, i32>,
    pub contest_seconds_remaining: i32,
    pub question_seconds_remaining: i32,
    pub is_frozen: bool,
    pub trap: Option<TrapInfo>,
    pub selection: MatchSelection,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum VerdictCode {
    Pending,
    Accepted,
    WrongAnswer,
    TimeLimitExceeded,
    MemoryLimitExceeded,
    RuntimeError,
    CompileError,
    PresentationError,
}

impl Default for VerdictCode {
    fn default() -> Self { VerdictCode::Pending }
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct MatchProblem {
    pub letter: String, // "A", "B", ..., "M"
    pub challenge_id: String,
    pub template_type: String,
    pub title: String,
    pub description: String,
    pub code_lines: Vec<String>,
    pub correct_line: Option<i32>,
    pub expected_string: Option<String>,
    pub options: Vec<String>,       // Para COMPLEXITY_MATCH e TAG_THE_PATTERN
    pub correct_options: Vec<String>, // Respostas corretas
    pub max_selections: i32,        // Para TAG_THE_PATTERN
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct TrapInfo {
    pub name: String,
    pub category: String,   // ex: "TRAP CLÁSSICA"
    pub title: String,
    pub explanation: String,
}

#[derive(Clone, Debug, Default)]
pub struct MatchSelection {
    pub selected_line: Option<i32>,         // SPOT_THE_BUG
    pub answer_string: Option<String>,      // FILL_IN_THE_BLANK
    pub drop_time: Option<String>,          // COMPLEXITY_MATCH
    pub drop_space: Option<String>,         // COMPLEXITY_MATCH
    pub selected_tags: Vec<String>,         // TAG_THE_PATTERN
}

/// ViewModel da partida para a UI.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default)]
#[facet(fg::namespace = "LogN")]
pub struct MatchViewModel {
    pub is_active: bool,
    pub current_letter: String,
    pub current_title: String,
    pub current_description: String,
    pub current_template_type: String,
    pub current_code_lines: Vec<String>,
    pub current_options: Vec<String>,
    pub max_selections: i32,
    pub lives: i32,
    pub max_lives: i32,
    pub penalty_minutes: i32,
    pub contest_seconds: i32,
    pub question_seconds: i32,
    pub is_frozen: bool,
    pub total_problems: i32,
    pub solved_count: i32,
    pub balloon_states: Vec<BalloonState>,
    pub selected_line: i32,              // -1 = nenhuma
    pub answer_string: String,
    pub drop_time: String,
    pub drop_space: String,
    pub selected_tags: Vec<String>,
    pub last_verdict: String,             // "", "AC", "WA", etc.
    pub has_trap: bool,
    pub trap_category: String,
    pub trap_title: String,
    pub trap_explanation: String,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct BalloonState {
    pub letter: String,
    pub is_accepted: bool,
}

impl MatchState {
    pub fn new(problems: Vec<MatchProblem>) -> Self {
        let letters: Vec<char> = problems.iter().filter_map(|p| p.letter.chars().next()).collect();
        let verdicts: HashMap<char, VerdictCode> = letters.iter().map(|&l| (l, VerdictCode::Pending)).collect();
        let attempts: HashMap<char, i32> = letters.iter().map(|&l| (l, 0)).collect();

        MatchState {
            is_active: true,
            problems,
            current_index: 0,
            verdicts,
            lives: 3,
            max_lives: 3,
            penalty_minutes: 0,
            attempts,
            contest_seconds_remaining: 180, // 3 minutos por sessão
            question_seconds_remaining: 60,
            is_frozen: false,
            trap: None,
            selection: MatchSelection::default(),
        }
    }

    pub fn current_problem(&self) -> Option<&MatchProblem> {
        self.problems.get(self.current_index)
    }

    pub fn current_letter(&self) -> char {
        self.current_problem()
            .and_then(|p| p.letter.chars().next())
            .unwrap_or('?')
    }

    /// Submete a resposta do problema atual. Retorna o veredito.
    pub fn submit(&mut self) -> VerdictCode {
        let problem = match self.problems.get(self.current_index) {
            Some(p) => p.clone(),
            None => return VerdictCode::WrongAnswer,
        };

        let letter = problem.letter.chars().next().unwrap_or('?');
        *self.attempts.entry(letter).or_insert(0) += 1;

        let is_correct = match problem.template_type.as_str() {
            "SPOT_THE_BUG" => {
                self.selection.selected_line == problem.correct_line
            }
            "FILL_IN_THE_BLANK" => {
                self.selection.answer_string.as_deref().map(|s| s.trim())
                    == problem.expected_string.as_deref().map(|s| s.trim())
            }
            "COMPLEXITY_MATCH" => {
                if problem.correct_options.len() >= 2 {
                    self.selection.drop_time.as_deref() == Some(&problem.correct_options[0])
                        && self.selection.drop_space.as_deref() == Some(&problem.correct_options[1])
                } else {
                    false
                }
            }
            "TAG_THE_PATTERN" => {
                let mut selected = self.selection.selected_tags.clone();
                let mut correct = problem.correct_options.clone();
                selected.sort();
                correct.sort();
                selected == correct
            }
            _ => false,
        };

        let verdict = if is_correct {
            VerdictCode::Accepted
        } else {
            VerdictCode::WrongAnswer
        };

        self.verdicts.insert(letter, verdict.clone());

        if !is_correct {
            self.lives -= 1;
            self.penalty_minutes += 20;
        }

        // Limpa seleção e avança
        self.selection = MatchSelection::default();

        if is_correct || self.lives > 0 {
            // Avança para o próximo problema não-resolvido
            self.advance();
        }

        // Checa fim de jogo
        if self.lives <= 0 || self.all_solved() || self.current_index >= self.problems.len() {
            self.is_active = false;
        }

        verdict
    }

    fn advance(&mut self) {
        let start = self.current_index;
        loop {
            self.current_index = (self.current_index + 1) % self.problems.len();
            let letter = self.problems[self.current_index].letter.chars().next().unwrap_or('?');
            if self.verdicts.get(&letter) != Some(&VerdictCode::Accepted) {
                break;
            }
            if self.current_index == start {
                // Todos resolvidos
                self.current_index = self.problems.len();
                break;
            }
        }
        self.question_seconds_remaining = 60;
    }

    fn all_solved(&self) -> bool {
        self.verdicts.values().all(|v| *v == VerdictCode::Accepted)
    }

    pub fn solved_count(&self) -> i32 {
        self.verdicts.values().filter(|v| **v == VerdictCode::Accepted).count() as i32
    }

    pub fn to_view_model(&self) -> MatchViewModel {
        let problem = self.current_problem();

        let balloon_states: Vec<BalloonState> = self.problems.iter().map(|p| {
            let letter = p.letter.chars().next().unwrap_or('?');
            BalloonState {
                letter: p.letter.clone(),
                is_accepted: self.verdicts.get(&letter) == Some(&VerdictCode::Accepted),
            }
        }).collect();

        MatchViewModel {
            is_active: self.is_active,
            current_letter: problem.map(|p| p.letter.clone()).unwrap_or_default(),
            current_title: problem.map(|p| p.title.clone()).unwrap_or_default(),
            current_description: problem.map(|p| p.description.clone()).unwrap_or_default(),
            current_template_type: problem.map(|p| p.template_type.clone()).unwrap_or_default(),
            current_code_lines: problem.map(|p| p.code_lines.clone()).unwrap_or_default(),
            current_options: problem.map(|p| p.options.clone()).unwrap_or_default(),
            max_selections: problem.map(|p| p.max_selections).unwrap_or(1),
            lives: self.lives,
            max_lives: self.max_lives,
            penalty_minutes: self.penalty_minutes,
            contest_seconds: self.contest_seconds_remaining,
            question_seconds: self.question_seconds_remaining,
            is_frozen: self.is_frozen,
            total_problems: self.problems.len() as i32,
            solved_count: self.solved_count(),
            balloon_states,
            selected_line: self.selection.selected_line.unwrap_or(-1),
            answer_string: self.selection.answer_string.clone().unwrap_or_default(),
            drop_time: self.selection.drop_time.clone().unwrap_or_default(),
            drop_space: self.selection.drop_space.clone().unwrap_or_default(),
            selected_tags: self.selection.selected_tags.clone(),
            last_verdict: String::new(),
            has_trap: self.trap.is_some(),
            trap_category: self.trap.as_ref().map(|t| t.category.clone()).unwrap_or_default(),
            trap_title: self.trap.as_ref().map(|t| t.title.clone()).unwrap_or_default(),
            trap_explanation: self.trap.as_ref().map(|t| t.explanation.clone()).unwrap_or_default(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample_problems() -> Vec<MatchProblem> {
        vec![
            MatchProblem {
                letter: "A".into(),
                challenge_id: "ch1".into(),
                template_type: "SPOT_THE_BUG".into(),
                title: "A · Soma de Dois Números".into(),
                description: "Encontre o bug.".into(),
                code_lines: vec!["int a = 0;".into(), "while (a < b)".into()],
                correct_line: Some(1),
                expected_string: None,
                options: vec![],
                correct_options: vec![],
                max_selections: 0,
            },
            MatchProblem {
                letter: "B".into(),
                challenge_id: "ch2".into(),
                template_type: "FILL_IN_THE_BLANK".into(),
                title: "B · Maior de Dois".into(),
                description: "Complete o retorno.".into(),
                code_lines: vec!["int max2(int a, int b) {".into(), "    return a > b ? _____ : b;".into()],
                correct_line: None,
                expected_string: Some("a".into()),
                options: vec![],
                correct_options: vec![],
                max_selections: 0,
            },
            MatchProblem {
                letter: "C".into(),
                challenge_id: "ch3".into(),
                template_type: "TAG_THE_PATTERN".into(),
                title: "C · Maior de Três".into(),
                description: "Qual pattern?".into(),
                code_lines: vec![],
                correct_line: None,
                expected_string: None,
                options: vec!["Grafos".into(), "BFS".into(), "DP".into(), "Greedy".into()],
                correct_options: vec!["Grafos".into(), "BFS".into()],
                max_selections: 2,
            },
        ]
    }

    #[test]
    fn test_match_correct_answer() {
        let mut state = MatchState::new(sample_problems());
        assert_eq!(state.lives, 3);
        assert!(state.is_active);

        // Select correct line for problem A (SPOT_THE_BUG)
        state.selection.selected_line = Some(1);
        let verdict = state.submit();
        assert_eq!(verdict, VerdictCode::Accepted);
        assert_eq!(state.lives, 3); // Não perde vida
        assert_eq!(state.solved_count(), 1);
    }

    #[test]
    fn test_match_wrong_answer_loses_life() {
        let mut state = MatchState::new(sample_problems());

        // Wrong line
        state.selection.selected_line = Some(0);
        let verdict = state.submit();
        assert_eq!(verdict, VerdictCode::WrongAnswer);
        assert_eq!(state.lives, 2);
        assert_eq!(state.penalty_minutes, 20);
    }

    #[test]
    fn test_match_game_over_on_3_wrongs() {
        let mut state = MatchState::new(sample_problems());

        for _ in 0..3 {
            state.selection.selected_line = Some(0); // Wrong
            state.submit();
        }

        assert_eq!(state.lives, 0);
        assert!(!state.is_active);
    }

    #[test]
    fn test_tag_the_pattern() {
        let mut state = MatchState::new(sample_problems());
        // Solve A first
        state.selection.selected_line = Some(1);
        state.submit();
        // Solve B
        state.selection.answer_string = Some("a".into());
        state.submit();
        // Now on C (TAG_THE_PATTERN)
        state.selection.selected_tags = vec!["Grafos".into(), "BFS".into()];
        let verdict = state.submit();
        assert_eq!(verdict, VerdictCode::Accepted);
    }
}
