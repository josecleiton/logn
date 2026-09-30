import java.util.Properties

plugins {
    id("com.android.application")
}

// A assinatura de release lê `android/keystore.properties`, fora do git (regra 9): o
// caminho da upload key e as senhas. Sem o arquivo, o release sai sem assinatura, e o
// Play Console recusa o upload — nunca uma chave de exemplo no lugar.
val keystoreFile = rootProject.file("keystore.properties")
val keystore = Properties().apply {
    if (keystoreFile.exists()) keystoreFile.inputStream().use { load(it) }
}

android {
    namespace = "sh.logn.app"
    compileSdk = 36

    defaultConfig {
        applicationId = "sh.logn.app"
        minSdk = 26
        targetSdk = 36
        // Cada upload no Play Console gasta um número: o de cima tem de ser maior que o
        // último enviado, mesmo que ele tenha sido recusado.
        versionCode = 2
        versionName = "0.0.2"
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

    buildTypes {
        release {
            isMinifyEnabled = false
            if (keystoreFile.exists()) signingConfig = signingConfigs.getByName("upload")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    // A Play Billing Library (ADR 0022). É ela quem declara a permissão BILLING, e é
    // pela versão dela que o Play Console decide se o app pode vender: a permissão
    // escrita à mão, sem a biblioteca, conta como a API AIDL antiga e é recusada. A
    // compra ainda não é chamada; o cliente de verdade usa esta mesma dependência.
    implementation("com.android.billingclient:billing:9.1.0")
}
