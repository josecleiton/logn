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
#
# Recusa publicar documento legal com marcador de rascunho ([A CONFIRMAR] e afins). Para
# levar um rascunho de propósito, durante a revisão do advogado:
# LEGAL_ALLOW_DRAFT=true DATABASE_URL=... just migrate-prod
migrate-prod:
	#!/usr/bin/env bash
	set -euo pipefail
	if [ "${LEGAL_ALLOW_DRAFT:-}" != "true" ] && grep -lE 'A CONFIRMAR|TO CONFIRM|POR CONFIRMAR' backend/schema/migrations/*.sql; then
		echo "Migração acima tem documento legal em rascunho. Publique o texto final ou use LEGAL_ALLOW_DRAFT=true." >&2
		exit 1
	fi
	cd backend && RUN_MIGRATIONS=true MIGRATE_ONLY=true go run .

# Confere os documentos legais do repositório de conteúdo: estrutura, links, línguas e
# ids de seção. Com `release`, recusa também os marcadores de rascunho.
# Uso: just legal-check        ou        just legal-check release
legal-check mode="":
	cd backend && go run ./cmd/legalcheck {{ if mode == "release" { "--release" } else { "" } }} ../../logn-conteudo/legal

# Faz o deploy do Backend para o Google Cloud Run, a partir do Dockerfile de backend/.
# Variáveis, secrets e probes vivem no serviço `logn` e são mantidas a cada deploy.
# As migrações não sobem junto: rode `just migrate-prod` antes, da sua máquina.
deploy-backend:
	gcloud run deploy logn \
		--source ./backend \
		--region us-east1 \

# Cria ou atualiza o job no Cloud Scheduler para expurgar contas deletadas.
# Cria ou atualiza o job no Cloud Scheduler para expurgar contas deletadas.
# Extrai automaticamente a URL e a Service Account do serviço Cloud Run.
deploy-scheduler:
	#!/usr/bin/env bash
	set -euo pipefail
	echo "Consultando informações do serviço 'logn' no Cloud Run..."
	URL=$(gcloud run services describe logn --region us-east1 --format 'value(status.url)')
	SA=$(gcloud run services describe logn --region us-east1 --format 'value(spec.template.spec.serviceAccountName)')
	
	if [ -z "$SA" ] || [ "$SA" = "None" ]; then
		PROJECT_NUMBER=$(gcloud projects describe $(gcloud config get-value project) --format 'value(projectNumber)')
		SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
	fi
	
	echo "Configurando Cloud Scheduler para bater em $URL usando a conta $SA"
	
	gcloud scheduler jobs create http purge-deleted-accounts \
		--schedule="0 3 * * *" \
		--uri="$URL/api/v1/internal/purge" \
		--http-method=POST \
		--oidc-service-account-email="$SA" \
		--oidc-token-audience="$URL" \
		--location=us-east1 \
		|| gcloud scheduler jobs update http purge-deleted-accounts \
		--schedule="0 3 * * *" \
		--uri="$URL/api/v1/internal/purge" \
		--http-method=POST \
		--oidc-service-account-email="$SA" \
		--oidc-token-audience="$URL" \
		--location=us-east1


# --- Core (Rust) ---

# Roda os testes da maquina de estados Crux
test-core:
	cd shared_core && cargo test

# --- Ambientes e Shells ---

# Sincroniza as chaves do .env raiz para o formato consumível das shells nativas
sync-env:
	@echo "// Generated auto-magically from .env by Justfile" > ios/LogNiOS/Local.xcconfig
	@sed -e 's/#.*//g' -e '/^$/d' -e 's/\/\//\/\$()\//g' .env >> ios/LogNiOS/Local.xcconfig
	@echo "Local.xcconfig synced from .env!"


# Empacota a trilha atual (nós + desafios) dentro do app, para a primeira abertura sem
# rede. Precisa do backend de pé. Rode de novo depois de cada migração de conteúdo.
seed-bundle:
	python3 tools/seed_bundle.py

# Empacota termos e política, no modo do app, para ler sem rede. Precisa do backend de
# pé. Recusa rascunho, a não ser com LEGAL_BUNDLE_ALLOW_DRAFT=1.
legal-bundle:
	python3 tools/legal_bundle.py

# Gera as strings de internacionalização (i18n) para Swift (e futuramente Kotlin)
i18n:
	cargo run --manifest-path tools_i18n/Cargo.toml -- --language swift --output-dir ios/LogNiOS/LogNiOS/DesignSystem

# --- iOS (Swift) ---

