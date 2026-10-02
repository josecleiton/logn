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

# Roda o servidor Go localmente na porta 8080 (Lembre-se de ter o .env). O binário de
# desenvolvimento traz a documentação da API em http://localhost:8080/docs (ADR 0025).
run-backend: db-up
    cd backend && go run -tags dev .

# Roda todos os testes do Backend, nos dois binários: o de produção e o de desenvolvimento
test-backend:
    cd backend && go test -v ./...
    cd backend && go test -tags dev ./internal/httpapi/

# Regenera o spec da API a partir das anotações dos handlers. Rode depois de mexer em
# rota, corpo ou código de erro: o teste do backend falha se rota e spec divergirem.
api-docs:
    cd backend && go tool swag init --quiet -g main.go -d ./,./internal/httpapi --parseInternal --parseDependency --parseDependencyLevel 1 --outputTypes json -o internal/httpapi/apidocs

# Aplica as migrações no banco de Produção e encerra sem subir o servidor HTTP.
# Uso: DATABASE_URL="postgres://admin..." just migrate-prod
#
# Recusa publicar documento legal com marcador de rascunho ([A CONFIRMAR] e afins). Para
# levar um rascunho de propósito, durante a revisão do advogado:
# LEGAL_ALLOW_DRAFT=true DATABASE_URL=... just migrate-prod
migrate-prod:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "${LEGAL_ALLOW_DRAFT:-}" != "true" ]; then
    	DRAFTS=$(grep -lE 'A CONFIRMAR|TO CONFIRM|POR CONFIRMAR' backend/schema/migrations/*.sql | grep -v '0039_documentos_legais_v1_sem_meta.sql' || true)
    	if [ -n "$DRAFTS" ]; then
    		echo "Migração acima tem documento legal em rascunho. Publique o texto final ou use LEGAL_ALLOW_DRAFT=true." >&2
    		echo "$DRAFTS" >&2
    		exit 1
    	fi
    fi
    cd backend && RUN_MIGRATIONS=true MIGRATE_ONLY=true go run .

# Confere os documentos legais do repositório de conteúdo: estrutura, links, línguas e
# ids de seção. Com `release`, recusa também os marcadores de rascunho.
# Uso: just legal-check        ou        just legal-check release
legal-check mode="":
    cd backend && go run ./cmd/legalcheck {{ if mode == "release" { "--release" } else { "" } }} ../../logn-conteudo/legal

# Confere a trilha (logn-conteudo/trilha e glossario) antes de gerar a migração. Sem
# `release`, tradução e reescrita que faltam aparecem como pendência; com `release`,
# reprovam — é a porta para a submissão.
# Uso: just content-check        ou        just content-check release
content-check mode="":
    cd backend && go run ./cmd/contentcheck {{ if mode == "release" { "--release" } else { "" } }} ../../logn-conteudo

# Faz o deploy do Backend para o Google Cloud Run, a partir do Dockerfile de backend/.
# Variáveis e secrets vivem no serviço `logn` e são mantidos a cada deploy (via Terraform).
# A própria API (main.go) vai cuidar de aplicar as migrações no boot, usando a secret DATABASE_MIGRATION_URL.
deploy-backend:
    gcloud run deploy logn \
        --source ./backend \
        --region us-east1

# O job de expurgo do Cloud Scheduler é do Terraform (terraform/scheduler.tf), com conta
# própria (ADR 0021). Não há receita para criá-lo à mão: a antiga usava a conta do Cloud
# Run, que a purga recusa, e todo expurgo passava a dar 403 sem ninguém ver.

# Revoga à mão a licença de uma conta numa trilha paga e avisa por e-mail (termos,
# seção 10.5; ADR 0021). reason: redistribution ou account_sharing. A evidência fica
# gravada: o que foi visto e onde, sem copiar dado pessoal.
revoke user track reason evidence:
    python3 tools/license_admin.py revoke {{ quote(user) }} {{ quote(track) }} {{ quote(reason) }} {{ quote(evidence) }}

# Responde à contestação e avisa por e-mail. outcome: received (redistribuição, a trilha
# segue fechada), review (compartilhamento, a licença volta enquanto analisamos),
# accepted ou rejected.
appeal user track outcome evidence:
    python3 tools/license_admin.py appeal {{ quote(user) }} {{ quote(track) }} {{ quote(outcome) }} {{ quote(evidence) }}

# --- Terraform: estado e variáveis no bucket `<projeto>-tfstate` ---
# O bucket é privado, versionado e em us-east1 (Always Free). O nome sai do projeto
# ativo no gcloud; nada identificável fica no repositório.

# Liga o Terraform ao estado do bucket. Rode uma vez por máquina, antes do plan.
tf-init:
    terraform -chdir=terraform init -backend-config="bucket=$(gcloud config get project)-tfstate"

# Baixa o terraform.tfvars do bucket. Recusa sobrescrever um local diferente, que pode
# ter edição ainda não enviada; FORCE=1 sobrescreve.
tfvars-pull:
    #!/usr/bin/env bash
    set -euo pipefail
    remote="gs://$(gcloud config get project)-tfstate/terraform/terraform.tfvars"
    tmp=$(mktemp)
    trap 'rm -f "$tmp"' EXIT
    gcloud storage cp "$remote" "$tmp" --quiet
    if [ -f terraform/terraform.tfvars ] && ! cmp -s "$tmp" terraform/terraform.tfvars && [ "${FORCE:-}" != "1" ]; then
    	echo "terraform/terraform.tfvars local é diferente do bucket. Suba com 'just tfvars-push' ou rode com FORCE=1." >&2
    	exit 1
    fi
    install -m 600 "$tmp" terraform/terraform.tfvars
    echo "terraform.tfvars atualizado do bucket."

# Sobe o terraform.tfvars local como versão nova. As 20 anteriores continuam no bucket.
tfvars-push:
    gcloud storage cp terraform/terraform.tfvars "gs://$(gcloud config get project)-tfstate/terraform/terraform.tfvars"

# Roda o Terraform com o token da Cloudflare lido do Keychain do macOS, sem ele passar
# por arquivo nenhum: nem o tfvars, nem o bucket. O item se chama
# logn-cloudflare-api-token e é criado uma vez, com o valor digitado no prompt:
#   security add-generic-password -a "$USER" -s logn-cloudflare-api-token -w
# Sem o item, para aqui, antes do Terraform cair no placeholder do variables.tf.
# Uso: just tf plan        just tf apply        just tf state list
tf *args:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! token=$(security find-generic-password -s logn-cloudflare-api-token -w 2>/dev/null); then
    	echo "Falta o item logn-cloudflare-api-token no Keychain (veja o comentário da receita)." >&2
    	exit 1
    fi
    TF_VAR_cloudflare_api_token="$token" terraform -chdir=terraform {{ args }}

# --- Landing page (Cloudflare Worker) ---

# Gera landing/dist nas três línguas. Com LOGN_PLAY_STORE_URL no ambiente, os botões
# levam ao Google Play; sem ele, "em breve". LOGN_APP_STORE_URL põe a App Store ao lado,
# e LOGN_API_ORIGIN abre a lista de espera do iPhone (landing/README.md).
landing-build:
    node landing/build.mjs

# Serve a landing em http://127.0.0.1:8788, com o mesmo Worker e os mesmos cabeçalhos.
landing-dev:
    cd landing && npx wrangler dev --port 8788

# Sobe a landing em logn.sh e www.logn.sh (rotas em landing/wrangler.toml), com o que está
# no ar: o link do Google Play e a lista de espera do iPhone aberta. Sem os dois, um deploy
# voltava a página a "em breve" e fechava a lista. As duas URLs são públicas. Para mudar,
# passe a variável; vazia (`LOGN_API_ORIGIN= just landing-deploy`) desliga aquele item.
landing-deploy:
    cd landing && \
        LOGN_API_ORIGIN="${LOGN_API_ORIGIN-https://api.logn.sh}" \
        LOGN_PLAY_STORE_URL="${LOGN_PLAY_STORE_URL-https://play.google.com/store/apps/details?id=sh.logn.app}" \
        npx wrangler deploy

# Aponta o proxy /legal/* da landing para o serviço `logn` do Cloud Run. A URL sai do
# gcloud direto para o secret do Worker, sem passar por arquivo versionado.
# O destino do redirect de `logn.sh/legal/*`: o domínio público da API, nunca a URL
# `.run.app`, que recusa `/legal` pela verificação de origem e apareceria no endereço.
# Uso: just landing-legal-origin https://api.logn.sh
landing-legal-origin origin:
    printf '%s' "{{ origin }}" | (cd landing && npx wrangler secret put LEGAL_ORIGIN)

# --- Core (Rust) ---

# Roda os testes da maquina de estados Crux
test-core:
    cd shared_core && cargo test

# --- Ambientes e Shells ---

# Sincroniza as chaves do .env raiz para o formato consumível das shells nativas
sync-env:
    @echo "// Generated auto-magically from .env by Justfile" > ios/LogNiOS/Local.xcconfig
    @# Segredo do servidor (`*_SECRET`) não vai para o build do app: config de build aparece
    @# em log do xcodebuild, e um `${...}` no Info.plist o embarcaria (ADR 0019). A chave
    @# pessoal do posthog-cli (`POSTHOG_CLI_*`, ADR 0024) também não.
    @sed -e 's/#.*//g' -e '/^$/d' -e '/^[A-Z0-9_]*_SECRET=/d' -e '/^POSTHOG_CLI_[A-Z_]*=/d' -e 's/\/\//\/\$()\//g' .env >> ios/LogNiOS/Local.xcconfig
    @echo "Local.xcconfig synced from .env!"

# Empacota a trilha atual (nós + desafios) dentro do app, para a primeira abertura sem
# rede, uma trilha por língua. Precisa do backend de pé. Rode de novo depois de cada
# migração de conteúdo. `just seed-bundle --release` recusa língua com trilha menor.
seed-bundle *flags:
    python3 tools/seed_bundle.py {{ flags }}

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
xcode: sync-env build-ios-ffi i18n xcodegen

# Só o XcodeGen, sem recompilar o Core. A versão vem de version.properties: o
# project.yml não lê arquivo, só variável de ambiente, e é aqui que as duas se ligam.
# Sem default: versão vazia vira CFBundleShortVersionString vazio, que a App Store
# recusa na submissão e nada percebe antes.
xcodegen:
    #!/usr/bin/env bash
    set -euo pipefail
    LOGN_VERSION_NAME=$(sed -nE 's/^name=(.*)$/\1/p' version.properties)
    LOGN_VERSION_BUILD=$(sed -nE 's/^build=(.*)$/\1/p' version.properties)
    if [ -z "$LOGN_VERSION_NAME" ] || [ -z "$LOGN_VERSION_BUILD" ]; then
    	echo "version.properties não tem name= e build=" >&2
    	exit 1
    fi
    export LOGN_VERSION_NAME LOGN_VERSION_BUILD
    cd ios/LogNiOS && xcodegen generate

# Falha se o Info.plist do iOS não diz a versão de version.properties: um projeto gerado
# antes de um bump, ou alguém que trocou o literal por `$(MARKETING_VERSION)`. O Android
# lê o arquivo a cada build e não precisa de conferência.
version-check:
    #!/usr/bin/env bash
    set -euo pipefail
    name=$(sed -nE 's/^name=(.*)$/\1/p' version.properties)
    build=$(sed -nE 's/^build=(.*)$/\1/p' version.properties)
    plist=ios/LogNiOS/LogNiOS/Info.plist
    got_name=$(plutil -extract CFBundleShortVersionString raw "$plist" 2>/dev/null || echo '<ausente>')
    got_build=$(plutil -extract CFBundleVersion raw "$plist" 2>/dev/null || echo '<ausente>')
    if [ "$got_name" != "$name" ] || [ "$got_build" != "$build" ]; then
    	echo "✗ $plist diz $got_name ($got_build), version.properties diz $name ($build) — rode just xcodegen" >&2
    	exit 1
    fi
    echo "✓ iOS e Android em $name ($build)"

# Abre o projeto no Xcode
open-ios: xcode
    open ios/LogNiOS/LogNiOS.xcodeproj

# Builda em Release, assina com o DEVELOPMENT_TEAM do Local.xcconfig e instala no iPhone conectado.
# Fala com o API_BASE_URL do .env (o `xcode` roda o sync-env antes), que hoje é a produção.
# Sem argumento, instala no primeiro iPhone pareado e disponível; com argumento, no id
# dado por `xcrun devicectl list devices`.
# Uso: just install-ios-device            ou   just install-ios-device <id>
install-ios-device device="": xcode
    #!/usr/bin/env bash
    set -euo pipefail
    device="{{ device }}"
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

# A trilha e os documentos legais que viajam dentro do app saem da própria API de
# produção (API_BASE_URL do .env), nunca do banco local: o app offline mostra
# exatamente o que a produção serve. Por isso a produção já tem de estar com as
# migrações e o backend novos. O build para antes de compilar se:
# - o conteúdo ou os documentos em logn-conteudo têm pendência ou rascunho;
# - alguma língua não está publicada inteira na produção (seed --release);
# - a produção não manda Content-Language (backend antigo);
# - algum documento legal da produção está em rascunho.
#
# Saída: ios/build/store/LogNiOS.ipa. Envie pelo Transporter ou pelo Organizer. Para o
# seu iPhone, o caminho é o `install-ios-device`.
#
# Build de loja: arquiva em Release e exporta o .ipa da App Store, sem enviar.
release-ios:
    #!/usr/bin/env bash
    set -euo pipefail
    base=$(sed -n 's/^API_BASE_URL=//p' .env | tr -d '"' | tail -1)
    if [ -z "$base" ]; then echo "API_BASE_URL vazio no .env" >&2; exit 1; fi
    echo "Conteúdo e documentos legais de $base"
    {{ just_executable() }} content-check release
    {{ just_executable() }} legal-check release
    env -u LEGAL_BUNDLE_ALLOW_DRAFT SEED_BUNDLE_BASE_URL="$base" python3 tools/seed_bundle.py --release
    env -u LEGAL_BUNDLE_ALLOW_DRAFT LEGAL_BUNDLE_BASE_URL="$base" python3 tools/legal_bundle.py
    {{ just_executable() }} xcode
    {{ just_executable() }} version-check
    rm -rf ios/build/store
    xcodebuild -project ios/LogNiOS/LogNiOS.xcodeproj -scheme LogNiOS \
    	-configuration Release -destination 'generic/platform=iOS' \
    	-archivePath ios/build/store/LogNiOS.xcarchive -allowProvisioningUpdates archive
    xcodebuild -exportArchive -archivePath ios/build/store/LogNiOS.xcarchive \
    	-exportOptionsPlist ios/LogNiOS/ExportOptions-AppStore.plist \
    	-exportPath ios/build/store -allowProvisioningUpdates
    echo "Pronto: ios/build/store/LogNiOS.ipa (envie pelo Transporter ou pelo Organizer)"

# Limpa o build do Rust e do Xcode
clean:
    cd shared_core && cargo clean
    rm -rf ios/LogNiOS/LogNiOS.xcodeproj
    rm -rf ios/LogNiOS/.build

# Checa se há literais hardcoded nas views do iOS
i18n-check:
    tools/check_ui_literals.py
