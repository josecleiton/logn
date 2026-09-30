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
include(":app")
