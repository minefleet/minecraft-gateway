FROM gradle:9-jdk25 AS build
ARG PLUGIN_VERSION=0.0.1-SNAPSHOT
ARG COMMIT_HASH=unknown
WORKDIR /workspace
COPY integrations/ .
COPY --chmod=755 tools/latest_build_url.sh /tools/latest_build_url.sh
RUN apt-get update -q && apt-get install -y -q --no-install-recommends jq && rm -rf /var/lib/apt/lists/*
RUN chmod +x gradlew && ./gradlew :velocity:shadowJar --no-daemon -PpluginVersion=${PLUGIN_VERSION} -PcommitHash=${COMMIT_HASH}

ARG VERSION=3.5.0-SNAPSHOT
RUN curl -fSL -o /velocity.jar "$(PROJECT=velocity VERSION=${VERSION} sh /tools/latest_build_url.sh)"


FROM eclipse-temurin:25.0.2_10-jre-alpine

WORKDIR /velocity
COPY --from=build /velocity.jar ./velocity.jar
RUN mkdir -p plugins
COPY --from=build /workspace/velocity/build/libs/velocity-*-all.jar plugins/minefleet.jar

EXPOSE 25565

ENTRYPOINT ["java", \
    "-XX:+UseG1GC", \
    "-XX:G1HeapRegionSize=4M", \
    "-XX:+UnlockExperimentalVMOptions", \
    "-XX:+ParallelRefProcEnabled", \
    "-XX:+AlwaysPreTouch", \
    "-XX:MaxInlineLevel=15", \
    "-jar", "velocity.jar"]