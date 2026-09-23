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
    /// Relógio parado enquanto o jogador lê o cartão de origem pela primeira vez.
    /// Vale uma vez por origem, e não por partida — ver `Model::origins_seen`.
    pub is_paused: bool,
    /// O cartão de confirmação de saída está aberto. Segura o relógio enquanto a
    /// pessoa decide: perguntar e continuar contando é cobrar pela pergunta.
    pub leave_pending: bool,
    pub is_frozen: bool,
    pub trap: Option<TrapInfo>,
    pub selection: MatchSelection,
    /// Veredito da última submissão. Dirige a tela de veredito em tela cheia.
    pub last_verdict: Option<VerdictCode>,
    /// Os erros da sessão, na ordem em que aconteceram.
    pub errors: Vec<MatchError>,
}

/// Descreve a resposta do jogador em texto, para a revisão. Vazio quando não respondeu.
/// Último recurso, quando o desafio não traz explicação própria. Fala do que dá para
/// falar sem conhecer o problema: os casos de borda.
pub const GENERIC_WRONG_ANSWER: &str =
    "A escolha não cobre todos os casos de entrada. Vale reler o enunciado olhando para os limites: o primeiro índice, o último, e o array vazio.";

fn describe_answer(template: &str, selection: &MatchSelection) -> String {
    match template {
        "SPOT_THE_BUG" => selection
            .selected_line
            .map(|l| format!("linha {}", l + 1))
            .unwrap_or_default(),
        "FILL_IN_THE_BLANK" => selection.answer_string.clone().unwrap_or_default(),
        "DRY_RUN" => selection.predicted_output.clone().unwrap_or_default(),
        "COMPLEXITY_MATCH" => match (&selection.drop_time, &selection.drop_space) {
            (Some(t), Some(s)) => format!("{} / {}", t, s),
            _ => String::new(),
        },
        "TAG_THE_PATTERN" => selection.selected_tags.join(", "),
        _ => String::new(),
    }
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

impl VerdictCode {
    /// A sigla do juiz, sem tradução — é ela que a UI exibe.
    pub fn code(&self) -> &'static str {
        match self {
            VerdictCode::Pending             => "",
            VerdictCode::Accepted            => "AC",
            VerdictCode::WrongAnswer         => "WA",
            VerdictCode::TimeLimitExceeded   => "TLE",
            VerdictCode::MemoryLimitExceeded => "MLE",
            VerdictCode::RuntimeError        => "RE",
            VerdictCode::CompileError        => "CE",
            VerdictCode::PresentationError   => "PE",
        }
    }
}

/// Quanto tempo cada template pede, medido e não chutado.
///
/// Três revisores cegos independentes cronometraram os mesmos cinco templates e o
/// resultado foi consistente: o custo é quase todo **do template**, não do desafio.
/// Prever saída exige simular estado passo a passo, e isso não tem atalho — as
/// estimativas ficaram entre 70 e 150 segundos contra os 60 que todos tinham.
///
/// Marcar padrão e completar lacuna são reconhecimento: quem sabe responde em 15
/// segundos, e quem não sabe não descobre com mais tempo.
pub fn seconds_for_template(template_type: &str) -> i32 {
    match template_type {
        // Simular estado é o trabalho caro, e é linear no tamanho da entrada.
        "DRY_RUN" => 150,
        // Ler o código inteiro procurando a linha, com o código na tela.
        "SPOT_THE_BUG" => 90,
        // Dois eixos e um raciocínio de custo em cada.
        "COMPLEXITY_MATCH" => 75,
        // Reconhecimento: ou se sabe, ou mais tempo não ajuda.
        _ => 60,
    }
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
    /// Índice da linha do bug em `code_lines`, **a partir de 0** — o mesmo que o
    /// toque devolve. O desafio a guarda a partir de 1; a conversão é feita ao montar.
    pub correct_line: Option<i32>,
    pub expected_string: Option<String>,
    /// Explicação própria do desafio para o erro. Vazia cai no texto genérico.
    pub explanation: String,
    pub options: Vec<String>,       // Para COMPLEXITY_MATCH e TAG_THE_PATTERN
    pub correct_options: Vec<String>, // Respostas corretas
    pub max_selections: i32,        // Para TAG_THE_PATTERN
    /// De onde o desafio veio. Vazio para o que nasceu aqui; a tela mostra um selo
    /// quando há algo.
    pub origin: String,
    /// Quanto tempo esta questão dá. Sai do template, a menos que o desafio traga
    /// `content.seconds` — o que é para o caso atípico, não para o comum.
    pub seconds: i32,
    pub watch_variables: Vec<crate::domain::WatchVariable>, // Para DRY_RUN
    pub watch_note: String,         // Para DRY_RUN
    /// O desafio já rendeu XP antes desta partida. Aceitar de novo não paga, e o
    /// relatório diz isso em vez de deixar o `+0` parecer defeito.
    pub already_paid: bool,
}

