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
	cd backend && go run .

# Roda todos os testes do Backend
test-backend:
	cd backend && go test -v ./...

# Aplica as migrações no banco de Produção e encerra sem subir o servidor HTTP.
# Uso: DATABASE_URL="postgres://admin..." just migrate-prod
migrate-prod:
	cd backend && RUN_MIGRATIONS=true MIGRATE_ONLY=true go run .

# Faz o deploy do Backend para o Google Cloud Run (usando Source-to-Image)
deploy-backend:
	gcloud run deploy logn-backend \
		--source ./backend \
		--region us-east1 \
		--allow-unauthenticated

# --- Core (Rust) ---

# Roda os testes da maquina de estados Crux
test-core:
	cd shared_core && cargo test

# --- Ambientes e Shells ---

# Sincroniza as chaves do .env raiz para o formato consumível das shells nativas
sync-env:
	@echo "// Generated auto-magically from .env by Justfile" > ios/LogNiOS/Local.xcconfig
	@sed -e 's/#.*//g' -e '/^$$/d' -e 's/\/\//\/\$()\//g' .env >> ios/LogNiOS/Local.xcconfig
	@echo "Local.xcconfig synced from .env!"


# Empacota a trilha atual (nós + desafios) dentro do app, para a primeira abertura sem
# rede. Precisa do backend de pé. Rode de novo depois de cada migração de conteúdo.
seed-bundle:
	python3 tools/seed_bundle.py

# Gera as strings de internacionalização (i18n) para Swift (e futuramente Kotlin)
i18n:
	cargo run --manifest-path tools_i18n/Cargo.toml -- --language swift --output-dir ios/LogNiOS/LogNiOS/DesignSystem

# --- iOS (Swift) ---

# Gera a ponte FFI em Swift (Facet + Bincode) e joga na pasta do iOS
codegen:
	cd shared_core && cargo run --bin codegen --features codegen -- --language swift --output-dir ../ios/SharedCore

# Compila as bibliotecas estáticas (Rust) para iOS e empacota no XCFramework
build-ios-ffi: codegen
	cd shared_core && rustup target add aarch64-apple-ios aarch64-apple-ios-sim x86_64-apple-ios
	cd shared_core && cargo build --target aarch64-apple-ios --release
	cd shared_core && cargo build --target aarch64-apple-ios-sim --release
	cd shared_core && cargo build --target x86_64-apple-ios --release
	mkdir -p shared_core/target/universal-sim
	lipo -create -output shared_core/target/universal-sim/libshared_core.a shared_core/target/aarch64-apple-ios-sim/release/libshared_core.a shared_core/target/x86_64-apple-ios/release/libshared_core.a
	cp shared_core/target/aarch64-apple-ios/release/libshared_core.a ios/LogNCoreFFI/LogNCoreFFI.xcframework/ios-arm64/libshared_core.a
	cp shared_core/target/universal-sim/libshared_core.a ios/LogNCoreFFI/LogNCoreFFI.xcframework/ios-arm64_x86_64-simulator/libshared_core.a

# Gera o projeto Xcode (.xcodeproj) usando o XcodeGen
xcode: sync-env build-ios-ffi i18n
	cd ios/LogNiOS && xcodegen generate

# Abre o projeto no Xcode
open-ios: xcode
	open ios/LogNiOS/LogNiOS.xcodeproj

# Limpa o build do Rust e do Xcode
clean:
	cd shared_core && cargo clean
	rm -rf ios/LogNiOS/LogNiOS.xcodeproj
	rm -rf ios/LogNiOS/.build
