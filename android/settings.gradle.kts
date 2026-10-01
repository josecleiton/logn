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

// A versão do app, de ../version.properties, o mesmo arquivo que dá a do iOS. Lida aqui
// uma vez e entregue a todo módulo por `extra`. Sem o arquivo o build para: um default
// é como um build sai com o número errado sem ninguém perceber.
val versionProperties =
    java.util.Properties().apply {
        val file = rootDir.resolve("../version.properties")
        require(file.isFile) { "version.properties não encontrado em ${file.canonicalPath}" }
        file.inputStream().use { load(it) }
    }

gradle.beforeProject {
    extra["lognVersionName"] = versionProperties.getProperty("name")
    extra["lognVersionCode"] = versionProperties.getProperty("build")
}

// :app é a interface em Compose. :core-shell é a ponte com o Core Rust e compila o que
// `just android/generate` gera em android/generated/ (tipos do Core, glue JNI, .so e as
// strings do catálogo); nada disso é módulo, e nada disso vai para o git (ADR 0023).
include(":app", ":core-shell")
