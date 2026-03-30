plugins {
    java
}

dependencies {
    implementation("com.google.protobuf:protobuf-java:4.34.1")
    implementation("io.grpc:grpc-stub:1.73.0")
    implementation("io.grpc:grpc-protobuf:1.73.0")
    compileOnly("org.apache.tomcat:annotations-api:6.0.53") // javax.annotation for generated code
}
