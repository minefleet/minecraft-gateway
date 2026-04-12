import org.gradle.api.tasks.testing.logging.TestExceptionFormat
import java.time.Instant

plugins {
    java
}

abstract class GenerateBuildInfoTask : DefaultTask() {
    @get:Input
    abstract val pluginVersion: Property<String>

    @get:Input
    abstract val commitHash: Property<String>

    @get:OutputDirectory
    abstract val outputDir: DirectoryProperty

    @TaskAction
    fun generate() {
        val buildTime = Instant.now().toString()
        val outDir = outputDir.get().asFile.resolve("dev/minefleet/api")
        outDir.mkdirs()
        outDir.resolve("BuildInfo.java").writeText(
            """
            package dev.minefleet.api;

            public final class BuildInfo {
                public static final String VERSION = "${pluginVersion.get()}";
                public static final String COMMIT_HASH = "${commitHash.get()}";
                public static final String BUILD_TIME = "$buildTime";

                private BuildInfo() {}
            }
            """.trimIndent()
        )
    }
}

val generateBuildInfo by tasks.registering(GenerateBuildInfoTask::class) {
    pluginVersion = providers.gradleProperty("pluginVersion").orElse("0.0.1-SNAPSHOT")
    commitHash = providers.gradleProperty("commitHash").orElse("unknown")
    outputDir = layout.buildDirectory.dir("generated/sources/buildinfo/java/main")
}

sourceSets.main {
    java.srcDir(generateBuildInfo.map { it.outputDir })
}

tasks.compileJava {
    dependsOn(generateBuildInfo)
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
