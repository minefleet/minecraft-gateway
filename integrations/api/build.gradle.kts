import org.gradle.api.tasks.testing.logging.TestExceptionFormat
import java.time.Instant

plugins {
    java
    signing
    alias(libs.plugins.sonatype.publisher)
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

tasks.javadoc {
    (options as StandardJavadocDocletOptions).addStringOption("Xdoclint:none", "-quiet")
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

signing {
    val gpgKey = System.getenv("GPG_KEY")
    val gpgKeyPassword = System.getenv("GPG_KEY_PASSWORD") ?: ""
    if (gpgKey != null) {
        useInMemoryPgpKeys(gpgKey, gpgKeyPassword)
    }
}

centralPortal {
    username = System.getenv("MAVEN_CENTRAL_USERNAME") ?: ""
    password = System.getenv("MAVEN_CENTRAL_PASSWORD") ?: ""
    pom {
        name = "Minefleet Network API"
        description = "Minefleet network integration API"
        url = "https://github.com/minefleet/minecraft-gateway"
        licenses {
            license {
                name = "Apache License, Version 2.0"
                url = "https://www.apache.org/licenses/LICENSE-2.0"
            }
        }
        developers {
            developer {
                id = "minefleet"
                name = "The Minefleet Authors"
                url = "https://minefleet.dev"
            }
        }
        scm {
            connection = "scm:git:git://github.com/minefleet/minecraft-gateway.git"
            developerConnection = "scm:git:ssh://github.com/minefleet/minecraft-gateway.git"
            url = "https://github.com/minefleet/minecraft-gateway"
        }
    }
}

