package domain

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type Challenge struct {
	ID           string          `json:"id" db:"id"`
	NodeID       string          `json:"node_id" db:"node_id"`
	TemplateType string          `json:"template_type" db:"template_type"`
	Version      int             `json:"version" db:"version"`
	Payload      json.RawMessage `json:"payload" db:"payload"`
	// Posição dentro do nó, a partir de 1. É ela que vira a letra A, B, C da partida.
	PositionIdx int `json:"position_idx" db:"position_idx"`
	// De onde o desafio veio, quando não foi escrito para o LogN. Vazio é o caso
	// comum; o cliente mostra um selo quando há algo aqui.
	Origin string `json:"origin" db:"origin"`
}

type GameEvent struct {
	ID           string `json:"id"`
	EventType    string `json:"event_type"`
	PayloadJSON  string `json:"payload_json"`
	Timestamp    int64  `json:"timestamp"`
	PreviousHash string `json:"previous_hash"`
	CurrentHash  string `json:"current_hash"`
}

type SyncPayload struct {
	UserID string      `json:"user_id"`
	Events []GameEvent `json:"events"`
}

// ComputeHash replica a lógica do Mini-Git Client-Side para validação e rebase
func ComputeHash(event GameEvent, previousHash string) string {
	payloadForHash := fmt.Sprintf("%s%s%s%s", previousHash, event.ID, event.EventType, event.PayloadJSON)

	hasher := sha256.New()
	hasher.Write([]byte(payloadForHash))

	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, uint64(event.Timestamp))
	hasher.Write(timestampBytes)

	return hex.EncodeToString(hasher.Sum(nil))
}

// ValidateSync verifica se a cadeia de hashes do payload é criptograficamente sólida
func ValidateSync(payload SyncPayload, serverLastHash string) (bool, error) {
	if len(payload.Events) == 0 {
		return true, nil
	}

	currentHash := serverLastHash
	if currentHash == "" {
		currentHash = "0000000000000000000000000000000000000000000000000000000000000000"
	}

	for i, event := range payload.Events {
		// Se o previous não bater, sinaliza necessidade de Rebase
		if event.PreviousHash != currentHash && i == 0 {
			return false, errors.New("force_rebase")
		}

		calculated := ComputeHash(event, currentHash)
		if calculated != event.CurrentHash {
			return false, fmt.Errorf("hash mismatch on event %s", event.ID)
		}

		currentHash = calculated
	}

	return true, nil
}
