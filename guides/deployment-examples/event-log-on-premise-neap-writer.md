# Writing On-Premise Event Logs to NEAP AFO/AFT

For on-premise deployments of Mix Dialog, event log collector provides a neap log writer that supports generating NII event log archives that can then be processed by Nuance R&D's `kafak2session` python script for injestion of dialog sessions into NEAP. This offers a solution for delivering NEAP AFO/AFT reports.

**Pre-Requisites**

* An on-premise deployment of Mix Dialog configured to write event logs to a Kafka topic

**Event Log Collector Components**

Because event logs are being written directly to Kafka we can leverage the built-in Kafka data pipeline that comes with `event-log-collector` and all we need to deploy is the `event-log-writer`. We'll deploy an instance of `event-log-writer` that writes to the `neap` storage provider.
 
**Instructions**
1. Gather the connection details to your on-premise kafka deployment *(for this demo we'll use a local instance of [kafka](../deployment-guides/dependencies/pipelines/kafka.md) containing event logs already fetched from our other deployment examples)*
   
   > **Caution!** This can create an infinite loop of fetching and writing event logs Mix hosted run-time if you have `event-log-fetcher` running from one of the other deployment examples.

2. Create the `event-log-writer` configuration file

    Create the following `config.neap.yaml` file

    **config.neap.yaml**
    ```yaml
    writer:
        name: neap
        # config:
            ### Required Parameters

            # none

            ### Optional Parameters

            # max_records: 1000 # Max Records specifies how many event logs shall be written to the file archive
            # compression: gzip # Specifies whether to create archives using gzip or zip. The python script from Nuance R&D expects gzip

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
    
    **Start event-log-writer**
    ```shell
    ./event-log-writer -c config.neap.yaml
    ```
