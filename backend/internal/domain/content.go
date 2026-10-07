package domain

import (
	"encoding/json"
	"fmt"
)

// ChallengeText é o que muda com a língua num desafio: o texto que a pessoa lê e o
// rótulo de cada opção neutra.
type ChallengeText struct {
	Title       string
	Description string
	Explanation string
	WatchNote   string
	// OptionLabels troca uma opção neutra (`queue`) pelo rótulo na língua (`Fila`).
	// Opção sem rótulo aqui fica como está: é o caso de código e de complexidade.
	OptionLabels map[string]string
}

// AssemblePayload devolve o payload na forma que o app sempre leu: a estrutura neutra
// do banco com o texto da língua pedida de volta no lugar.
//
// A forma da resposta não muda de propósito. App já instalado segue lendo, e o motor
// de partida compara resposta com gabarito dentro do mesmo payload — trocar a opção e
// o gabarito pelo mesmo rótulo mantém a comparação certa em qualquer língua.
func AssemblePayload(neutral json.RawMessage, text ChallengeText) (json.RawMessage, error) {
	var payload map[string]map[string]json.RawMessage
	if err := json.Unmarshal(neutral, &payload); err != nil {
		return nil, fmt.Errorf("unreadable payload: %w", err)
	}
	content, validation := payload["content"], payload["validation"]
	if content == nil || validation == nil {
		return nil, fmt.Errorf("payload without content or validation")
	}

	set := func(m map[string]json.RawMessage, key, value string) {
		b, _ := json.Marshal(value)
		m[key] = b
	}
	set(content, "title", text.Title)
	set(content, "description", text.Description)
	set(validation, "explanation", text.Explanation)
	if text.WatchNote != "" {
		set(content, "watch_note", text.WatchNote)
	}

	if len(text.OptionLabels) > 0 {
		for _, key := range []string{"options", "correct_options"} {
			raw, ok := content[key]
			if !ok {
				continue
			}
			var options []string
			if err := json.Unmarshal(raw, &options); err != nil {
				return nil, fmt.Errorf("%s is not a list of strings: %w", key, err)
			}
			for i, o := range options {
				if label, ok := text.OptionLabels[o]; ok {
					options[i] = label
				}
			}
			b, _ := json.Marshal(options)
			content[key] = b
		}
	}

	return json.Marshal(payload)
}
