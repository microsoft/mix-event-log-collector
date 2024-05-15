# Deploying Event Log Collector to Kubernetes

## Pre-Requisites

* Access to the helm charts (Please reach out to Nuance PS or your account manager to get access to `event-log-collector` source code and helm charts)

The helm charts will deploy a single instance of:

* event-log-fetcher
* event-log-processor
* event-log-writer

> Deployment also includes a default grafana chart

**Instructions**

Update `values.yaml`. Most notable sections needing attention are:

* image.repository
* image.tag
* environment
* eventLogCollector.config
  * provide a valid Mix client ID and secret if not using environment vars
  * in the pipeline configuration, it's good practice to use your Mix AppID to create unique topics 

```yaml
image:
  # Provide a valid repository 
  repository: <repository name>event-log-collector
  tag: latest
  pullPolicy: Always

# Update the config content to meet your specific event log collector configuration requirements
eventLogCollector:
  config:
    #filename: config.yaml
    config.yaml: |-
      fetcher:
      name: mix-log-api
      config:
          credentials:
              auth_disabled: false
              token_url: https://auth.crt.nuance.com/oauth2/token
              scope: log
              client_id: Provide a Mix Client ID
              client_secret: Provide a Mix Client Secret
              auth_timeout_ms: 5000
          max_retries: -1
          retry_delay: 5
          record_check_frequency: 10
          client_name: nuance-mob-ps-log-collector #neap
          consumer_group_id: "03"
          api_url: https://log.api.nuance.com
          consumer_options:
              auto.offset.reset: latest

      processor:
          name: mix-default
          config:
              num_workers: 10
              filter:
                  type: whitelist
                  content_type:
                  - application/x-nuance-asr-callsummary*
              cache:
                  type: redis
                  # redis params
                  addr: redis-master:6379
                  db: 0
                  default_ttl: 900
                  password: RedisIsGreat
      writer:
          name: opensearch
          config:
              index_prefix: elc-v2-mix-logs
              refresh: true
              addresses:
                  - http://opensearch-master.analytics:9200

              num_workers: 10
              rate_limit: 
                  limit: 10
                  burst: 1
              max_retries: 10
              delay: 10

      pipeline:
          storage_provider: kafka
          fetcher:
              writer:
                  hosts:
                    - kafka:9092
                  topic: queues.fetcher

          processor:
              reader:
                  hosts:
                    - kafka:9092                  
                  group: elc
                  topic: queues.fetcher
                  # offset: earliest

              writer:
                  hosts:
                    - kafka:9092
                  topic: queues.processor

          writer:
              reader:
                  hosts:
                    - kafka:9092                  
                  group: elc
                  topic: queues.processor
                  # offset: earliest
```

Deploy the helm chart

```
NAMESPACE=analytics

helm upgrade --install --namespace ${NAMESPACE} --create-namespace ${NAMESPACE} \
    -f helm-charts/event-log-collector/values.yaml \
    helm-charts/event-log-collector
```