# Gera a ponte FFI em Swift (Facet + Bincode) e joga na pasta do iOS.
# ATENÇÃO: este target sozinho deixa a biblioteca estática (.a) velha.
# O caminho normal para compilar tudo junto é o `build-ios-ffi`.
codegen:
	cd shared_core && cargo run --bin codegen --features codegen -- --language swift --output-dir ../ios/LogNCoreFFI/Sources/CodegenOut
	mv ios/LogNCoreFFI/Sources/App/BoltFFI ios/LogNCoreFFI/BoltFFI_tmp
	mv ios/LogNCoreFFI/Sources/App/boltffi.h ios/LogNCoreFFI/boltffi.h_tmp
	rm -rf ios/LogNCoreFFI/Sources/App ios/LogNCoreFFI/Sources/LogN ios/LogNCoreFFI/Sources/Serde
	cp -R ios/LogNCoreFFI/Sources/CodegenOut/App/Sources/App ios/LogNCoreFFI/Sources/
	cp -R ios/LogNCoreFFI/Sources/CodegenOut/App/Sources/LogN ios/LogNCoreFFI/Sources/
	cp -R ios/LogNCoreFFI/Sources/CodegenOut/App/Sources/Serde ios/LogNCoreFFI/Sources/
	mv ios/LogNCoreFFI/BoltFFI_tmp ios/LogNCoreFFI/Sources/App/BoltFFI
	mv ios/LogNCoreFFI/boltffi.h_tmp ios/LogNCoreFFI/Sources/App/boltffi.h
	rm -rf ios/LogNCoreFFI/Sources/CodegenOut

# Compila as bibliotecas estáticas (Rust) para iOS e empacota no XCFramework
build-ios-ffi: codegen
	cd shared_core && rustup target add aarch64-apple-ios aarch64-apple-ios-sim x86_64-apple-ios
	cd shared_core && cargo build --target aarch64-apple-ios --release
	cd shared_core && cargo build --target aarch64-apple-ios-sim --release
	cd shared_core && cargo build --target x86_64-apple-ios --release
	mkdir -p shared_core/target/universal-sim
	lipo -create -output shared_core/target/universal-sim/libshared_core.a shared_core/target/aarch64-apple-ios-sim/release/libshared_core.a shared_core/target/x86_64-apple-ios/release/libshared_core.a
	
	# Monta o XCFramework do zero
	rm -rf ios/LogNCoreFFI/LogNCoreFFI.xcframework
	mkdir -p shared_core/target/headers/shared_core
	cp ios/LogNCoreFFI/Sources/App/boltffi.h shared_core/target/headers/shared_core/shared_core.h
	echo 'module LogNCoreFFIFFI { header "shared_core/shared_core.h" export * }' > shared_core/target/headers/module.modulemap
	xcodebuild -create-xcframework -library shared_core/target/aarch64-apple-ios/release/libshared_core.a -headers shared_core/target/headers -library shared_core/target/universal-sim/libshared_core.a -headers shared_core/target/headers -output ios/LogNCoreFFI/LogNCoreFFI.xcframework

# Gera o projeto Xcode (.xcodeproj) usando o XcodeGen
xcode: sync-env build-ios-ffi i18n
	cd ios/LogNiOS && xcodegen generate

# Abre o projeto no Xcode
open-ios: xcode
	open ios/LogNiOS/LogNiOS.xcodeproj

# Builda em Release, assina com o time do project.yml e instala no iPhone conectado.
# Fala com o API_BASE_URL do .env (o `xcode` roda o sync-env antes), que hoje é a produção.
# Sem argumento, instala no primeiro iPhone pareado e disponível; com argumento, no id
# dado por `xcrun devicectl list devices`.
# Uso: just install-ios-device            ou   just install-ios-device <id>
install-ios-device device="": xcode
	#!/usr/bin/env bash
	set -euo pipefail
	device="{{device}}"
	if [ -z "$device" ]; then
		json=$(mktemp)
		xcrun devicectl list devices --json-output "$json" >/dev/null
		device=$(python3 -c 'import json,sys; ds=json.load(open(sys.argv[1]))["result"]["devices"]; ok=[d for d in ds if d["hardwareProperties"].get("platform")=="iOS" and d["connectionProperties"].get("pairingState")=="paired" and d["connectionProperties"].get("tunnelState")!="unavailable"]; print(ok[0]["identifier"] if ok else "")' "$json")
		rm -f "$json"
		if [ -z "$device" ]; then
			echo "Nenhum iPhone pareado e disponível. Conecte o aparelho e desbloqueie a tela." >&2
			exit 1
		fi
	fi
	xcodebuild -project ios/LogNiOS/LogNiOS.xcodeproj -scheme LogNiOS \
		-configuration Release -destination 'generic/platform=iOS' \
		-derivedDataPath ios/build/device -allowProvisioningUpdates build
	xcrun devicectl device install app --device "$device" \
		ios/build/device/Build/Products/Release-iphoneos/LogNiOS.app

# Limpa o build do Rust e do Xcode
clean:
	cd shared_core && cargo clean
	rm -rf ios/LogNiOS/LogNiOS.xcodeproj
	rm -rf ios/LogNiOS/.build

# Checa se há literais hardcoded nas views do iOS
i18n-check:
	tools/check_ui_literals.py
