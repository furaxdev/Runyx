// Runyx Android app skeleton. Real pairing/voice/vision features land in
// M5 — see docs/ROADMAP.md. Not built or tested by CI yet (no Android SDK
// in the M0/M1 CI image); add an android-ci job once this module has real
// code to build.
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "runyx-android"
include(":app")
