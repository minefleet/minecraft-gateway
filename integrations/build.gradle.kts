plugins {
    java
}

allprojects {
    group = "dev.minefleet"
    version = ((findProperty("pluginVersion") as String?) ?: "0.0.1-SNAPSHOT").removePrefix("v")

    repositories {
        mavenCentral()
    }
}

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(25))
    }
}
