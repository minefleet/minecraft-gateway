plugins {
    java
}

dependencies {
    implementation("com.google.protobuf:protobuf-java:4.34.1")
    implementation("io.grpc:grpc-stub:1.80.0")
    implementation("io.grpc:grpc-protobuf:1.80.0")
    compileOnly("org.apache.tomcat:annotations-api:6.0.53") // javax.annotation for generated code

    testImplementation("org.junit.jupiter:junit-jupiter:5.10.0")
    testImplementation("org.mockito:mockito-junit-jupiter:5.12.0")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.test {
    useJUnitPlatform()
}
