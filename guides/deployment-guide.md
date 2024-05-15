# Deployment Guide

- Run Locally in Stand-Alone Mode
  - [Quick Start Guide](quickstart-guide.md)
- Deploying Event Log Collector Dependencies
  - [Cache](deployment-guides/dependencies/cache.md)
  - [Storage Providers](deployment-guides/dependencies/storage-providers.md)
  - [Data Pipeline](deployment-guides/dependencies/pipelines.md)
  - [Monitoring](deployment-guides/dependencies/monitoring.md)
- Deployment Examples
  - [Running `event-log-client` with redis cache and kakfa pipeline](deployment-examples/event-log-client-redis-kafka.md)
  - [Running each event log component separately](deployment-examples/event-log-components-distributed.md)
  - [Running multiple instances of `event-log-writer`](deployment-examples/event-log-writer-multiple-instances.md)
  - [Writing on-premise event logs to mix log.api](deployment-examples/event-log-on-premise-writer.md)
  - [Writing on-premise event logs to NEAP AFO/AFT](deployment-examples/event-log-on-premise-neap-writer.md)
  - [Using `docker-compose`](deployment-examples/event-log-components-docker-compose.md)
  - [Deployment to `kubernetes`](deployment-examples/event-log-components-kubernetes.md)
