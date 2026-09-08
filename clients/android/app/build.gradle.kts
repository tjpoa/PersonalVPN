plugins {
    id("com.android.application")
    kotlin("android")
    kotlin("plugin.serialization")
}

import org.jetbrains.kotlin.gradle.dsl.JvmTarget

android { namespace = "com.personalvpn.client"; compileSdk = 35
    buildFeatures { buildConfig = true }
    defaultConfig {
        applicationId = "com.personalvpn.client"; minSdk = 26; targetSdk = 35; versionCode = 1; versionName = "0.1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
    buildTypes {
        getByName("debug") {
            buildConfigField("String", "API_BASE_URL", "\"https://10.0.2.2:8443\"")
        }
        getByName("release") {
            buildConfigField("String", "API_BASE_URL", "\"https://api.example.invalid\"")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
        isCoreLibraryDesugaringEnabled = true
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

dependencies {
    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
    androidTestImplementation("androidx.test:runner:1.6.2")
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.0.3")
    implementation("androidx.core:core-ktx:1.19.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.10.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.8.0")
    // Official embeddable WireGuard tunnel library; pin and review on upgrades.
    implementation("com.wireguard.android:tunnel:1.0.20260102")
}
