import org.gradle.api.tasks.testing.logging.TestExceptionFormat

plugins {
    java
}

val mockitoAgent = configurations.create("mockitoAgent")!!

dependencies {
    implementation(libs.bundles.grpc)
    compileOnly(libs.javax.annotations)

    testImplementation(libs.bundles.junit)
    testImplementation(libs.bundles.mockito)
    testRuntimeOnly(libs.junit.platform.launcher)
    mockitoAgent(libs.mockito.core) { isTransitive = false }
}

tasks.test {
    useJUnitPlatform()
    testLogging {
        events("passed", "skipped", "failed")
        showStandardStreams = true
        exceptionFormat = TestExceptionFormat.FULL
    }
    val agentJar = mockitoAgent.asPath
    doFirst {
        jvmArgs("-javaagent:$agentJar")
    }
}
