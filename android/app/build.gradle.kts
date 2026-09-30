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
        versionCode = 1
        versionName = "0.0.1"
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
