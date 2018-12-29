---
layout: "docs"
page_title: "Drivers: Java"
sidebar_current: "docs-drivers-java"
description: |-
  The Java task driver is used to run Jars using the JVM.
---

# Java Driver

Name: `java`

The `java` driver is used to execute Java applications packaged into a Java Jar
file. The driver requires the Jar file to be accessible from the Nomad
client via the [`artifact` downloader](/docs/job-specification/artifact.html).

## Task Configuration

```hcl
task "webservice" {
  driver = "java"

  config {
    jar_path    = "local/example.jar"
    jvm_options = ["-Xmx2048m", "-Xms256m"]
  }
}  
```

The `java` driver supports the following configuration in the job spec:

* `class` - (Optional) The name of the class to run. If `jar_path` is specified
  and the manifest specifies a main class, this is optional. If shipping classes
  rather than a Jar, please specify the class to run and the `class_path`.

* `class_path` - (Optional) The `class_path` specifies the class path used by
  Java to lookup classes and Jars.

* `jar_path` - (Optional) The path to the downloaded Jar. In most cases this will just be
  the name of the Jar. However, if the supplied artifact is an archive that
  contains the Jar in a subfolder, the path will need to be the relative path
  (`subdir/from_archive/my.jar`).

* `args` - (Optional) A list of arguments to the Jar's main method. References
  to environment variables or any [interpretable Nomad
  variables](/docs/runtime/interpolation.html) will be interpreted before
  launching the task.

* `jvm_options` - (Optional) A list of JVM options to be passed while invoking
  java. These options are passed without being validated in any way by Nomad.

## Examples

A simple config block to run a Java Jar:

```hcl
task "web" {
  driver = "java"

  config {
    jar_path    = "local/hello.jar"
    jvm_options = ["-Xmx2048m", "-Xms256m"]
  }

  # Specifying an artifact is required with the "java" driver. This is the
  # mechanism to ship the Jar to be run.
  artifact {
    source = "https://internal.file.server/hello.jar"

    options {
      checksum = "md5:123445555555555"
    }
  }
}
```

A simple config block to run a Java class:

```hcl
task "web" {
  driver = "java"

  config {
    class       = "Hello"
    class_path  = "${NOMAD_TASK_DIR}"
    jvm_options = ["-Xmx2048m", "-Xms256m"]
  }

  # Specifying an artifact is required with the "java" driver. This is the
  # mechanism to ship the Jar to be run.
  artifact {
    source = "https://internal.file.server/Hello.class"

    options {
      checksum = "md5:123445555555555"
    }
  }
}
```

## Client Requirements

The `java` driver requires Java to be installed and in your system's `$PATH`. On
Linux, Nomad must run as root since it will use `chroot` and `cgroups` which
require root privileges. The task must also specify at least one artifact to
download, as this is the only way to retrieve the Jar being run.

## Client Attributes

The `java` driver will set the following client attributes:

* `driver.java` - Set to `1` if Java is found on the host node. Nomad determines
this by executing `java -version` on the host and parsing the output
* `driver.java.version` - Version of Java, ex: `1.6.0_65`
* `driver.java.runtime` - Runtime version, ex: `Java(TM) SE Runtime Environment (build 1.6.0_65-b14-466.1-11M4716)`
* `driver.java.vm` - Virtual Machine information, ex: `Java HotSpot(TM) 64-Bit Server VM (build 20.65-b04-466.1, mixed mode)`

Here is an example of using these properties in a job file:

```hcl
job "docs" {
  # Only run this job where the JVM is higher than version 1.6.0.
  constraint {
    attribute = "${driver.java.version}"
    operator  = ">"
    value     = "1.6.0"
  }
}
```

## Resource Isolation

The resource isolation provided varies by the operating system of
the client and the configuration.

On Linux, Nomad will attempt to use cgroups, namespaces, and chroot
to isolate the resources of a process. If the Nomad agent is not
running as root, many of these mechanisms cannot be used.

As a baseline, the Java jars will be run inside a Java Virtual Machine,
providing a minimum amount of isolation.
