# Running Multiple Instances of Event Log Writer

This example builds upon [Running Each Event Log Collector Component Locally](event-log-components-distributed.md). We'll start with the configuration used in that example and extend it by:

 1. Create multiple config files, one for each instance of `event-log-writer`
 2. Write logs to console via fluentd
 3. Write logs to MongoDB for querying and analytics
 
**Instructions**
1. Deploy [redis](../deployment-guides/dependencies/cache.md#docker-compose)
2. Deploy [kafka](../deployment-guides/dependencies/pipelines/kafka.md#docker-compose)
3. Deploy [fluentd](../deployment-guides/dependencies/storage-providers/fluentd.md#docker-compose)
4. Deploy [MongoDB](../deployment-guides/dependencies/storage-providers/mongodb.md#docker-compose)
5. Create two Event Log `Writer` configuration files`

    Create the following `config.fluentd-writer.yaml` file

    **config.fluentd-writer.yaml**
    ```yaml
    writer:
        name: fluentd
        # config:
            # buffered: true
            # address: 127.0.0.1:24224
            # buffer_limit: # 8MB default. Specify integer value. e.g. 8388608 == 8 * 1024 * 1024

    pipeline:
        storage_provider: kafka
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

    Create the following `config.mongodb-writer.yaml` file

    **config.mongodb-writer.yaml**
    ```yaml
    writer:
        name: mongodb
        config:
            ### Required Parameters

            # The following uri assumes mongodb is running locally without any security
            mongodb_uri: "mongodb://localhost:27017/?directConnection=true"

            ### Optional Parameters
            
            # database_name: # default is event-log-collector
            # collection_name: # default is mix-event-logs

            # num_workers: 50   # Num Workers specifies how many event logs can be processed in parallel by log writer
            # rate_limit:       # Rate Limit controls how fast event logs can be written to storage
                # limit: 1000
                # burst: 10

    pipeline:
        storage_provider: kafka
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

    > Refer to [Running Each Event Log Collector Component Locally](event-log-components-distributed.md#) for details on running `event-log-fetcher` and `event-log-processor`
    
    Using a separate terminal for each instance of `event-log-writer`...

    **Start event-log-writer (fluentd)**
    ```shell
    ./event-log-writer -c config.fluentd-writer.yaml
    ```

    **Start event-log-writer (mongodb)**
    ```shell
    ./event-log-writer -c config.mongodb-writer.yaml
    ```
