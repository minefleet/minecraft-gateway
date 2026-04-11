plugins {
    java
    alias(libs.plugins.shadow)
}

repositories {
    maven("https://repo.papermc.io/repository/maven-public/")
}

dependencies {
    compileOnly(libs.velocity.api)
    annotationProcessor(libs.velocity.api)
    implementation(project(":api"))
}

tasks.shadowJar {
    relocate("io.grpc", "dev.minefleet.shadow.io.grpc")
    relocate("com.google.protobuf", "dev.minefleet.shadow.com.google.protobuf")
    mergeServiceFiles()
}