package domain

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
)

type ArgonConfig struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
}

// Parâmetros da recomendação mínima da OWASP para Argon2id: 19 MiB, duas passadas,
// uma thread.
//
// Eram 64 MiB por hash. Sem limite de concorrência, cinquenta logins simultâneos
// pediam mais de 3 GB e derrubavam a instância. Hash antigo continua verificando, porque
// os parâmetros viajam dentro dele, e é refeito com estes no próximo login que acertar
// (NeedsRehash).
var argonCfg = &ArgonConfig{
	time:    2,
	memory:  19 * 1024,
	threads: 1,
	keyLen:  32,
}

// argonSlots limita a memória dos hashes que rodam ao mesmo tempo. Cada vaga vale
// `argonCfg.memory`; o resto espera na fila, e o pico fica em argonSlotCount × 19 MiB,
// qualquer que seja o tráfego.
const argonSlotCount = 4

var argonSlots = make(chan struct{}, argonSlotCount)

// argonWide deixa um só hash de várias vagas juntando as dele por vez. Dois hashes
// antigos pegando vagas aos poucos podiam ficar com metade cada um e travar os dois.
var argonWide = make(chan struct{}, 1)

// argonCompareWait é quanto a comparação espera por vaga. O login não consumiu nada
// ainda, então desistir e mandar tentar de novo é barato; fila sem fim só empilha
// pedido que o cliente já abandonou.
const argonCompareWait = 10 * time.Second

// ErrArgonBusy diz que o hash não rodou porque não houve vaga a tempo.
var ErrArgonBusy = errors.New("argon2: no slot available")

// acquireArgon reserva as vagas de um hash de `memory` KiB e devolve como soltá-las.
//
// Hash antigo, de 64 MiB, ocupava uma vaga só, como um de 19 MiB: quatro deles juntos
// passavam dos 256 MiB da instância. Agora cada um pega as vagas da memória que usa.
func acquireArgon(ctx context.Context, memory uint32) (func(), error) {
	need := int((memory + argonCfg.memory - 1) / argonCfg.memory)
	need = max(1, min(need, argonSlotCount))

	held := 0
	release := func() {
		for ; held > 0; held-- {
			<-argonSlots
		}
	}
	if need > 1 {
		select {
		case argonWide <- struct{}{}:
			defer func() { <-argonWide }()
		case <-ctx.Done():
			return nil, ErrArgonBusy
		}
	}
	for held < need {
		select {
		case argonSlots <- struct{}{}:
			held++
		case <-ctx.Done():
			release()
			return nil, ErrArgonBusy
		}
	}
	return release, nil
}

// HoldArgonSlots toma todas as vagas do Argon2 até `release`. É para os testes que
// precisam do servidor ocupado; o código de produção não chama.
func HoldArgonSlots(ctx context.Context) (release func(), err error) {
	return acquireArgon(ctx, argonCfg.memory*argonSlotCount)
}

func argonIDKey(ctx context.Context, password, salt []byte, timeCost, memory uint32, threads uint8, keyLen uint32) ([]byte, error) {
	release, err := acquireArgon(ctx, memory)
	if err != nil {
		return nil, err
	}
	defer release()
	return argon2.IDKey(password, salt, timeCost, memory, threads, keyLen), nil
}

// HashPassword hashes a password using Argon2id. Espera a vaga enquanto `ctx` valer, sem
// prazo próprio: no cadastro e na troca de senha o código já foi consumido, e desistir
// por fila obrigaria a pedir outro.
func HashPassword(ctx context.Context, password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash, err := argonIDKey(ctx, []byte(password), salt, argonCfg.time, argonCfg.memory, argonCfg.threads, argonCfg.keyLen)
	if err != nil {
		return "", err
	}

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonCfg.memory, argonCfg.time, argonCfg.threads, b64Salt, b64Hash)
	return encodedHash, nil
}

// ComparePasswordAndHash compares a password with an Argon2id hash. Sem vaga em
// argonCompareWait, devolve ErrArgonBusy.
func ComparePasswordAndHash(ctx context.Context, password, encodedHash string) (bool, error) {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return false, ErrInvalidHash
	}

	var version int
	_, err := fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil {
		return false, err
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(vals[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.Strict().DecodeString(vals[5])
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(ctx, argonCompareWait)
	defer cancel()
	hashToCompare, err := argonIDKey(ctx, []byte(password), salt, timeCost, memory, threads, uint32(len(decodedHash)))
	if err != nil {
		return false, err
	}

	if subtle.ConstantTimeCompare(decodedHash, hashToCompare) == 1 {
		return true, nil
	}
	return false, nil
}

// NeedsRehash diz se o hash foi feito com parâmetros diferentes dos atuais.
func NeedsRehash(encodedHash string) bool {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return true
	}
	var memory, timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return true
	}
	return memory != argonCfg.memory || timeCost != argonCfg.time || threads != argonCfg.threads
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// DummyHash é o hash que o login compara quando não há senha de verdade para comparar:
// e-mail inexistente ou conta sem senha. Assim a resposta leva o mesmo tempo nos dois
// casos e não entrega quem tem conta.
//
// Era uma string fixa no handler, e ia ficar para trás na primeira troca de
// parâmetros. Gerado aqui, acompanha argonCfg sozinho.
func DummyHash() string {
	dummyHashOnce.Do(func() {
		secret := make([]byte, 32)
		_, _ = rand.Read(secret)
		dummyHash, _ = HashPassword(context.Background(), base64.RawStdEncoding.EncodeToString(secret))
	})
	return dummyHash
}

// JWT Generation
var JwtSecretKey []byte

func GenerateAccessToken(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	})
	return token.SignedString(JwtSecretKey)
}

// ErrUnauthenticated cobre token ausente, malformado, expirado ou com assinatura
// que não confere. O motivo exato não volta para o cliente de propósito.
var ErrUnauthenticated = errors.New("unauthenticated")

// ErrRefreshTokenAlreadyUsed sinaliza refresh token apresentado duas vezes. Com
// rotação isso é sinal de cópia: o token de quem realmente está logado já foi trocado.
var ErrRefreshTokenAlreadyUsed = errors.New("refresh token already used")

// UserIDFromAccessToken valida o JWT e devolve de quem ele é.
//
// O `/sync` confiava no `user_id` que vinha no corpo do pedido: qualquer um podia
// escrever na cadeia de outra pessoa. Quem é o dono da sessão só pode sair daqui.
func UserIDFromAccessToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnauthenticated
		}
		return JwtSecretKey, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return "", ErrUnauthenticated
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", ErrUnauthenticated
	}
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return "", ErrUnauthenticated
	}
	return userID, nil
}

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
