# Event Log Collector Release Notes

## [v2.7.2]
05-14-2024

`Enhancement`

* Upgrade to golang 1.22.3 and update dependencies

## [v2.7.1]
05-14-2024

`Enhancement`

* Upgrade to golang 1.21.3 and update dependencies
* Refactor filesystem writer to include topic name in path written to
* Pre-process records in io pipeline to ensure data consistency when there is log data redaction
* Add description with links to online docs in cli help

`Bugfix`

* Verify audioUrn exists before trying to use it to fetch audio
* Add formattedLiteral to nlu event log schema

## [v2.7.0]
07-19-2023

`Enhancement`

* Add support for elasticsearch and opensearch ingest pipelines
* Add flag to optionally disable appending the date to the elasticsearch and opensearch index name
* Retrieve elasticsearch version info to improve support, specifically for doc_type, in versions prior to version 8
* Update grafana configmap helm template

`Bugfix`

* Fix errors due to a mismatch in expected field counts when processing elasticsearch/opensearch errors and calling monitoring methods.

## [v2.6.1]
06-07-2023

`Enhancement`

- Add configuration parameter to kakfa pipeline reader to optionally create a unique group name. Allows for horizontal scaling of log processors and writers when a kafka topic is configured with more than one partition.
- Update helm charts with support for component-specific resource limits in values.yaml

## [v2.6.0]
05-26-2023

`Bugfix`

- Fix io pipeline default path name generation
- Fixed monitoring guide with instruction on installing prometheus-operator

`Enhancement`

- Improve configuration support of opensearch client. Supports most opensearch client config options, including a couple TLS config options
- Improve configuration support of elasticsearch client. Supports most elasticsearch client config options, including a couple TLS config options
- Updated configuration guide

## [v2.5.1]
04-10-2023

`Bugfix`

- When reading from on-premise Kafka, if a Key.ID is not available, assign Value.ID to Key.ID
- Ensure log writers close pipeline reader when being shutdown
- Update docker-compose yaml samples to enclose boolean env vars with quotes
  
##  [v2.5.0]
04-06-2023

`Enhancement`

- Added support for event log writer to write NII event logs to archive files that can be imported into NEAP for AFO/AFT
- Update golang to 1.20.2 and dependencies / vendoring to address security vulnerabilities

## [v2.4.2]
03-28-2023

`Bugfix`

- Map on-premise kafka event log schema to match that of event logs fetched from mix log.api
- Add support for operation timeout in io pipeline
- Miscellaneous minor bug fixes

## [v2.4.1]
03-08-2023

`Enhancement`

- Updated Quick Start and Deployment Guides
- Provide default MongoDB URI that points to localhost

`Bugfix`

- Fix logging statement in partition fetcher
- Fix helm chart values

## [v2.4.0]
03-02-2023

`Enhancement`

- Add support for Opensearch
- Add retry logic for 503 connection reset error when fetching logs from mix log api
- Add support for multiple instances of kafka pipeline readers with the use of uniquely generated group ID's.

## [v2.3.0]
02-28-2023

`Enhancement`

- Add support for Fluentd
- Update golang to 1.19.6 and dependencies / vendoring to address security vulnerabilities

## [v2.2.0]
02-08-2023

`Enhancement`

- Add support for MongoDB

`Bugfix`

- Fix sync with worker pool shutdown
- Fix uncaught error when closing Kafka pipeline reader

## [v2.1.0]
02-07-2023

`Enhancement`

- Add support for on-premise deployments of Mix coretech services. Event Log Collector now includes a log writer to the mix log producer api that allows for migrating event logs from on-premise (stored in kafka) to the Mix hosted run-time (for availability in Mix nlu.dashboard)

## [v2.0.9]
02-03-2023

`Enhancement`

- Add support for enabling/disabling auto commit of kafka offset with kafka pipeline reader

`Bugfix`

- Fix logging and monitoring of metrics with elasticsearch writer

## [v2.0.8]
02-01-2023

`Enhancement`

- Add support for downloading audio when using the filesystem log writer

## [v2.0.7]
01-30-2023

`Enhancement`

- Add filtering of sensitive data to logger
- Update image to use golang 1.19 to address security issues
- Add new scanning tools to analysis script
- Add manual commit of offset in mix log api fetcher when auto.commit.enable is not explicitly set to true

`Bugfix`

- Apply fixes to issues raised by code scans

## [v2.0.6]
01-23-2023

`Enhancement`

- Add more structure to folder names when writing log files to the filesystem. Groups log files by dialog session id and asr session id if they are available.

## [v2.0.5]
01-23-2023

`Bugfix`

- Fix calculation of offset behind
- fix crashing of clients when config file is not provided

`Enhancement`

- Add record fetch retry logic when a request to mix log api fails

## [v2.0.4]
01-03-2023

`Bugfix`

- Fix deployment guide

## [v2.0.3]
12-12-2022

