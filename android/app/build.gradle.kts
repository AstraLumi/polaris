import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// The server's address is built into the app; kept out of git (see secrets.example.properties)
val secrets = Properties().apply {
    rootProject.file("secrets.properties").takeIf { it.exists() }?.reader()?.use { load(it) }
}
fun secret(name: String) = "\"" + (secrets.getProperty(name) ?: "").trim() + "\""

// The app carries the version of the Polaris it was built from (frontend/package.json)
val polarisVersion = Regex("\"version\"\\s*:\\s*\"(\\d+)\\.(\\d+)\\.(\\d+)\"")
    .find(rootProject.file("../frontend/package.json").readText())!!
    .groupValues.drop(1).map { it.toInt() }

android {
    namespace = "net.astralumi.polaris"
    compileSdk = 35

    defaultConfig {
        applicationId = "net.astralumi.polaris"
        minSdk = 29
        targetSdk = 35
        versionCode = polarisVersion[0] * 10000 + polarisVersion[1] * 100 + polarisVersion[2]
        versionName = polarisVersion.joinToString(".")

        buildConfigField("String", "SERVER_URL", secret("POLARIS_URL"))
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            // Personal sideloaded app: sign release builds with the debug key
            signingConfig = signingConfigs.getByName("debug")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        buildConfig = true
    }
}

dependencies {
    implementation("androidx.activity:activity-ktx:1.9.3")
    implementation("androidx.core:core-ktx:1.15.0")
    // window.print() and messages from the page, limited to the server's own pages
    implementation("androidx.webkit:webkit:1.12.1")
}
