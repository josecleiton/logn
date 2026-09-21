# LogN Project Justfile

set shell := ["bash", "-c"]

default:
	@just --list

# --- Backend (Golang) ---

# Sobe o banco de dados via Docker Compose
db-up:
	docker-compose up -d db

# Derruba o banco de dados
db-down:
	docker-compose down

# Roda o servidor Go localmente na porta 8080 (Lembre-se de ter o .env)
run-backend: db-up
	cd backend && go run main.go

# Roda todos os testes do Backend
test-backend:
	cd backend && go test -v ./...

# --- Core (Rust) ---

# Roda os testes da maquina de estados Crux
test-core:
	cd shared_core && cargo test

# --- iOS (Swift) ---

# Gera a ponte FFI em Swift (Facet + Bincode) e joga na pasta do iOS
codegen:
	cd shared_core && cargo run --bin codegen --features codegen -- --language swift --output-dir ../ios/SharedCore

# Gera o projeto Xcode (.xcodeproj) usando o XcodeGen
xcode: codegen
	cd ios/LogNiOS && xcodegen generate

# Abre o projeto no Xcode
open-ios: xcode
	open ios/LogNiOS/LogNiOS.xcodeproj

# Limpa o build do Rust e do Xcode
clean:
	cd shared_core && cargo clean
	rm -rf ios/LogNiOS/LogNiOS.xcodeproj
	rm -rf ios/LogNiOS/.build
