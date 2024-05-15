# Running Each Event Log Collector Component in Docker-Compose

> These instructions assume 
>  * you've deployed redis, kafka, and OpenSearch using the docker-compose.yaml files provided within each respective deployment guide
>  * you've downloaded the `event-log-collector` source code so the Docker image can be built as part of the `docker-compose up` command

First, we'll update the event-log-collector config file used in [Running Each Event Log Collector Component Locally](event-log-components-distributed.md) with appropriate hostnames when run within the Docker run-time

**config.distributed-docker-compose.yaml**
```yaml
fetcher:
    config:
        consumer_options:
            auto.offset.reset: earliest

processor:
    name: mix-default
    config:
        filter:
            type: blacklist # blacklist or whitelist
            content_type:   # filter out some of the ASR event logs to reduce noise and storage space required
                - application/x-nuance-asr-statusresponse*
                - application/x-nuance-asr-startspeechresponse*
                - application/x-nuance-asr-partialresultresponse*
                - application/x-nuance-asr-finalresultresponse*
        cache:
            type: redis
            # parameters for redis cache
            # addr: # Default value is redis:6379
            # db: # Default value is 0
            # default_ttl: # Default value is 900
            # password: # Default value is empty

writer:
    name: opensearch
    config:
        ### Required Parameters

        # none

        ### Optional Parameters
        
        # index_prefix: # Default value is set to mix3-logs-v2
        # refresh: # Default value is true
        addresses:
            - http://opensearch:9200   # this is the default

        # num_workers: 50   # Num Workers specifies how many event logs can be processed in parallel by log writer
        # rate_limit:       # Rate Limit controls how fast event logs can be written to storage
            # limit: 1000
            # burst: 10

pipeline:
    storage_provider: kafka
    fetcher:
        writer:
            ## Required
            hosts:
                - kafka:9092                
            topic: queues.fetcher
    processor:
        reader:                
            ## Required
            hosts:
                - kafka:9092
            group: elc
            topic: queues.fetcher

            ## Optional                
            # offset: earliest # default is none
            # auto_commit_offset_enabled: false # default is true

        writer:
            ## Required
            hosts:
                - kafka:9092
            topic: queues.processor                
    writer:
        reader:
            ## Required
            hosts:
                - kafka:9092                
            group: elc
            topic: queues.processor

            ## Optional                
            # offset: earliest # default is none
            # auto_commit_offset_enabled: false # default is true
```

> Place this in the following path `./configs/config.distributed-docker-compose.yaml`

## event-log-writer

Create the following `docker-compose.writer.yaml` file

**docker-compose.writer.yaml**
```yaml
version: '3'

services:
  elc-log-writer:
    build: .
    container_name: elc-log-writer
    restart: unless-stopped
    environment:
      - ELC_ENABLE_MONITORING=1
    depends_on:
      - kafka
      - opensearch
    volumes:
      - ./configs/config.distributed-docker-compose.yaml:/nuance/configs/config.yaml
    entrypoint: [ "./bin/event-log-writer", "-c", "configs/config.yaml" ]
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

networks:
  event-log-collector:
    driver: bridge
```

**Command Line**
```
docker-compose -f docker-compose.writer.yaml up
```

## event-log-processor

Create the following `docker-compose.processor.yaml` file

**docker-compose.processor.yaml**
```yaml
version: '3'

services:
  elc-log-processor:
    build: .
    container_name: elc-log-processor
    restart: unless-stopped
    environment:
      - ELC_ENABLE_MONITORING=1
    depends_on:
      - kafka
      - redis
    volumes:
      - ./configs/config.distributed-docker-compose.yaml:/nuance/configs/config.yaml
    entrypoint: [ "./bin/event-log-processor", "-c", "configs/config.yaml" ]
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

networks:
  event-log-collector:
    driver: bridge
```

**Command Line**
```
docker-compose -f docker-compose.processor.yaml up
```

## event-log-fetcher

Create the following `docker-compose.fetcher.yaml` file

**docker-compose.fetcher.yaml**
```yaml
version: '3'

services:
  elc-log-fetcher:
    build: .
    container_name: elc-log-fetcher
    restart: unless-stopped
    environment:
      - ELC_CLIENT_ID=$ELC_CLIENT_ID
      - ELC_CLIENT_SECRET=$ELC_CLIENT_SECRET
      - ELC_ENABLE_MONITORING=1
    depends_on:
      - kafka
    volumes:
      - ./configs/config.distributed-docker-compose.yaml:/nuance/configs/config.yaml
    entrypoint: [ "./bin/event-log-fetcher", "-c", "configs/config.yaml" ]
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

networks:
  event-log-collector:
    driver: bridge
```

**Command Line**
```
ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> docker-compose -f docker-compose.fetcher.yaml up
```