/// XP de um aceito que ainda não tinha pago. O servidor aplica a mesma regra.
pub const XP_PER_ACCEPTED: i32 = 50;

/// Um erro da sessão, guardado para a revisão do relatório pós-partida.
/// Um por problema errado — o relatório lista todos, não só o último.
#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct MatchError {
    pub letter: String,
    pub title: String,
    /// Sigla do juiz: `WA`, `TLE`, …
    pub verdict: String,
    /// O que o jogador respondeu, já em texto legível.
    pub given_answer: String,
    pub explanation: String,
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
    pub predicted_output: Option<String>,   // DRY_RUN
}

/// Normaliza a saída prevista do DRY_RUN: espaço em branco é ignorado, o resto não.
/// O jogador está prevendo um valor, não formatando uma saída de juiz.
pub fn normalize_output(s: &str) -> String {
    s.chars().filter(|c| !c.is_whitespace()).collect()
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
    /// Origem do problema atual, vazia quando ele nasceu aqui.
    pub current_origin: String,
    /// Origem cujo cartão de homenagem está aberto. Vazia quando não há cartão.
    pub origin_sheet: String,
    /// O cartão aberto está segurando o relógio. Só na primeira leitura de cada origem,
    /// e é isso que o cartão avisa ao jogador.
    pub origin_sheet_paused: bool,
    /// O cartão de confirmação de saída está na tela.
    pub leave_pending: bool,
    /// Quantos problemas do nó já foram aceitos — o que a pessoa deixa para trás se
    /// sair agora, e o número que o cartão mostra.
    pub solved_so_far: i32,
    pub lives: i32,
    pub max_lives: i32,
    pub penalty_minutes: i32,
    pub contest_seconds: i32,
    pub question_seconds: i32,
    pub is_frozen: bool,
    pub total_problems: i32,
    pub solved_count: i32,
    pub balloon_states: Vec<BalloonState>,
    /// XP que a partida rendeu: só os aceitos que ainda não tinham pago.
    pub xp_earned: i32,
    pub selected_line: i32,              // -1 = nenhuma
    pub answer_string: String,
    pub drop_time: String,
    pub drop_space: String,
    pub selected_tags: Vec<String>,
    pub predicted_output: String,         // DRY_RUN
    pub watch_variables: Vec<crate::domain::WatchVariable>, // DRY_RUN
    pub watch_note: String,               // DRY_RUN
    pub last_verdict: String,             // "", "AC", "WA", etc.
    pub errors: Vec<MatchError>,          // revisão do relatório pós-partida
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
    /// Já tinha rendido XP antes desta partida.
    pub already_paid: bool,
}

impl MatchState {
    pub fn new(problems: Vec<MatchProblem>) -> Self {
        let letters: Vec<char> = problems.iter().filter_map(|p| p.letter.chars().next()).collect();
        let verdicts: HashMap<char, VerdictCode> = letters.iter().map(|&l| (l, VerdictCode::Pending)).collect();
        let attempts: HashMap<char, i32> = letters.iter().map(|&l| (l, 0)).collect();

        // Os dois relógios saem daqui, antes de `problems` mudar de dono.
        let soma_segundos: i32 = problems.iter().map(|p| p.seconds).sum();
        let primeira_questao = problems.first().map_or(60, |p| p.seconds);

        MatchState {
            is_active: true,
            problems,
            current_index: 0,
            verdicts,
            lives: 3,
            max_lives: 3,
            penalty_minutes: 0,
            attempts,
            is_paused: false,
            leave_pending: false,
            // A sessão vale a soma do que os problemas deste nó pedem, mais um quinto de
            // folga para ler enunciado e trocar de tela.
            //
            // Era 180 fixo, e isso tornava o relógio da questão decorativo: com seis
            // problemas davam 30 segundos por problema em média, então ninguém
            // conseguia gastar o minuto em mais de três. Agora um nó com DRY_RUN
            // naturalmente dura mais que um sem, que é o que ele custa.
            contest_seconds_remaining: soma_segundos + soma_segundos / 5,
            question_seconds_remaining: primeira_questao,
            is_frozen: false,
            trap: None,
            selection: MatchSelection::default(),
            last_verdict: None,
            errors: Vec::new(),
        }
    }

