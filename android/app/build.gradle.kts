import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
}

// Release signing comes from android/keystore.properties (local builds) or
// SHORTR_KEYSTORE_* environment variables (CI). Without either, the release
// APK is built unsigned and the debug APK is what you install.
val keystoreProps = Properties().apply {
    val f = rootProject.file("keystore.properties")
    if (f.exists()) f.inputStream().use { load(it) }
}
fun signingValue(key: String, env: String): String? =
    keystoreProps.getProperty(key) ?: System.getenv(env)?.takeIf { it.isNotBlank() }

android {
    namespace = "io.github.anand34577.shortr"
    compileSdk = 37

    defaultConfig {
        applicationId = "io.github.anand34577.shortr"
        minSdk = 26
        targetSdk = 36
        versionCode = (System.getenv("SHORTR_VERSION_CODE") ?: "1").toInt()
        versionName = System.getenv("SHORTR_VERSION_NAME") ?: "1.0.0"
        // AppAuth's SSO redirect: io.github.anand34577.shortr:/oauth2redirect
        // (reverse-DNS scheme per RFC 8252, so it never clashes with shortr://connect)
        manifestPlaceholders["appAuthRedirectScheme"] = "io.github.anand34577.shortr"
    }

    val storePath = signingValue("storeFile", "SHORTR_KEYSTORE_FILE")
    if (storePath != null) {
        signingConfigs {
            create("release") {
                storeFile = rootProject.file(storePath)
                storePassword = signingValue("storePassword", "SHORTR_KEYSTORE_PASSWORD")
                keyAlias = signingValue("keyAlias", "SHORTR_KEY_ALIAS")
                keyPassword = signingValue("keyPassword", "SHORTR_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            if (storePath != null) signingConfig = signingConfigs.getByName("release")
        }
        debug {
            versionNameSuffix = "-debug"
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.core.splashscreen)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.graphics)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material.icons.extended)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.process)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.okhttp)
    implementation(libs.androidx.datastore.preferences)
    implementation(libs.androidx.camera.camera2)
    implementation(libs.androidx.camera.lifecycle)
    implementation(libs.androidx.camera.view)
    implementation(libs.mlkit.barcode.scanning)
    implementation(libs.appauth)
    implementation(libs.androidx.browser)
    implementation(libs.androidx.biometric)
    debugImplementation(libs.androidx.compose.ui.tooling)
    testImplementation(libs.junit)
}
