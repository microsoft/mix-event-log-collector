# Configuration Guide

A yaml configuration file may be provided to the application. Start with one of the following out-of-box configs:

* config.sample-full.yaml
* config.sample-defaults.yaml

> `config.sample-defaults.yaml` offers a quick approach to getting started with `event-log-client` and local event log collection. It only requires you to provide a Mix client ID and client secret. It will apply default configuration settings and start fetching the latest event logs available. Add in just the configuration settings you want to over-ride to start customizing the behavior of event log client (e.g. fetching earliest available event logs, applying log filters, or changing where and how logs are stored)

> An alternative option to providing a config file is to pass in [env vars](#environment-vars). `ELC_CLIENT_ID` and `ELC_CLIENT_SECRET` are required if not providing a config file.
> 

## Configuration Settings in Detail

The configuration is split into four sections:

1. `fetcher`: event log fetcher parameters
2. `processor`: event log processor parameters
3. `writer`: event log writer parameters
4. `pipeline`: data pipeline parameters

### fetcher

|Parameter|Type|Default|Description|
|---|---|---|---|
|name|string|mix-log-api|The name of the event log fetcher. Currently supported names include: `mix-log-api`|
|[config](#fetcherconfig)|object||configuration parameters|

#### **fetcher.config**

|Parameter|Type|Default|Description|
|---|---|---|---|
|[credentials](#credentials)|object|authentication parameters|
|max_retries|int|-1|The maximum number of retries when making an http request|
|retry_delay|int|5|Retry request interval in seconds|
|record_check_frequency|int|10|Specifies the amount of time, in seconds, between polling for event log records. For apps with low traffic, polling less frequently (e.g. 60s) is recommended. For apps with high traffic, polling more frequently (e.g. 5s) is recommended.|
|client_name|string|neap|Provide the client_name associated with the Mix App ID. If the `client_id` contains a client_name, then that will override any value provided here. This parameter is to support legacy client_id's that did not contain the client_name. Typically this will be either `default` or `neap`. |
|consumer_group_id|string|01|Provide one of 00, 01, 02, 03, or 04. Up to 5 consumer groups can be created. If two clients share the same consumer_group_id, then they will impact each other's reading of the kafka event log stream. If different organizations are fetching log data, it's best to coordinate assignment of consumer group id's to each organization.|
|api_url|string|https://log.api.nuance.com|Nuance kafka Rest API endpoint|
|consumer_options|json object|{"auto.offset.reset": "latest"}|Check out the [docs](https://docs.mix.nuance.com/runtime-event-logs/#consumers-and-groups) for the complete set of available options. The parameter primarily set in the config is `auto.offset.reset` to specify where in the stream to start reading from (`earliest`, `latest`, or `none` (for current) cursor position) e.g ) ```{"auto.offset.reset": "earliest"}```|

##### **credentials**

|Parameter|Type|Default|Description|
|---|---|---|---|
|auth_disabled|string|false|Option to disable authentication with mix|
|token_url|string|https://auth.crt.nuance.com/oauth2/token|Mix authentication url|
|scope|string|log, log.write|`log` is required by log fetcher. `log.write` is required by log writer when writing to `mix-log-producer-api`|
|client_id|string||Provide a valid Mix client_id|
|client_secret|string||Provide a valid Mix client_secret
|auth_timeout_ms|int|5000|configurable timeout ms for mix authentication|

### processor

|Parameter|Type|Default|Description|
|---|---|---|---|
|name|string|mix-default|The name of the event log processor. Currently supported names include: [`mix-default`, `mix-bi`, `noop`]|
|[config](#processorconfig)|object||configuration parameters

>* `mix-default` provides call log normalization and asr call log merging
>* `mix-bi` reduces call logs to a minimal set of metrics used for business reporting
>* `noop` validates and filters logs, but does not apply any transformations or merging of call logs

#### **processor.config**

|Parameter|Type|Default|Description|
|---|---|---|---|
|num_workers|int|1|The number of concurrent workers available to process event logs.|
|[filter](#processorconfigfilter)|object||log filtering parameters|
|[cache](#processorconfigcache)|object||cache parameters|

#### **processor.config.filter**

|Parameter|Type|Default|Description|
|---|---|---|---|
|type|string|blacklist|Supported options are `blacklist` and `whitelist`|
|content_type|array||List of event log types to either whitelist or blacklist|

#### **processor.config.cache**

|Parameter|Type|Default|Description|
|---|---|---|---|
|type|string|local|Supported options are `local` and `redis`|
|expiration|int|3600|[`local`] Duration in seconds before cached items expire.|
|addr|string|redis:6379|[`redis`] Cache hostname and port (e.g. redis:6379)|
|db|int|0|[`redis`] Redis DB to use|
|default_ttl|3600|[`redis`] Duration in seconds before cached item expires|
|password|string|\<empty\>|[`redis`] Cache password|


### writer

|Parameter|Type|Default|Description|
|---|---|---|---|
|name|string|filesystem|The name of the event log writer. Currently supported names include: [`filesystem`, `elasticsearch`, `opensearch`, `mongodb`, `mix-log-producer-api`, `fluentd`, `neap`]|
|[config](#writerconfig)|object||configuration parameters

#### **writer.config**

|Parameter|Type|Default|Description|
|---|---|---|---|
|num_workers|int|5|The number of concurrent workers available to write event logs.|
|path|string|./event-logs|[`filesystem`, `neap`] The directory where event logs should be written to|
|[audio_fetcher](#writerconfigaudio_fetcher)|object||[`filesystem`] Configure fetching of audio. Only available with the filesystem writer.|
|addresses|array|[http://localhost:9200]|[`elasticsearch`, `opensearch`] List of available elasticsearch or opensearch addresses|
|doc_type|string|mix3-record|[`elasticsearch`] The elasticsearch doc_type name|
|index_prefix|string|mix3-logs-v2|[`elasticsearch`, `opensearch`] The elasticsearch or opensearch index prefix|
|append_date_to_index|boolean|true|[`elasticsearch`, `opensearch`] Set to false to have all docs indexed under the same `index_prefix` name|
|refresh|boolean|true|[`elasticsearch`, `opensearch`] Signals to elasticsearch or opensearch to force an index refresh after each write to storage|
|mongodb_uri|string||[`mongodb`] The mongodb uri using [Standard Connection String Format](https://www.mongodb.com/docs/manual/reference/connection-string/#std-label-connections-standard-connection-string-format)|
|database_name|string|event-log-collector|[`mongodb`] The mongodb database|
|collection_name|string|mix-event-logs|[`mongodb`] The mongodb database collection|
|[credentials](#credentials)|object||[`mix-log-producer-api`] authentication parameters|
|mix_log_producer_api_url|string|https://log.api.nuance.com/producers|[`mix-log-producer-api`] mix log producer api endpoint|
|address|string|127.0.0.1:24224|[`fluentd`] The TCP endpoint that fluentd is listening on|
|tag|string|the `Topic` extracted from the event log|[`fluentd`] Specify the fluentd tag that should be used when writing the record to fluentd|
|buffered|boolean|false|[`fluentd`] Enable buffering of event logs as they're written to fluentd. Setting to true causes the fluentd client to write to fluentd asynchronously and not wait for status of the request.|
|buffer_limit|int|8388608|[`fluentd`] The buffer size, in MB, to use when buffered is set to true|
|max_records|int|1000|[`neap`] The number of NII event logs to include in a single archive|
|compression|string|gzip|[`neap`] The compression type to use when archiving neap event logs|
|rate_limit|object|10|[`elasticsearch`, `opensearch`, `mongodb`, `mix-log-producer-api`] Rate limiting applied when writing to storage|
|max_retries|int|10|[`elasticsearch`, `opensearch`, `mongodb`, `mix-log-producer-api`] Max number of retries before failing a write|
|delay|int|10|[`elasticsearch`, `opensearch`, `mongodb`, `mix-log-producer-api`] Delay between each failed write request|
|username|string||[`elasticsearch`, `opensearch`] Storage provider username|
|password|string||[`elasticsearch`, `opensearch`] Storage provider password|
|pipeline|string||[`elasticsearch`, `opensearch`] Provide a valid [ingest pipeline](https://www.elastic.co/guide/en/elasticsearch/reference/current/ingest.html) name|
|retry_on_status|[]int]|[502, 503, 504]|[`elasticsearch`, `opensearch`] List of status codes for retry. Default: 502, 503, 504.|
|disable_retry|boolean|false|[`elasticsearch`, `opensearch`]|
|enable_retry_on_timeout|boolean|false|[`elasticsearch`, `opensearch`]|
|compress_request_body|boolean|false|[`elasticsearch`, `opensearch`]|
|discover_node_on_start|boolean|false|[`elasticsearch`, `opensearch`] Discover nodes when initializing the client. Default: false.|
|enable_metrics|boolean|false|[`elasticsearch`, `opensearch`] Enable the metrics collection.|
|enable_debug_logger|boolean|false|[`elasticsearch`, `opensearch`] Enable the debug logging.|
|user_response_check_only|boolean|false|[`elasticsearch`, `opensearch`]|
|[tls_config](#writerconfigtls_config)|object|false|[`elasticsearch`, `opensearch`]|
|cloud_id|string||[`elasticsearch`] Endpoint for the Elastic Service (https://elastic.co/cloud).|
|api_key|string||[`elasticsearch`] Base64-encoded token for authorization; if set, overrides username/password and service token.|
|service_token|string||[`elasticsearch`] Service token for authorization; if set, overrides username/password.|
|certificate_fingerprint|string||[`elasticsearch`] SHA256 hex fingerprint given by Elasticsearch on first launch.|
|enable_compatibility_mode|boolean|false|[`elasticsearch`] Enable sends compatibility header|
|disable_meta_header|boolean|false|[`elasticsearch`] Disable the additional "X-Elastic-Client-Meta" HTTP header.|

##### **writer.config.audio_fetcher**

|Parameter|Type|Default|Description|
|---|---|---|---|
|enabled|bool|true|Enable/Disable fetching of audio|
|[credentials](#credentials)|object||authentication parameters|
|api_url|string|https://log.api.nuance.com/api/v1/audio|The AFSS api endpoint|

##### **writer.config.tls_config**

|Parameter|Type|Default|Description|
|---|---|---|---|
|server_name|string|||
|insecure_skip_verify|boolean|false||

### pipeline

|Parameter|Type|Default|Description|
|---|---|---|---|
|storage_provider|string|embedded|Supported options include [`embedded`, `kafka`, `redis`, and `pulsar`]|
|fetcher|object||Pipeline parameters for fetcher|
|processor|object||Pipeline parameters for processor|
|writer|object||Pipeline parameters for writer|

> `redis`: Redis pub/sub does not persist events out-of-box like kafka and pulsar. As a result, published events are lost if a subscriber is not already available and listening for published events on a given channel. Therefore, for production deployments, it's recommended to use either kafka or pulsar.

> `embedded`: The embedded pipeline writes to/reads from your local filesystem. It's not suitable for a distributed deployment model of event-log-collector or for production-level traffic loads.

#### **pipeline.fetcher**

|Parameter|Type|Description|
|---|---|---|
|writer|object|Pipeline writer i/o parameters|

#### **pipeline.processor**

|Parameter|Type|Description|
|---|---|---|
|reader|object|Pipeline reader i/o parameters|
|writer|object|Pipeline writer i/o parameters|

#### **pipeline.writer**

|Parameter|Type|Description|
|---|---|---|
|reader|object|Pipeline reader i/o parameters|

#### **pipeline.\<component\>.writer**

|Parameter|Type|Default|Description|
|---|---|---|---|
|path|string|os.TempDir()/fetcher and os.TempDir()/processor|[`emdedded`] The path the pipeline will write to for the persistent FIFO queue|
|host|string||[`pulsar`, `redis`] The host to connect to|
|hosts|array||[`kafka`] The list of hosts to connect to|
|topic|string||[`pulsar`, `kafka`] The topic name to write to|
|password|string||[`redis`] The cache password|
|DB|int||[`redis`] The cache database number|
|channel|string||[`redis`] The cache pub/sub channel to write to|

#### **pipeline.\<component\>.reader**

|Parameter|Type|Default|Description|
|---|---|---|---|
|path|string|os.TempDir()/fetcher and os.TempDir()/processor|[`emdedded`] The path the pipeline will read from for the persistent FIFO queue|
|host|string||[`pulsar`, `redis`]| The host to connect to
|hosts|array||[`kafka`] The list of hosts to connect to|
|create_unique_group_id|boolean|true|[`kafka`] When set to true, a unique id will be appended to the group name ensuring no other client interferes with this clients reading of a kafka topic. Set to false when you want to scale horizontally multiple clients across partitions for a given topic|
|group|string||[`pulsar`, `kafka`] The group name to create when reading from the pipeline|
|topic|string||[`pulsar`, `kafka`] The topic name to read from|
|offset|string||[`pulsar`, `kafka`] The offset to start from [`earliest`, `latest`, `none`]|
|auto_commit_offset_enabled|bool|true|[`kafka`] Enable / Disable auto committing kafka offset|
|password|string||[`redis`] The cache password|
|DB|int||[`redis`] The cache database number|
|channel|string||[`redis`] The cache pub/sub channel to read from|

## Environment Vars

The following environment vars are supported and can be used to over-ride parameters set in the config file.

|Env Var|Description|
|---|---|
|ELC_CLIENT_ID|Applies to any component with a credentials object|
|ELC_CLIENT_SECRET|Applies to any component with a credentials object|
|ELC_TOKEN_URL|Applies to log `fetcher` when set to use `mix-log-api`|
|ELC_API_URL|Applies to log `fetcher` when set to use `mix-log-api`|
|ELC_CONSUMER_GROUP_ID|Applies to log `fetcher` when set to use `mix-log-api`|
|ELC_AFSS_URL|Applies to log `writer` when set to use `filesystem`|
|ELC_MONGODB_URI|Applies to log `writer` when set to use `mongodb`|
|ELC_MONGODB_DATABASE_NAME|Applies to log `writer` when set to use `mongodb`|
|ELC_MONGODB_COLLECTION_NAME|Applies to log `writer` when set to use `mongodb`|
|ELC_OPENSEARCH_USERNAME|Applies to log `writer` when set to use `opensearch`|
|ELC_OPENSEARCH_PASSWORD|Applies to log `writer` when set to use `opensearch`|
|ELC_ELASTICSEARCH_USERNAME|Applies to log `writer` when set to use `elasticsearch`|
|ELC_ELASTICSEARCH_PASSWORD|Applies to log `writer` when set to use `elasticsearch`|
|ELC_ELASTICSEARCH_API_KEY|Applies to log `writer` when set to use `elasticsearch`|
|ELC_ELASTICSEARCH_CLOUD_ID|Applies to log `writer` when set to use `elasticsearch`|
|ELC_ELASTICSEARCH_CERT_FINGERPRINT|Applies to log `writer` when set to use `elasticsearch`|
|ELC_ELASTICSEARCH_SERVICE_TOKEN|Applies to log `writer` when set to use `elasticsearch`|

## Example Config (Minimum)

```yaml
fetcher:
    config:
        credentials:
            client_id: "Provide a Mix Client ID"
            client_secret: "Provide a Mix Client Secret"
```
## Example Config (Full)

```yaml
fetcher:
    name: mix-log-api
    config:
        credentials:
            auth_disabled: false
            token_url: "https://auth.crt.nuance.com/oauth2/token"
            scope: log
            client_id: "Provide a Mix Client ID"
            client_secret: Provide a Mix Client Secret
            auth_timeout_ms: 5000
        max_retries: -1
        retry_delay: 5
        record_check_frequency: 3
        client_name: neap
        consumer_group_id: "01" # values of 01 - 04 are supported
        api_url: "https://log.api.nuance.com"
        consumer_options:
            auto.offset.reset: latest # specify earliest, latest, or none

processor:
    name: mix-default
    config:
        num_workers: 1
        filter:
            type: blacklist # blacklist or whitelist
            content_type: [] # supports trailing wildcard. e.g. application/x-nuance-asr-recognitioninitmessage*
        cache:
            type: local # specify local or redis
            # parameters for local cache
            expiration: 2880
            # parameters for redis cache
            addr: localhost:6379
            db: 0
            default_ttl: 900
            password: 

writer:
    name: filesystem # available options are filesystem, elasticsearch, opensearch, mongodb, fluentd, neap and mix-log-producer-api
    config:
        # all
        num_workers: 5
        
        # filesystem + neap parameters
        path: 'event-logs/my-app-name/' # provide a path for event logs to be written out to
        
        # audio_fetcher params available with the filesystem writer
        audio_fetcher:
            enabled: false # default is true
            credentials:
                auth_disabled: false
                token_url: https://auth.crt.nuance.com/oauth2/token
                scope: log
                client_id: "Provide a Mix Client ID"
                client_secret: "Provide a Mix Client Secret"
                auth_timeout_ms: 5000
            api_url: https://log.api.nuance.com/api/v1/audio

        # neap
        # max_records: 1000 # default is 1000 records per archive
        # compression: gzip # gzip or zip. default is gzip

        # fluentd
        # address: 127.0.0.1:24224 # default value is localhost and fluentd's default port
        # tag: Optional. By default the event log Topic will be used
        # buffered: true # default value is false. Setting buffered to true masks write failures
        # buffer_limit: # 8MB default. Specify integer value. e.g. 8388608 == 8 * 1024 * 1024

        # elasticsearch + opensearch
        doc_type: mix3-record
        index_prefix: mix3-logs-v2
        # append_date_to_index: false # default value is true
        refresh: true
        addresses:
            - http://localhost:9200
        # username: # optional
        # password: # optional
        # pipeline: # optional. Provide a valid ingest pipeline name. e.g. mix-event-log-pipeline
        # retry_on_status: [502, 503, 504] # optional
        # disable_retry: false # optional
        # enable_retry_on_timeout: false # optional
        # compress_request_body: false # optional
        # discover_node_on_start: false # optional
        # enable_metrics: false # optional
        # enable_debug_logger: false # optional
        # user_response_check_only: false # optional
        # tls_config: # optional
            # insecure_skip_verify: false # optional
            # server_name: # optional

        # additional elasticsearch only config options
        # cloud_id: # optional
        # api_key: # optional
        # service_token: # optional
        # certificate_fingerprint: # optional
        # enable_compatibility_mode: false # optional
        # disable_meta_header: false # optional

        # mongodb
        mongodb_uri: # Required. e.g. Azure Cosmos DB uri - "mongodb://<COSMOSDB_ACCOUNT_NAME>:<COSMOSDB_PASSWORD>@<COSMOSDB_ACCOUNT_NAME>.documents.azure.com:10255/?ssl=true&replicaSet=globaldb&maxIdleTimeMS=120000&appName=@<COSMOSDB_ACCOUNT_NAME>@"
        # database_name: # default is event-log-collector
        # collection_name: # default is mix-event-logs

        # mix-log-producer-api
        credentials:
            auth_disabled: false
            token_url: https://auth.crt.nuance.com/oauth2/token
            scope: log.write
            client_id: "Provide a Mix Client ID"
            client_secret: "Provide a Mix Client Secret"
            auth_timeout_ms: 5000
        mix_log_producer_api_url: https://log.api.nuance.com/producers

        # elasticsearch + opensearch + mongodb + neap + mix-log-producer-api
        rate_limit: 
            limit: 10
            burst: 1
        
        # elasticsearch + mongodb + mix-log-producer-api
        max_retries: 10
        delay: 10

pipeline:
    # data pipelining configuration requires readers and/or writers for each component (fetcher, processor, writer)
    #
    # fetcher (writer) ---> (reader) processor (writer) ---> (reader) writer
    #
    # embedded uses a local FIFO queue backed by local filesystem. This is a good option when running event-log-client in stand-alone mode
    # for low event log traffic volumes (e.g. dev + debugging)
    # 
    # kafka is recommended when running components in distributed mode (e.g. each component deployed as it's own service in k8s) for scaling and
    # message persistence/durability
    storage_provider: embedded # available options are embedded, kafka, redis, and pulsar. 
    fetcher:
        writer:
            # embedded
            path: queues/my-app-name/fetcher # specify the name of your app you're collecting event logs for

            # pulsar + redis
            #host: 127.0.0.1:6650 # pulsar
            host: 127.0.0.1:6379 # redis

            # kafka
            hosts:
                - 127.0.0.1:9092
            
            # pulsar
            # operation_timeout: 30

            # pulsar + kafka
            topic: queues.fetcher

            # redis
            password: ""
            DB: 0
            channel: queues:fetcher

    processor:
        reader:
            # embedded
            path: queues/my-app-name/fetcher # specify the name of your app you're collecting event logs for

            # pulsar + redis
            #host: 127.0.0.1:6650 # pulsar
            host: 127.0.0.1:6379 # redis
            
            # kafka
            hosts:
                - 127.0.0.1:9092
            # create_unique_group_id: false # default is true

            # pulsar
            # operation_timeout: 30

            # pulsar + kafka
            group: elc
            topic: queues.fetcher
            offset: earliest
            # auto_commit_offset_enabled: false # default is true

            # redis
            password: ""
            DB: 0
            channel: queues:fetcher
        writer:
            # embedded
            path: queues/my-app-name/processor # specify the name of your app you're collecting event logs for
            
            # pulsar + redis
            #host: 127.0.0.1:6650 # pulsar
            host: 127.0.0.1:6379 # redis

            # kafka
            hosts:
                - 127.0.0.1:9092

            # pulsar
            # operation_timeout: 30

            # pulsar + kafka
            topic: queues.processor
    
            # redis
            password: ""
            DB: 0
            channel: queues:processor
    
    writer:
        reader:
            # embedded
            path: queues/my-app-name/processor # specify the name of your app you're collecting event logs for

            # pulsar + redis
            #host: 127.0.0.1:6650 # pulsar
            host: 127.0.0.1:6379 # redis

            # kafka
            hosts:
                - 127.0.0.1:9092
            # create_unique_group_id: false # default is true

            # pulsar
            # operation_timeout: 30
            
            # pulsar + kafka
            group: elc
            topic: queues.processor
            offset: earliest
            # auto_commit_offset_enabled: false # default is true

            # redis
            password: ""
            DB: 0
            channel: queues:processor
```