`Enhancement`

- update grafana configmap for helm charts
- provide out-of-box provisioning of grafana and prometheus with docker-compose
- update guides
- update makefile

`Bugfix`

- Reset kafka consumer when consumer not found
- Fix metrics in http package

## [v2.0.2]
12-09-2022

`Enhancement`

- update documentation
- update scripts and makefile to work with current code base
- improve monitoring of event-log-collector components
- code cleanup
- build binaries for multiple platforms
- update helm charts to align with the new code base
- implement config samples

`Bugfix`
- fix processor's use of cache

## [v2.0.0]
11-28-22

`Enhancement`

- major re-design and re-write of event-log-collector to address scaling of data pipeline
- removed support for on-premise and dead-letter queues (to be added in future minor release)
- removed support for s3/minio storage


## [v1.0.2]
11-21-2022

`Enhancement`

- optimized memory allocation in mix3 consumer component
- fixed bugs related to client retry wrapper
- enhanced error handler for mix3 consumer
- applied dependency injection for authenticator in mix3 consumer
- increased test coverage for mix3 consumer
- upgraded to go 1.19
- enhanced code readability and reduced cluttered codes in mix3 consumer

## [v1.0.1]
07-21-2022

`Enhancement`

- client retry wrapper applied to mix3 log api client for clean code
- documentation enhanced

## [v1.0.0]
07-08-2022

`Enhancement`

- golang version upgraded to 1.18
- dependencies up-to-date
- separated out retrier logic from mix3 log api client's business logics
- set offset range check interval to 5 second to prevent consumer rebalance
- docker container security enhanced
- unit test added for local cache library
- gomock testing framework applied for retrier in mix3 log api client
- gitlab ci pipeine setup changed to use gorelaser-cross for faster release

## [v0.2.6]
04-18-2022

`Enhancement`
- refactored offset, config api
  - removed unecessary consumer recreation 
  - loosen schema validation for config set command from event-log-collector-helper

## [v0.2.5]
04-08-2022

`Enhancement`
- refactored config loading codes
- refactored the way processors are composed using base processor
- upgraded prometheus client library to the latest as a remediation of Veracode Scanning.

`Bugfix`
- fixed the bug of loading configuration in single or array of json or yaml
- unit tests added for loading configuration for multiple cases

## [v0.2.4]
04-01-2022

`Enhancement`
- added elc_events_data_size_in_bytes prometheus metric
- added graph showing elc_events_data_size_in_bytes metric in grafana dashboard

## [v0.2.3]
03-31-2022

`Enhancement`
- added unit tests for event-log-client 
- added unit tests for loading single object configuration
- restructured README.md into 4 parts
  - quickstart, configuration, developer, deployment
- refactored upgrade.sh and downgrade.sh to modify version using `yq` command line tool to modify version

`Bugfix`
- fixed issue of loading single json object configuration
- fixed issue of event-log-client cli

## [v0.2.2]
03-08-2022

`Enhancement`
- added Fluentd sidecar support for helm chart deployment
- the current latest fluentd image supports elasticsearch output plugin out of the box
  - the source code for custom fluent image is maintained here 
    - https://git.labs.nuance.com/mob-ps/xaas-logging/event-log-infra
## [v0.2.1]
03-03-2022

`Enhancement`
- added Sonarqube scan support 
  - added docker-compose-sonarqube.yaml to run sonarqube server easily 
  
## [v0.2.0]
02-28-2022

`Enhancement`
- helm chart deployment supports yaml format
  - backward compatible with json format confuration but will be deprecated in the next major release

## [v0.1.37]

02-17-2022

`Enhancement`
- yaml or yml extension is supported in loading configuration. 
- cross-compatibility is secured  among all consumers, processors, transformers, storage providers including mix3 processor and mix3 storage provider.  
## [v0.1.36]
02-14-2022

`Enhancement`
- release guideline documented in README.md
- make-release.sh has been modifed to support separate tasks into the following subtasks:
  - upgrade.sh is only for bumping version up on a semantic version level such as major, minor, patch.
  - commit.sh is for making a commit with message input from user before executing release.sh.
  - downgrade.sh is only for decreasing version by 1 on a desired level.
  - release.sh is taking care of creating a tag with the latest commit and pushing it to the origin master to trigger automated gitlab ci pipeline. 
## [v0.1.35]
02-09-2022

`Enhancement`

- gitlab ci pipeline has been setup for `master` branch for `push` or `web` trigger. and it's strictly following the Semantic Versioning of tags for making releases as it's relying on gorelease as the underlying tool. so be sure to push the tag of the latest commit to the origin before pushing the latest commit to the origin master. 
- rolled out max_retries in all storage providers and tested. if it's set to -1 ( value less than zero), it's going to be infinite retry so be careful to use the value, because the concurrency model only allows specific amount of workers to handle "commit" to storages. if all of workers are in infinite loop of writing, consumers can stop reading. 
- configuration for partitions in `Consumer` component has been modified to support array e.g) [1,2,3], and the wording has been changed to `partitions` from `partition`.


