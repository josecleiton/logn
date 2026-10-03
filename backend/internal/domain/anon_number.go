package domain

import (
	"crypto/rand"
	"math/big"
)

// O número de "jogador #N" no placar (docs/specs/logn_placar_spec.md, seção 4.2).
//
// Sorteado com crypto/rand, nunca derivado do id nem da ordem do cadastro: um número
// sequencial diria quando a conta nasceu, e um tirado do id deixaria cruzar com os
// lugares onde o id aparece.
const (
	// Dígitos da faixa inicial: 1000 a 9999.
	anonFirstDigits = 4
	// Colisões seguidas numa faixa antes de subir um dígito.
	anonDrawsPerWidth = 3
	// Teto de sorteios num cadastro. Com a faixa crescendo, chegar aqui é defeito.
	anonMaxDraws = 12
)

// anonDraw é trocável nos testes, para forçar colisão.
var anonDraw = drawAnonNumber

// drawAnonNumber sorteia um número com exatamente `digits` dígitos.
func drawAnonNumber(digits int) (int, error) {
	low := pow10(digits - 1)
	n, err := rand.Int(rand.Reader, big.NewInt(int64(pow10(digits)-low)))
	if err != nil {
		return 0, err
	}
	return low + int(n.Int64()), nil
}

func pow10(n int) int {
	v := 1
	for i := 0; i < n; i++ {
		v *= 10
	}
	return v
}