    /// Registra um erro para a revisão. A resposta é lida antes da seleção ser limpa.
    fn record_error(&mut self, problem: &MatchProblem, verdict: &VerdictCode, explanation: &str) {
        self.errors.push(MatchError {
            letter: problem.letter.clone(),
            title: problem.title.clone(),
            verdict: verdict.code().to_string(),
            given_answer: describe_answer(&problem.template_type, &self.selection),
            explanation: explanation.to_string(),
        });
    }

    /// A última hora do contest, na escala de uma sessão de três minutos: o último
    /// terço. A partir daí o placar congela e ninguém sabe mais o resultado.
    pub const FREEZE_SECONDS: i32 = 60;

    /// Recalcula o congelamento a partir do relógio. Chamado a cada tick.
    pub fn refresh_freeze(&mut self) {
        self.is_frozen = self.is_active && self.contest_seconds_remaining <= Self::FREEZE_SECONDS;
    }

    pub fn current_problem(&self) -> Option<&MatchProblem> {
        self.problems.get(self.current_index)
    }

    pub fn current_letter(&self) -> char {
        self.current_problem()
            .and_then(|p| p.letter.chars().next())
            .unwrap_or('?')
    }

    pub fn current_template_type(&self) -> &str {
        self.current_problem()
            .map(|p| p.template_type.as_str())
            .unwrap_or("")
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
            "DRY_RUN" => {
                // O jogador prevê a saída; espaço em branco não conta.
                match (&self.selection.predicted_output, &problem.expected_string) {
                    (Some(given), Some(expected)) => {
                        normalize_output(given) == normalize_output(expected)
                    }
                    _ => false,
                }
            }
            _ => false,
        };

        let verdict = if is_correct {
            VerdictCode::Accepted
        } else {
            VerdictCode::WrongAnswer
        };

        self.verdicts.insert(letter, verdict.clone());
        self.last_verdict = Some(verdict.clone());

