# Running Each Event Log Collector Component Locally

This example builds upon [Running Event Log Collector with Distributed Cache and Data Pipeline](event-log-client-redis-kafka.md). We'll start with the configuration used in that example and extend it by:

 1. Adding OpenSearch as the designated storage provider
 2. Filtering out some of the ASR event logs
 3. Running each event-log-collector component individually

**Instructions**
1. Deploy [redis](../deployment-guides/dependencies/cache.md#docker-compose)
2. Deploy [kafka](../deployment-guides/dependencies/pipelines/kafka.md#docker-compose)
3. Deploy [OpenSearch](../deployment-guides/dependencies/storage-providers/opensearch.md#docker-compose)
4. Configure Event Log `Processor`, `Writer` and `Data Pipeline`

    **Use the `mix-default` Processor to merge and normalize event logs**
    ```yaml
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
                addr: localhost:6379 # Default value is redis:6379
                # db: # Default value is 0
                # default_ttl: # Default value is 900
                # password: # Default value is empty
    ```

    **Use the `OpenSearch` Log Writer**
    ```yaml
    writer:
        name: opensearch
        # config:
            ### Required Parameters

            # none

            ### Optional Parameters
            
            # index_prefix: # Default value is set to mix3-logs-v2
            # refresh: # Default value is true
            # addresses:
                # - http://localhost:9200   # this is the default

            # num_workers: 50   # Num Workers specifies how many event logs can be processed in parallel by log writer
            # rate_limit:       # Rate Limit controls how fast event logs can be written to storage
                # limit: 1000
                # burst: 10
    ```

    **Enable Kafka for the `data pipeline`**
    ```yaml
    pipeline:
        storage_provider: kafka
        fetcher:
            writer:
                ## Required
                hosts:
                    - 127.0.0.1:9092                
                topic: queues.fetcher
        processor:
            reader:                
                ## Required
                hosts:
                    - 127.0.0.1:9092
                group: elc
                topic: queues.fetcher

                ## Optional                
                # offset: earliest # default is none
                # auto_commit_offset_enabled: false # default is true

            writer:
                ## Required
                hosts:
                    - 127.0.0.1:9092
                topic: queues.processor                
        writer:
            reader:
                ## Required
                hosts:
                    - 127.0.0.1:9092                
                group: elc
                topic: queues.processor

                ## Optional                
                # offset: earliest # default is none
                # auto_commit_offset_enabled: false # default is true
    ```

    Create the following `config.distributed.yaml` file with the configuration details provided above

    **config.distributed.yaml**
    ```yaml
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
                addr: localhost:6379 # Default value is redis:6379
                # db: # Default value is 0
                # default_ttl: # Default value is 900
                # password: # Default value is empty

    writer:
        name: opensearch
        # config:
            ### Required Parameters

            # none

            ### Optional Parameters
            
            # index_prefix: # Default value is set to mix3-logs-v2
            # refresh: # Default value is true
            # addresses:
                # - http://localhost:9200   # this is the default

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
                    - 127.0.0.1:9092                
                topic: queues.fetcher
        processor:
            reader:                
                ## Required
                hosts:
                    - 127.0.0.1:9092
                group: elc
                topic: queues.fetcher

                ## Optional                
                # offset: earliest # default is none
                # auto_commit_offset_enabled: false # default is true

            writer:
                ## Required
                hosts:
                    - 127.0.0.1:9092
                topic: queues.processor                
        writer:
            reader:
                ## Required
                hosts:
                    - 127.0.0.1:9092                
                group: elc
                topic: queues.processor

                ## Optional                
                # offset: earliest # default is none
                # auto_commit_offset_enabled: false # default is true
    ```

    > **NOTE:** In this example we're using `event-log-processor` to filter and modify event logs before having them written to a storage provider. If your use case does not require any special processing of event logs then you can simply deploy just `event-log-fetcher` and `event-log-writer` and connect them to the same kafka topic in the data pipeline.

    Using a separate terminal for each component...

    **Start event-log-writer**
    ```shell
    ./event-log-writer -c config.distributed.yaml
    ```

    **Start event-log-processor**
    ```shell
    ./event-log-processor -c config.distributed.yaml
    ```

    **Start event-log-fetcher**
    ```shell
    ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-fetcher -c config.distributed.yaml
    ```

## Using `Docker-Compose`

Check out [Running Each Event Log Collector Component in Docker-Compose](event-log-components-docker-compose.md) for details on running event-log-collector components via `docker-compose`

