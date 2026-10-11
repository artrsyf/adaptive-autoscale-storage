ThisBuild / version := "0.1.0-SNAPSHOT"
ThisBuild / scalaVersion := "3.3.7"

val tapirVersion = "1.13.15"

lazy val root = (project in file("."))
  .enablePlugins(JavaAppPackaging)
  .settings(
    name := "control-plane",
    Compile / mainClass := Some("storage.controlplane.Main"),
    libraryDependencies ++= Seq(
      "dev.zio" %% "zio" % "2.1.25",
      "dev.zio" %% "zio-http" % "3.10.1",
      "dev.zio" %% "zio-json" % "0.7.44",
      "com.softwaremill.sttp.tapir" %% "tapir-core" % tapirVersion,
      "com.softwaremill.sttp.tapir" %% "tapir-zio-http-server" % tapirVersion,
      "com.softwaremill.sttp.tapir" %% "tapir-json-zio" % tapirVersion,
      "org.tpolecat" %% "doobie-core" % "1.0.0-RC12",
      "org.tpolecat" %% "doobie-postgres" % "1.0.0-RC12",
      "org.tpolecat" %% "doobie-hikari" % "1.0.0-RC12",
      "dev.zio" %% "zio-interop-cats" % "23.1.0.13",
      "org.postgresql" % "postgresql" % "42.7.10",
      "org.yaml" % "snakeyaml" % "2.4",
      "org.slf4j" % "slf4j-simple" % "1.7.36",
      "dev.zio" %% "zio-test" % "2.1.25" % Test,
      "dev.zio" %% "zio-test-sbt" % "2.1.25" % Test
    ),
    dependencyOverrides += "dev.zio" %% "zio-json" % "0.7.44",
    testFrameworks += new TestFramework("zio.test.sbt.ZTestFramework"),
    Test / fork := true,
    scalacOptions ++= Seq("-deprecation", "-feature", "-unchecked", "-Wunused:all")
  )
