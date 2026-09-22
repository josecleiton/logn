use facet::Facet;
use facet_generate_attrs as fg;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(C)]
pub enum TemplateType {
    FillInTheBlank,
    SpotTheBug,
    ComplexityMatch,
    TagThePattern,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct GameEvent {
    pub id: String,
    pub event_type: String,
    pub payload_json: String,
    pub timestamp: i64,
    pub previous_hash: String,
    pub current_hash: String,
}

impl GameEvent {
    /// O Mini-Git: Calcula Hash(N) = SHA-256(Hash(N-1) + Payload + Timestamp)
    pub fn new(id: String, event_type: String, payload_json: String, timestamp: i64, previous_hash: String) -> Self {
        let mut hasher = Sha256::new();
        let payload_for_hash = format!("{}{}{}{}", previous_hash, id, event_type, payload_json);
        hasher.update(payload_for_hash.as_bytes());
        hasher.update(&timestamp.to_be_bytes());
        let current_hash = hasher.finalize().iter().map(|b| format!("{:02x}", b)).collect::<String>();

        Self {
            id,
            event_type,
            payload_json,
            timestamp,
            previous_hash,
            current_hash,
        }
    }
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct SyncPayload {
    pub user_id: String,
    pub events: Vec<GameEvent>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mini_git_hash() {
        let event = GameEvent::new(
            "ch1".to_string(),
            "ANSWER".to_string(),
            "{}".to_string(),
            1690000000,
            "0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        );

        // O hash gerado deve ser consistente e não vazio
        assert!(!event.current_hash.is_empty());
        assert_eq!(event.current_hash.len(), 64); // SHA-256 len
    }

    /// O contrato do `GET /api/v1/nodes` como o Go realmente responde.
    ///
    /// Sem `status` no JSON a lista inteira falhava no serde e o `if let Ok(..)` engolia
    /// o erro: quem entrava autenticado via a árvore vazia e nenhuma mensagem.
    #[test]
    fn test_parses_nodes_without_status_from_the_backend() {
        let body = r#"[
            {"id":"10000000-0000-0000-0000-000000000001","name":"Nó A",
             "description":"Simulação de algoritmos.","row":0,"column":0,
             "required_xp":0,"prerequisites":[]},
            {"id":"20000000-0000-0000-0000-000000000002","name":"Nó B",
             "description":"Stacks e Queues.","row":1,"column":-1,
             "required_xp":100,"prerequisites":["10000000-0000-0000-0000-000000000001"]}
        ]"#;

        let nodes: Vec<SkillNode> =
            serde_json::from_slice(body.as_bytes()).expect("o payload do Go tem de desserializar");

        assert_eq!(nodes.len(), 2);
        assert_eq!(nodes[1].prerequisites, vec![nodes[0].id.clone()]);
        // `status` é derivado em view(); o default só precisa existir para o parse passar.
        assert_eq!(nodes[0].status, NodeStatus::Locked);
    }
}


#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengeValidation {
    #[serde(rename = "type")]
    pub validation_type: String, // "LINE_MATCH", "EXACT_MATCH" or "OUTPUT_MATCH" (DRY_RUN)
    pub correct_line: Option<i32>,
    pub expected_string: Option<String>,
}

/// Uma variável do painel de watch do DRY_RUN — nome e valor no estado inicial.
/// Não entrega a resposta: diz *o que* acompanhar durante o trace.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
pub struct WatchVariable {
    pub name: String,
    pub value: String,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengeContent {
    pub title: String,
    pub description: String,
    pub code_lines: Vec<String>,
    pub options: Option<Vec<String>>,
    pub correct_options: Option<Vec<String>>,
    /// Só em DRY_RUN: estado inicial mostrado no painel de watch.
    #[serde(default)]
    pub watch_variables: Option<Vec<WatchVariable>>,
    /// Só em DRY_RUN: em que ponto o watch foi capturado, ex. "antes da linha 4".
    #[serde(default)]
    pub watch_note: Option<String>,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengePayload {
    pub content: ChallengeContent,
    pub validation: ChallengeValidation,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct Challenge {
    pub id: String,
    pub node_id: String,
    pub template_type: String,
    pub version: i32,
    pub payload: ChallengePayload, // Typed for Facet
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq, Default)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum NodeStatus {
    #[default]
    Locked,
    Active,
    Completed,
}

/// Estado de uma célula do telão. Os quatro estados do DS, e só eles.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(u8)]
pub enum ScoreCellState {
    /// Aceito — `+` ou `+N` em cima, minuto do AC embaixo.
    Accepted,
    /// Tentado sem AC — `−N` em cima.
    Failed,
    /// Submetido após o congelamento — `?` em cima, `frz` embaixo.
    Frozen,
    /// Não tentado — célula vazia.
    Untried,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ScoreCell {
    pub state: ScoreCellState,
    pub top: String,
    pub bottom: String,
}

/// Uma linha do telão. 13 células, uma por letra A—M.
#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ScoreboardRow {
    pub rank: i32,
    pub team: String,
    pub university: String,
    pub solved: i32,
    pub penalty: i32,
    pub is_user: bool,
    pub cells: Vec<ScoreCell>,
}

/// Uma linha do ranking de celular.
#[derive(Facet, Serialize, Deserialize, Clone, Debug, Default)]
#[facet(fg::namespace = "LogN")]
pub struct StandingRow {
    pub rank: i32,
    pub handle: String,
    pub university: String,
    pub solved: i32,
    pub penalty: i32,
    pub is_user: bool,
    /// Só na linha do usuário: `subiu 6 nesta rodada`. Vazio nas demais.
    pub note: String,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct SkillNode {
    pub id: String,
    pub name: String,
    pub description: String,
    pub row: i32,
    pub column: i32,
    pub required_xp: i32,
    pub prerequisites: Vec<String>,
    /// Derivado em `view()` a partir do XP e do DAG, nunca persistido: o Go não manda
    /// este campo, e sem o `default` a lista inteira falhava no `serde` — o usuário
    /// autenticado via uma árvore vazia sem erro nenhum aparecer.
    #[serde(default)]
    pub status: NodeStatus,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
#[repr(C)]
pub enum TelemetryOperation {
    Identify { user_id: String },
    Track { event: String, properties: std::collections::HashMap<String, String> },

}

impl crux_core::capability::Operation for TelemetryOperation {
    type Output = ();
}


#[derive(Facet, Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
#[facet(fg::namespace = "LogN")]
#[repr(C)]
pub enum MonitoringOperation {
    LogError { message: String, details: String },
    StartSpan { name: String },
    EndSpan { name: String },
}

impl crux_core::capability::Operation for MonitoringOperation {
    type Output = ();
}
