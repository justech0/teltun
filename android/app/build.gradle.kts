plugins {
    id("com.android.application")
}

android {
    namespace = "app.teltun"
    compileSdk = 36

    defaultConfig {
        applicationId = "app.teltun"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1.2.0-beta1"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            // Test builds are signed with the debug key; Play Store builds use a real key.
            signingConfig = signingConfigs.getByName("debug")
        }
    }
}

dependencies {
    // Go engine (internal/droid via ./mobile), built by build-android.ps1 with gomobile.
    implementation(files("libs/teltun.aar"))
}
