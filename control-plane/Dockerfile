FROM sbtscala/scala-sbt:eclipse-temurin-21.0.8_9_1.12.10_2.13.18 AS build
WORKDIR /src
COPY project ./project
COPY build.sbt .scalafmt.conf ./
RUN sbt -batch update
COPY src ./src
RUN sbt -batch stage

FROM eclipse-temurin:21.0.8_9-jre
RUN apt-get update && apt-get install -y --no-install-recommends wget && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/target/universal/stage /opt/control-plane
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/opt/control-plane/bin/control-plane"]