`Bugfix`

- fixed the bug of nil interface error in parsing map[string]interface{} jsonObj type in bi_analytics Transformer's Transform method
- fixed the bug of "Consumer instance not found." error not being caught for error enveloped in body, not error object. 
- removed the extra `-` in forming index pattern. for example, from `mix3-bi-analytics--*` to `mix3-bi-analytics-*`
  
## [v0.1.34]
01-14-2022

`Enhancement`
* `bi-analytics` processor and transformer is supported. 
* `rate_limit` is supported for all storage providers.
* `workerpool` is a new configuration to customize the number of workers for storage providers
* concurrency model has been optimized for better performance
## [v0.1.33]
12-20-2021

`Bugfix`
* support graceful shutdown of offset range check goroutines with multiple partitions in mix3_log_api and confluent_kafka_client consumers
* fixed json fields consistency in response of GET /offset api 
## [v0.1.32]
`Bugfix`
* fixed loading worker pools configuration for elasticsearch storage
## [v0.1.31]
12-08-2021

`Enhancement`
* Added periodic offset range check feature in consumer component for both confluent kafka consumer and mix3 log api consumer. 
  * for every 5 seconds, separate goroutine process checks the current, high, low offset. 
* Added metrics for monitoring offset ranges (current, high, low, behind)
  * metrics name and type
    * elc_consumer_offset_low
    * elc_consumer_offset_current
    * elc_consumer_offset_high
    * elc_consumer_offset_behind
* Added charts in grafana template to show offset range graphs
## [v0.1.30]
11-30-2021
* Added auto connection recovery for rabbitmq in mix3 storage
* graceful shutdown behavior modified to support CTRL+C signal to kill the entire process
  * 1st CTRL+C command will shutdown gracefully `Collector` process
  * With the `Collector` process shutdown completed, 2nd CTRL+C command will shutdown `Http Server` process
* Added docker compose yaml for dependencies to make it easy for new users to quickly run local proces using docker compose
  * it includes components such as kafka, kafdrop, zookeeper, rabbitmq

## [v0.1.29]
11-24-2021
* Added support for data recovery in times of service downtime at mix3 log api
  * As a part of mix3 storage's features, writing failed event logs to rabbitmq as dead letters is supported.
    * Additionally, ELC requires DQP(Dead Queue Processor) to handle those dead letters.
  * `max_retries` should be set to a value greater than zero so that mix3 storage's dead letter producer can be triggered when the threshold is crossed. 

> max_retries config example
```json
"type": "nuance-mix3",
"config": {
    "credentials": {
        ...omitted
    },
    ...omitted
    "max_retries": 10,
```
> New Configuration of Mix3 Storage for Dead Letter Queue
```json
"dead_letter_queue": {
    "type": "rabbitmq",
    "config": {
        "host": "amqp://guest:guest@localhost:5672/",
        "exchange_name": "dead-letters",
        "exchange_type": "direct",
        "queue_name": "dead-letters.elc",
        "routing_key": "dead-letters.elc"
    }
}
```
  * New prometheus metrics for dead letters are added
    * metrics 
      * elc_dead_letter_total ( counter type )
        * supported label: producer 
        * description : the number of dead letter produced.
      * elc_dead_letter_records_total ( counter type )
        * supported label: producer, reason, description
        * description : the number of dead letter records ( event logs ) produced.
  * Added 2 graph charts in the built-in grafana dashboard template, showing `elc_dead_letter_total,elc_dead_letter_records_total` status in the built-in grafana dashboard template 
  

    
## [v0.1.28]
10-26-2021
* Swagger API Documentation 
    * swagger api documentation is exposed when debug mode is enabled 
    * relevant contents are documented in README.md such as:
        * how to generate swagger docs using swagger command line
        * how to enable debug mode for swagger api
    
## [v0.1.27]
10-22-2021
* Performance Optimized
    * Introduced configurable queue & workers in storages
        * queue size and workers are configurable 
        * processors will push events to the storage queue and spawned workers will poll in parallel to max out performance  
        * check out configuration detail in [README.md](./README.md)
    * Introduced configurable thresholds in bytes for mix3 storage
        * mix3 storage will trigger flushing out cached records to mix3 endpoints whenever the threshold in bytes is crossed per worker records cache
        * check out configuration detail in [README.md](./README.md)
        
* API Utility Added
    * GET /api/v1/config
        * get runtime elc configuration json
    * GET /api/v1/offset
        * get offset range information based on consumers configuration
    * PUT /api/v1/config
        * modify runtime configuration and restart event log collector process in graceful way
    * PUT /api/v1/offset
        * set offset to a specific parameter
    * DELETE /api/v1/offset
        * resert offsets of all partitions of topics in configuration
