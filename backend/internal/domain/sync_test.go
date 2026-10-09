package domain

import (
	"errors"
	"strings"
	"testing"
)

// Sync sem teto enchia o banco e derrubava a instância com uma conta grátis.
func TestCheckSyncShapeRecusaOQuePassaDoTeto(t *testing.T) {
	ok := GameEvent{ID: "match_1", EventType: "MATCH_ANSWER", PayloadJSON: `{"is_correct":true}`}

	if err := CheckSyncShape(SyncPayload{Events: []GameEvent{ok, {ID: "match_1_end", EventType: "MATCH_END", PayloadJSON: `{"solved":1}`}}}); err != nil {
		t.Fatalf("sync do Core devia passar: %v", err)
	}

	many := make([]GameEvent, MaxSyncEvents+1)
	for i := range many {
		many[i] = ok
	}
	if err := CheckSyncShape(SyncPayload{Events: many}); !errors.Is(err, ErrSyncTooLarge) {
		t.Fatalf("acima de MaxSyncEvents: esperava ErrSyncTooLarge, veio %v", err)
	}
	if err := CheckSyncShape(SyncPayload{Events: many[:MaxSyncEvents]}); err != nil {
		t.Fatalf("no teto devia passar: %v", err)
	}

	for name, e := range map[string]GameEvent{
		"tipo que o Core não escreve": {ID: "x", EventType: "GRANT_XP", PayloadJSON: "{}"},
		"payload acima do teto":       {ID: "x", EventType: "MATCH_ANSWER", PayloadJSON: strings.Repeat("a", MaxEventPayloadSize+1)},
		"id vazio":                    {ID: "", EventType: "MATCH_ANSWER", PayloadJSON: "{}"},
		"id acima do teto":            {ID: strings.Repeat("i", MaxEventIDLength+1), EventType: "MATCH_ANSWER", PayloadJSON: "{}"},
	} {
		if err := CheckSyncShape(SyncPayload{Events: []GameEvent{e}}); err == nil {
			t.Errorf("%s: devia ser recusado", name)
		}
	}
}

func TestValidateSync(t *testing.T) {
	// 1. Setup a valid event manually hashed identically to Rust
	// Payload: prev=00...0, id=ch1, type=ANSWER, json={}

	event1 := GameEvent{
		ID:           "ch1",
		EventType:    "ANSWER",
		PayloadJSON:  "{}",
		Timestamp:    1690000000,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
	}

	event1.CurrentHash = ComputeHash(event1, event1.PreviousHash)

	event2 := GameEvent{
		ID:           "ch2",
		EventType:    "SKIP",
		PayloadJSON:  "{}",
		Timestamp:    1690000005,
		PreviousHash: event1.CurrentHash,
	}
	event2.CurrentHash = ComputeHash(event2, event2.PreviousHash)

	payload := SyncPayload{
		UserID: "user_x",
		Events: []GameEvent{event1, event2},
	}

	valid, err := ValidateSync(payload, "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("Expected valid sync, got error: %v", err)
	}
	if !valid {
		t.Fatalf("Expected valid sync")
	}

	// 2. Test Invalid Hash (Tampering)
	payload.Events[1].PayloadJSON = `{"tampered": true}`
	valid, err = ValidateSync(payload, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatalf("Expected error on tampered payload")
	}

	// 3. Test Rebase Logic (Previous Hash mismatch on first event)
	valid, err = ValidateSync(payload, "1111111111111111111111111111111111111111111111111111111111111111")
	if err == nil || err.Error() != "force_rebase" {
		t.Fatalf("Expected force_rebase error, got: %v", err)
	}

	// 4. previous_hash forjado depois do primeiro: o current_hash está certo, porque foi
	// calculado sobre o anterior de verdade, mas a coluna gravaria outro valor. Não
	// passa, e pede rebase em vez de recusar a fila.
	payload.Events[1].PayloadJSON = "{}"
	payload.Events[1].PreviousHash = "2222222222222222222222222222222222222222222222222222222222222222"
	valid, err = ValidateSync(payload, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil || valid || err.Error() != "force_rebase" {
		t.Fatalf("previous_hash forjado no segundo evento devia pedir rebase, veio valid=%v err=%v", valid, err)
	}
}
