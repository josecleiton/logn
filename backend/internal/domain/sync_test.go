package domain

import (
	"testing"
)

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
}
