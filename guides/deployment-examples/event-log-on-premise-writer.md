# Writing On-Premise Event Logs to Mix Log.api

For on-premise deployments of Mix XaaS engines (ASR, NLU, TTS, Dialog), it may be desirable or required to push event logs from on-premise into the Mix hosted run-time. This allows you to leverage Mix platform features such as:

 * NLU Discover
 * Professional Services-assisted debugging and performance evaluation of your solution
 * Automated feedback loops into R&D for NLU and ASR datapack improvements.

**Pre-Requisites**

* An on-premise deployment of at least one XaaS engine configured to write event logs to a Kafka topic
* A Mix Application with credentials configured in the Mix hosted run-time

**Event Log Collector Components**

Because event logs are being written directly to Kafka we can leverage the built-in Kafka data pipeline that comes with `event-log-collector` and all we need to deploy is the `event-log-writer`. We'll deploy an instance of `event-log-writer` that writes to the `mix-log-producer-api` storage provider.
 
**Instructions**
1. Gather the connection details to your on-premise kafka deployment *(for this demo we'll use a local instance of [kafka](../deployment-guides/dependencies/pipelines/kafka.md) containing event logs already fetched from our other deployment examples)*
   
   > **Caution!** This can create an infinite loop of fetching and writing event logs Mix hosted run-time if you have `event-log-fetcher` running from one of the other deployment examples.

2. Create the `event-log-writer` configuration file

    Create the following `config.mix-log-api-producer.yaml` file

    **config.mix-log-api-producer.yaml**
    ```yaml
    writer:
        name: mix-log-producer-api
        # config:
            ### Required Parameters

            # none

            ### Optional Parameters

            # credentials:
                # client_id:  # Provide a "Mix Client ID" or pass in with ELC_CLIENT_ID env var
                # client_secret: # Provide a "Mix Client Secret" or pass in with ELC_CLIENT_SECRET env var
            # mix_log_producer_api_url: # Default value is https://log.api.nuance.com/producers

            # num_workers: 50   # Num Workers specifies how many event logs can be processed in parallel by log writer
            # rate_limit:       # Rate Limit controls how fast event logs can be written to storage
                # limit: 1000
                # burst: 10
            # max_retries: 10   # Number of attempts to try and write to storage before failing
            # delay: 10         # Amount of time, in seconds, to wait to retry a failed write to storage

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
    
    **Start event-log-writer**
    ```shell
    ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-writer -c config.mix-log-api-producer.yaml
    ```
