// Cliente Android do LogN (ADR 0022). Por enquanto é o esqueleto que existe para o Play
// Console ter um AAB do pacote `sh.logn.app`: sem ele a Developer API responde
// `applicationNotFound`, e os produtos das trilhas não podem ser criados.

pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "LogN"

// :app é a interface em Compose. :core-shell é a ponte com o Core Rust e compila o que
// `just android/generate` gera em android/generated/ (tipos do Core, glue JNI, .so e as
// strings do catálogo); nada disso é módulo, e nada disso vai para o git (ADR 0023).
include(":app", ":core-shell")
