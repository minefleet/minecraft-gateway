plugins {
    java
    id("com.gradleup.shadow") version "9.4.1"
}

repositories {
    maven("https://repo.papermc.io/repository/maven-public/")
}

dependencies {
    compileOnly("com.velocitypowered:velocity-api:3.5.0-SNAPSHOT")
    annotationProcessor("com.velocitypowered:velocity-api:3.5.0-SNAPSHOT")
    implementation(project(":api"))
}

tasks.shadowJar {
    relocate("io.grpc", "dev.minefleet.shadow.io.grpc")
    relocate("com.google.protobuf", "dev.minefleet.shadow.com.google.protobuf")
    mergeServiceFiles()
}