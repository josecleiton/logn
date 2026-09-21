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
}


#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengeValidation {
    #[serde(rename = "type")]
    pub validation_type: String, // "LINE_MATCH" or "EXACT_MATCH"
    pub correct_line: Option<i32>,
    pub expected_string: Option<String>,
}

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct ChallengeContent {
    pub title: String,
    pub description: String,
    pub code_lines: Vec<String>,
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

#[derive(Facet, Serialize, Deserialize, Clone, Debug)]
#[facet(fg::namespace = "LogN")]
pub struct SkillNode {
    pub id: String,
    pub name: String,
    pub description: String,
    pub order_index: i32,
    pub required_xp: i32,
}
