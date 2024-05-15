# Developer Guide

> `event-log-collector` is written in golang. 

This guide will review how to build the application, and how to use the Makefile provided to manage the dev lifecycle

## Pre-requisites

**Compiling / Building**
* golang 1.18+

**Running Event Log Collector Components**

> The following are required to fetch event log data from Mix `log.api`
> * Mix client ID and secret
> * Event log collection enabled for your Mix App ID (Nuance PS can assist you with this)

> One of the following are required for a `distributed data pipeline`
> * Kafka
> * Redis
> * Pulsar

> The following is required for `event log caching` for processors that do call log merging
> * Redis

> One of the following are required for `storage and analytics` if not writing to your local filesystem
> * Elasticsearch + Kibana
> * Opensearch + Opensearch Dashboard
> * MongoDB
> * Fluentd

## System Resources

* `Memory`
  
  A minimum of 1GB RAM is recommended. Total RAM required is dependent upon the amount of event log data being processed (ie. production vs dev).

* `CPU`
  
  A minimum of 1 CPU core is recommended. Total number of CPU cores required will be determined by the number of event logs being processed concurrently.

## Dev Lifecycle Tasks Using Make

A `Makefile` is provided that wraps access to the scripts provided under `/scripts`

**scan**

*launch sonarqube*
```
make docker-run-sonarqube
```

*start the scan*
```
make scan
```

**unit test**
```
make unit-test
```

**commit code changes**
```
make commit message="your commit message"
```

**bump version**

*after code has been approved and release notes updated, bump the version*
```
make update level=major|minor|patch
```

*if the version needs to be downgraded...*
```
make downgrade
```

**commit the new release**
```
make commit message="release 2.1.0"
```

**create new builds of the binaries**
```
make build
```

**containerize event-log-collector**
```
make docker-build
```

**tag the release**
```
make release
```

## Building `mix-event-log-collector`

To build `mix-event-log-collector`, run `make build` or run the script directly.

```
./scripts/setup.sh
````

This script will intialize golang mod vendoring and build the following:
* `event-log-fetcher`: The application that consumes logs from mix log.api
* `event-log-processor`: The application that validates, filters, and transforms event logs
* `event-log-writer`: The application that writes event logs to a storage provider
* `event-log-client`: A client that wraps all three of the above into a single stand-alone application.

## Creating a New Release

### Pre-requisites

* [yq](https://mikefarah.gitbook.io/yq/)

  The shell scripts provided are dependant on `yq` yaml procssor for bumping up versions in version releated files like Chart.yaml, Values.yaml, etc.

  Windows
  ```
  choco install yq
  ```

  macOS
  ```
  brew install yq
  ```

### 1. Add a release note to [release-notes.md](./release-notes.md)

    Use the following format when adding release notes:
    ```
    ## [v<Major>.<Minor>.<Patch>]
    MM-DD-YYYY

    - `Enhancement`
    - `Bugfix`
    ```

### 2. Bump the event-log-collector version 

Bump the version up on a desired level with the following command.
Specify either `patch`, `minor`, or `major` to bump the version to the appropriate level.

```
make upgrade level=[major|minor|patch]
```

> If you need to downgrade the version, specify `downgrade` instead of `upgrade` 


### 3. Commit the code with the new version in the commit message

```
make commit message="release v1.0.0"
```

### 4. Make a release

```
make release 
```

> **NOTE**: Only make releases on the `master` branch.
> 
> The `make release` command creates and pushes a tag using the latest commit after increasing the semantic version.
>
>  The gitlab ci pipeline is then triggered with the new tag (e.g v0.1.35 ). The pipeline is dependent upon a locally running gitlab-runner.


## Running Event Log Collector

Refer to the following documents:

* [Quick Start Guide](./quickstart-guide.md)
* [Deployment Guide](./deployment-guide.md)
* [Configuration Guide](./configuration-guide.md)

## Scan for Security Vulnerabilities

> The `make scan` command will run multiple static analysis tools, including:
> * `dockle`
>
>    Simple security auditing, helping build the Best Docker Images
> * `gosec`
>
>   Golang security checker
> * `sast-scan`
>
>   With an integrated multi-scanner based design, Scan can detect various kinds of security flaws in your application, and infrastructure code in a single fast scan without the need for any remote server
> * `semgrep`
>
>   Easily detect and prevent bugs and anti-patterns in your codebase
> * `trivy`
> 
>     Vulnerability scanner for container images, file systems, and Git repos
> * `sonar-scanner + sonar-qube`
>
>   Launcher to analyze a project with SonarQube
> 
>   Manage code quality

### Pre-requisites

* [dockle](https://github.com/goodwithtech/dockle)

  > [installation instructions](https://github.com/goodwithtech/dockle#installation)

  **Mac**
  ```
  brew install goodwithtech/r/dockle
  ```

* [gosec](https://github.com/securego/gosec)

  ```bash
  go get -u github.com/securego/gosec/v2/cmd/gosec
  ```

* [sast-scan](https://github.com/ShiftLeftSecurity/sast-scan)
  > runs as a docker container

* [semgrep](https://github.com/returntocorp/semgrep)

  > [installation instructions](https://semgrep.dev/docs/getting-started/)

  **Mac**

  ```
  brew install semgrep
  ```

* [trivy](https://github.com/aquasecurity/trivy)

  > [installation instructions](https://aquasecurity.github.io/trivy/v0.36/getting-started/installation/)

  **Mac**
  ```
  brew install trivy
  ```

* [sonar-scanner](https://docs.sonarqube.org/latest/analyzing-source-code/scanners/sonarscanner/)

  > Downloads available [here](https://docs.sonarqube.org/latest/analyzing-source-code/scanners/sonarscanner/)
  
  **Mac**
  ```
  brew install sonar-scanner
  ```

* [sonar-qube](https://docs.sonarqube.org/latest/try-out-sonarqube/)
  
  > runs as a docker container

  ```
  make docker-run-sonarqube
  ```

  > This project contains a `sonar-project.template.properties` file. Rename or copy this file to `sonar-project.properties` which you'll update with a sonarqube token and is used by the container to perform the scan. Before running a scan follow these steps to generate a token to update the properties file with.
  > * Follow the instructions [here](https://docs.sonarqube.org/latest/try-out-sonarqube/) for getting started with sonarqube once the container is up and running.
  > * After logging in, you will be prompted on how you want to create your project. Select `manually`.
  > * Enter `event-log-collector` for the project name and click on `Set Up`
  > * You will then be prompted for how you want to analyze your project. Select `Locally`
  > * Generate a token and update the `sonar.login` parameter in the `sonar-project.properties` file provided in this project


Scan event-log-collector codebase 

```makefile
make scan
```

> scan reports are available under the `reports` directory