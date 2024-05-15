# Running Event Log Collector with Distributed Cache and Data Pipeline

It can be useful configuring event-log-collector with kafka for the distributed pipeline because event logs remain in the kafka topics and can be re-processed if necessary. This example illustrates using event-log-client to fetch, process, and write event logs for simplicity. Running each event log component separately in distributed fashion is more practical.

1. Deploy [redis](../deployment-guides/dependencies/cache.md#docker-compose)
2. Deploy [kafka](../deployment-guides/dependencies/pipelines/kafka.md#docker-compose)
3. Configure Event Log Processor and Data Pipeline

    **Use the `mix-default` Processor to merge and normalize event logs**
    ```yaml
    processor:
        name: mix-default
        config:
            cache:
                type: redis
                # parameters for redis cache
                addr: localhost:6379 # Default value is redis:6379
                # db: # Default value is 0
                # default_ttl: # Default value is 900
                # password: # Default value is empty
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

    Create the following `config.redis-kafka.yaml` file with the configuration details provided above

    **config.redis-kafka.yaml** <a href="../resources/sample-configs/config.redis-kafka.yaml">download</a>
    ```yaml
    processor:
        name: mix-default
        config:
            cache:
                type: redis
                # parameters for redis cache
                addr: localhost:6379 # Default value is redis:6379
                # db: # Default value is 0
                # default_ttl: # Default value is 900
                # password: # Default value is empty

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

    **Start collecting logs using distributed cache and data pipeline**
    ```shell
    ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client -c config.redis-kafka.yaml
    ```