        if !is_correct {
            self.lives -= 1;
            self.penalty_minutes += 20;
            
            let explanation: &str = if problem.explanation.is_empty() {
                GENERIC_WRONG_ANSWER
            } else {
                &problem.explanation
            };
            let title = crate::app::strip_problem_letter(&problem.title);

            // Guarda o erro antes de limpar a seleção: a resposta dada é o que o
            // relatório mostra em "sua resposta".
            self.record_error(&problem, &verdict, explanation);

            self.trap = Some(TrapInfo {
                name: "Wrong Answer".into(),
                category: "TRAP CLÁSSICA".into(),
                title,
                explanation: explanation.into(),
            });
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

    pub fn submit_tle(&mut self) -> VerdictCode {
        let problem = match self.problems.get(self.current_index) {
            Some(p) => p.clone(),
            None => return VerdictCode::WrongAnswer,
        };

        let letter = problem.letter.chars().next().unwrap_or('?');
        *self.attempts.entry(letter).or_insert(0) += 1;

        let verdict = VerdictCode::TimeLimitExceeded;
        self.verdicts.insert(letter, verdict.clone());
        self.last_verdict = Some(verdict.clone());

        self.lives -= 1;
        self.penalty_minutes += 20;

        // O relógio estourar não muda o que o desafio tinha para ensinar, e quem
        // perdeu a vida no tempo é justamente quem ainda não sabe a resposta. O
        // cartão traz o enquadramento do contest e, em seguida, a explicação do
        // próprio problema — antes ela era descartada, e o estouro de tempo era o
        // único jeito de errar sem aprender nada.
        const CONTEXTO_TLE: &str = "No contest o tempo conta como resposta errada: o problema fica em aberto e a penalidade entra igual.";
        let explanation = if problem.explanation.trim().is_empty() {
            CONTEXTO_TLE.to_string()
        } else {
            format!("{} {}", CONTEXTO_TLE, problem.explanation.trim())
        };

        // Antes de limpar a seleção, para o relatório saber o que estava escolhido.
        self.record_error(&problem, &verdict, &explanation);
        self.selection = MatchSelection::default();

        self.trap = Some(TrapInfo {
            name: "Time Limit Exceeded".into(),
            category: "TIME LIMIT EXCEEDED".into(),
            title: "O relógio da questão zerou".into(),
            explanation,
        });

        if self.lives > 0 {
            self.advance();
        }

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
        // Cada problema traz o seu tempo: passar de um TAG para um DRY_RUN tem de dar
        // mais minuto, não o mesmo.
        self.question_seconds_remaining = self
            .problems
            .get(self.current_index)
            .map_or(60, |p| p.seconds);
    }

    fn all_solved(&self) -> bool {
        self.verdicts.values().all(|v| *v == VerdictCode::Accepted)
    }

    /// Problemas que o jogador respondeu — certo ou errado. Estourar o tempo não conta:
    /// ninguém enviou nada.
    pub fn answered_count(&self) -> i32 {
        self.verdicts
            .values()
            .filter(|v| !matches!(v, VerdictCode::Pending | VerdictCode::TimeLimitExceeded))
            .count() as i32
    }

    pub fn solved_count(&self) -> i32 {
        self.verdicts.values().filter(|v| **v == VerdictCode::Accepted).count() as i32
    }

    /// `origin_sheet` vem do `Model`, não do motor: qual cartão está aberto é estado de
    /// navegação, e a partida não precisa saber dele para julgar nada.
    pub fn to_view_model(&self, origin_sheet: &str) -> MatchViewModel {
        let problem = self.current_problem();

        let balloon_states: Vec<BalloonState> = self.problems.iter().map(|p| {
            let letter = p.letter.chars().next().unwrap_or('?');
            BalloonState {
                letter: p.letter.clone(),
                is_accepted: self.verdicts.get(&letter) == Some(&VerdictCode::Accepted),
                already_paid: p.already_paid,
            }
        }).collect();
        let xp_earned = balloon_states.iter()
            .filter(|b| b.is_accepted && !b.already_paid)
            .count() as i32 * XP_PER_ACCEPTED;

        MatchViewModel {
            is_active: self.is_active,
            current_letter: problem.map(|p| p.letter.clone()).unwrap_or_default(),
            current_title: problem.map(|p| p.title.clone()).unwrap_or_default(),
            current_description: problem.map(|p| p.description.clone()).unwrap_or_default(),
            current_template_type: problem.map(|p| p.template_type.clone()).unwrap_or_default(),
            current_code_lines: problem.map(|p| p.code_lines.clone()).unwrap_or_default(),
            current_options: problem.map(|p| p.options.clone()).unwrap_or_default(),
            max_selections: problem.map(|p| p.max_selections).unwrap_or(1),
            current_origin: problem.map(|p| p.origin.clone()).unwrap_or_default(),
            origin_sheet: origin_sheet.to_string(),
            origin_sheet_paused: !origin_sheet.is_empty() && self.is_paused,
            leave_pending: self.leave_pending,
            solved_so_far: self.solved_count(),
            lives: self.lives,
            max_lives: self.max_lives,
            penalty_minutes: self.penalty_minutes,
            contest_seconds: self.contest_seconds_remaining,
            question_seconds: self.question_seconds_remaining,
            is_frozen: self.is_frozen,
            total_problems: self.problems.len() as i32,
            solved_count: self.solved_count(),
            balloon_states,
            xp_earned,
            selected_line: self.selection.selected_line.unwrap_or(-1),
            answer_string: self.selection.answer_string.clone().unwrap_or_default(),
            drop_time: self.selection.drop_time.clone().unwrap_or_default(),
            drop_space: self.selection.drop_space.clone().unwrap_or_default(),
            selected_tags: self.selection.selected_tags.clone(),
            predicted_output: self.selection.predicted_output.clone().unwrap_or_default(),
            watch_variables: problem.map(|p| p.watch_variables.clone()).unwrap_or_default(),
            watch_note: problem.map(|p| p.watch_note.clone()).unwrap_or_default(),
            last_verdict: self.last_verdict.as_ref().map(|v| v.code().to_string()).unwrap_or_default(),
            errors: self.errors.clone(),
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
                explanation: String::new(),
                options: vec![],
                correct_options: vec![],
                max_selections: 0,
                origin: String::new(),
                seconds: 60,
                watch_variables: vec![],
                watch_note: String::new(),
                already_paid: false,
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
                explanation: String::new(),
                options: vec![],
                correct_options: vec![],
                max_selections: 0,
                origin: String::new(),
                seconds: 60,
                watch_variables: vec![],
                watch_note: String::new(),
                already_paid: false,
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
                explanation: String::new(),
                options: vec!["Grafos".into(), "BFS".into(), "DP".into(), "Greedy".into()],
                correct_options: vec!["Grafos".into(), "BFS".into()],
                max_selections: 2,
                origin: String::new(),
                seconds: 60,
                watch_variables: vec![],
                watch_note: String::new(),
                already_paid: false,
            },
            MatchProblem {
                letter: "D".into(),
                challenge_id: "ch4".into(),
                template_type: "DRY_RUN".into(),
                title: "D · Somando o Contador".into(),
                description: "Qual o valor final de acc?".into(),
                code_lines: vec!["var acc = 0".into()],
                correct_line: None,
                expected_string: Some("6".into()),
                explanation: String::new(),
                options: vec![],
                correct_options: vec![],
                max_selections: 0,
                origin: String::new(),
                seconds: 60,
                watch_variables: vec![crate::domain::WatchVariable {
                    name: "acc".into(),
                    value: "0".into(),
                }],
                watch_note: "antes da linha 2".into(),
                already_paid: false,
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

    /// Avança até o problema D, que é o DRY_RUN.
    fn state_at_dry_run() -> MatchState {
        let mut state = MatchState::new(sample_problems());
        state.selection.selected_line = Some(1);
        state.submit(); // A
        state.selection.answer_string = Some("a".into());
        state.submit(); // B
        state.selection.selected_tags = vec!["Grafos".into(), "BFS".into()];
        state.submit(); // C
        assert_eq!(state.current_template_type(), "DRY_RUN");
        state
    }

    #[test]
    fn test_dry_run_accepts_predicted_output() {
        let mut state = state_at_dry_run();
        state.selection.predicted_output = Some("6".into());
        assert_eq!(state.submit(), VerdictCode::Accepted);
    }

    #[test]
    fn test_dry_run_ignores_surrounding_whitespace() {
        let mut state = state_at_dry_run();
        state.selection.predicted_output = Some("  6 \n".into());
        assert_eq!(state.submit(), VerdictCode::Accepted);
    }

    #[test]
    fn test_dry_run_rejects_wrong_value() {
        let mut state = state_at_dry_run();
        state.selection.predicted_output = Some("5".into());
        assert_eq!(state.submit(), VerdictCode::WrongAnswer);
    }

    #[test]
    fn test_dry_run_without_answer_is_wrong() {
        let mut state = state_at_dry_run();
        assert_eq!(state.submit(), VerdictCode::WrongAnswer);
    }

    #[test]
    fn test_view_model_exposes_last_verdict() {
        let mut state = MatchState::new(sample_problems());
        assert_eq!(state.to_view_model("").last_verdict, "");

        state.selection.selected_line = Some(0); // errada
        state.submit();
        assert_eq!(state.to_view_model("").last_verdict, "WA");

        state.selection.answer_string = Some("a".into()); // certa
        state.submit();
        assert_eq!(state.to_view_model("").last_verdict, "AC");
    }

    #[test]
    fn test_view_model_exposes_watch_panel_only_for_dry_run() {
        let state = MatchState::new(sample_problems());
        // Problema A é SPOT_THE_BUG — sem painel de watch.
        assert!(state.to_view_model("").watch_variables.is_empty());

        let dry = state_at_dry_run();
        let vm = dry.to_view_model("");
        assert_eq!(vm.watch_variables.len(), 1);
        assert_eq!(vm.watch_variables[0].name, "acc");
        assert_eq!(vm.watch_note, "antes da linha 2");
    }

    #[test]
    fn test_errors_are_recorded_with_the_given_answer() {
        let mut state = MatchState::new(sample_problems());

        state.selection.selected_line = Some(0); // errada, a certa é 1
        state.submit();

        assert_eq!(state.errors.len(), 1);
        let e = &state.errors[0];
        assert_eq!(e.letter, "A");
        assert_eq!(e.verdict, "WA");
        assert_eq!(e.given_answer, "linha 1", "a linha é exibida 1-based");
        assert!(!e.explanation.is_empty());
    }

    #[test]
    fn test_correct_answers_leave_no_error() {
        let mut state = MatchState::new(sample_problems());
        state.selection.selected_line = Some(1); // certa
        state.submit();
        assert!(state.errors.is_empty());
    }

    #[test]
    fn test_errors_accumulate_across_problems() {
        let mut state = MatchState::new(sample_problems());
        state.selection.selected_line = Some(0);
        state.submit(); // A errado
        state.selection.answer_string = Some("errado".into());
        state.submit(); // B errado

        assert_eq!(state.errors.len(), 2);
        assert_eq!(state.errors[1].letter, "B");
        assert_eq!(state.errors[1].given_answer, "errado");
        assert_eq!(state.to_view_model("").errors.len(), 2);
    }

    #[test]
    fn test_timeout_records_the_error_too() {
        let mut state = MatchState::new(sample_problems());
        state.selection.selected_line = Some(0);
        state.submit_tle();

        assert_eq!(state.errors.len(), 1);
        assert_eq!(state.errors[0].verdict, "TLE");
        assert_eq!(state.errors[0].given_answer, "linha 1");
    }

    #[test]
    fn test_timeout_still_teaches_the_challenge_explanation() {
        // O estouro de tempo trocava a explicação do desafio por um texto genérico
        // sobre o relógio, então errar por tempo era o único jeito de errar sem
        // aprender nada. O cartão traz os dois, nessa ordem.
        let mut problems = sample_problems();
        problems[0].explanation =
            "Explicação do desafio de teste.".into();

        let mut state = MatchState::new(problems);
        state.submit_tle();

        let trap = state.trap.as_ref().expect("o TLE tem de montar o cartão");
        assert!(
            trap.explanation.contains("penalidade entra igual"),
            "o enquadramento do contest continua no cartão: {}",
            trap.explanation
        );
        assert!(
            trap.explanation.contains("Explicação do desafio de teste"),
            "a explicação do desafio tem de sobreviver ao estouro: {}",
            trap.explanation
        );
        assert_eq!(
            state.errors[0].explanation, trap.explanation,
            "o relatório pós-partida mostra o mesmo texto do cartão"
        );
    }

    #[test]
    fn test_timeout_without_explanation_keeps_only_the_generic_text() {
        // `sample_problems` nasce com a explicação vazia, que é o caso do desafio
        // que não trouxe uma: aí o cartão fica só com o enquadramento, sem sobra.
        let mut state = MatchState::new(sample_problems());
        state.submit_tle();

        let trap = state.trap.as_ref().unwrap();
        assert!(trap.explanation.contains("penalidade entra igual"));
        assert!(
            !trap.explanation.ends_with(' '),
            "sem espaço pendurado quando não há o que emendar"
        );
    }

    #[test]
    fn test_scoreboard_freezes_in_the_last_stretch() {
        let mut state = MatchState::new(sample_problems());
        assert!(!state.is_frozen, "placar não começa congelado");

        state.contest_seconds_remaining = MatchState::FREEZE_SECONDS + 1;
        state.refresh_freeze();
        assert!(!state.is_frozen, "um segundo antes do limiar ainda está aberto");

        state.contest_seconds_remaining = MatchState::FREEZE_SECONDS;
        state.refresh_freeze();
        assert!(state.is_frozen, "no limiar o placar congela");
        assert!(state.to_view_model("").is_frozen);
    }

    #[test]
    fn test_finished_match_is_not_frozen() {
        let mut state = MatchState::new(sample_problems());
        state.contest_seconds_remaining = 0;
        state.is_active = false;
        state.refresh_freeze();
        // Contest encerrado não é contest congelado: o resultado já é conhecido.
        assert!(!state.is_frozen);
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
