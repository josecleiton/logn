import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

// A assinatura de release lê `android/keystore.properties`, fora do git (regra 9): o
// caminho da upload key e as senhas. Sem o arquivo, o release sai sem assinatura, e o
// Play Console recusa o upload — nunca uma chave de exemplo no lugar.
val keystoreFile = rootProject.file("keystore.properties")
val keystore =
    Properties().apply {
        if (keystoreFile.exists()) keystoreFile.inputStream().use { load(it) }
    }

// O que o app precisa saber do ambiente, lido do `.env` da raiz (ignorado) ou de uma
// propriedade `-Plogn.<CHAVE>`. Só estas chaves saem do `.env`: nenhum `*_SECRET` entra
// no APK (regra 9).
val appKeys = setOf("API_BASE_URL", "TELEMETRY_KEY", "POSTHOG_HOST", "GOOGLE_WEB_CLIENT_ID", "GITHUB_CLIENT_ID")
val dotenv: Map<String, String> =
    rootProject.file("../.env").let { file ->
        if (!file.exists()) {
            emptyMap()
        } else {
            file
                .readLines()
                .map { it.substringBefore('#').trim() }
                .filter { '=' in it }
                .associate { it.substringBefore('=').trim() to it.substringAfter('=').trim().trim('"') }
                .filterKeys { it in appKeys }
        }
    }

fun cfg(key: String): String = providers.gradleProperty("logn.$key").orNull ?: dotenv[key].orEmpty()

fun quoted(value: String): String = "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\""

android {
    namespace = "sh.logn.app"
    compileSdk = 37

    defaultConfig {
        applicationId = "sh.logn.app"
        // Tem de ser o min_sdk de shared_core/boltffi.toml.
        minSdk = 26
        targetSdk = 37
        // Cada upload no Play Console gasta um número: o de cima tem de ser maior que o
        // último enviado, mesmo que ele tenha sido recusado.
        versionCode = 3
        versionName = "0.1.0"
        // Tem de ser architectures de shared_core/boltffi.toml.
        ndk { abiFilters += listOf("arm64-v8a", "x86_64") }
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        listOf("TELEMETRY_KEY", "POSTHOG_HOST", "GOOGLE_WEB_CLIENT_ID", "GITHUB_CLIENT_ID")
            .forEach { buildConfigField("String", it, quoted(cfg(it))) }
    }

    signingConfigs {
        if (keystoreFile.exists()) {
            create("upload") {
                storeFile = file(keystore.getProperty("storeFile").replaceFirst("~", System.getProperty("user.home")))
                storePassword = keystore.getProperty("storePassword")
                keyAlias = keystore.getProperty("keyAlias")
                keyPassword = keystore.getProperty("keyPassword")
            }
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    buildTypes {
        debug {
            // O backend local, pelo `adb reverse tcp:8080 tcp:8080` (roteiro Android).
            buildConfigField(
                "String",
                "API_BASE_URL",
                quoted(providers.gradleProperty("logn.API_BASE_URL").orNull ?: "http://localhost:8080"),
            )
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            buildConfigField("String", "API_BASE_URL", quoted(cfg("API_BASE_URL")))
            // O Play simboliza o stack nativo do Core com a tabela de símbolos.
            ndk { debugSymbolLevel = "SYMBOL_TABLE" }
            if (keystoreFile.exists()) signingConfig = signingConfigs.getByName("upload")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    packaging { jniLibs.useLegacyPackaging = false }

    lint {
        abortOnError = true
        checkDependencies = true
    }

    testOptions { unitTests.isIncludeAndroidResources = true }
}

// Release só com a API de produção por HTTPS e o `.so` de release: o debug do Core não
// sai para a loja.
val verifyReleaseConfig by tasks.registering {
    val base = cfg("API_BASE_URL")
    val profile = rootProject.file("generated/PROFILE")
    doLast {
        check(base.startsWith("https://")) { "API_BASE_URL de release tem de ser https:// (ponha no .env)" }
        check(profile.exists() && profile.readText().trim() == "release") {
            "rode just android/generate release antes do build de release"
        }
    }
}
tasks.matching { it.name == "preReleaseBuild" }.configureEach { dependsOn(verifyReleaseConfig) }

composeCompiler {
    stabilityConfigurationFiles.add(rootProject.layout.projectDirectory.file("compose-stability.conf"))
}

dependencies {
    implementation(project(":core-shell"))

    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.foundation)
    implementation(libs.compose.material3)
    implementation(libs.compose.ui.tooling.preview)
    debugImplementation(libs.compose.ui.tooling)
    implementation(libs.activity.compose)
    implementation(libs.lifecycle.runtime.compose)
    implementation(libs.core.ktx)
    implementation(libs.core.splashscreen)
    implementation(libs.okhttp)
    implementation(libs.security.crypto)
    implementation(libs.posthog.android)
    implementation(libs.billing.ktx)
    implementation(libs.credentials)
    implementation(libs.credentials.play.services)
    implementation(libs.googleid)
    implementation(libs.browser)

    testImplementation(libs.junit)
    testImplementation(libs.robolectric)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.androidx.test.core)

    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.ext.junit)
}
