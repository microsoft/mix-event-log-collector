# Quickstart Guide

## Start Collecting Mix Event Logs in Less Than 5 Minutes

**Pre-requisites**
* Mix client credentials for authenticating with Mix log.api
* Enabling log collection for your Mix App ID (Nuance PS can assist you with this)

**Instructions**
1. Download and unzip the binary for your target platform from [here](https://white-mud-08c17b10f.1.azurestaticapps.net/)
2. Run `event-log-client` with your Mix Client ID and Secret

    > Note: Your OS may prompt you to allow running the downloaded app

    ```shell
    ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client
    ```

    **Example Output**
    ```
      _   _
    | \ | |  _   _    __ _   _ __     ___    ___
    |  \| | | | | |  / _` | | '_ \   / __|  / _ \
    | |\  | | |_| | | (_| | | | | | | (__  |  __/
    |_| \_|  \__,_|  \__,_| |_| |_|  \___|  \___|
      _____                          _       _                          ____           _   _                 _
    | ____| __   __   ___   _ __   | |_    | |       ___     __ _     / ___|   ___   | | | |   ___    ___  | |_    ___    _ __
    |  _|   \ \ / /  / _ \ | '_ \  | __|   | |      / _ \   / _` |   | |      / _ \  | | | |  / _ \  / __| | __|  / _ \  | '__|
    | |___   \ V /  |  __/ | | | | | |_    | |___  | (_) | | (_| |   | |___  | (_) | | | | | |  __/ | (__  | |_  | (_) | | |
    |_____|   \_/    \___| |_| |_|  \__|   |_____|  \___/   \__, |    \____|  \___/  |_| |_|  \___|  \___|  \__|  \___/  |_|
                                                            |___/
              ____        ____         ___
    __   __ |___ \      |___ \       / _ \
    \ \ / /   __) |       __) |     | | | |
      \ V /   / __/   _   / __/   _  | |_| |
      \_/   |_____| (_) |_____| (_)  \___/

    2023-03-03T14:32:22.015-0500    info    processor/processor.go:287      starting noop processer
    2023-03-03T14:32:22.015-0500    info    mix_log_api/fetcher.go:609      starting mix-log-api log fetcher
    2023-03-03T14:32:22.016-0500    info    filesystem/writer.go:404        Starting filesystem writer. Writing logs to event-logs
    2023-03-03T14:32:23.261-0500    info    mix_log_api/fetcher.go:242      Partition [0] offset details: {"start": 91854, "end": 91857, "current": 91857, "total": 3, "behind": 0}
    2023-03-03T14:32:23.722-0500    info    mix_log_api/fetcher.go:242      Partition [1] offset details: {"start": 85649, "end": 85659, "current": 85659, "total": 10, "behind": 0}
    ```

    > **Default Behavior:**
    > 
    > * Event Log Collector authenticates with and connects to the **Mix US** (`log.api.nuance.com`) geography. You can use either [env vars](configuration-guide.md#environment-vars) or a config file to collect event log data from other Mix geographies.
    > * Event logs are written to file under a folder named `./event-logs`. Logs are partitioned first by date and an attempt made to group logs by dialog session id (if dialog is involved) and ASR session id (if asr is involved)
    > * Audio is also fetched and saved as a wav file for ASR requests
    > * The log offset is set to the latest available log for each partition (you will likely only have one partition). This means that only new event logs created after you start event log collector will be fetched. If you want to collect existing log data, this can be done using a [configuration](configuration-guide.md#fetcherconfig) file.

### Connecting to Another Geography

> Set the following env vars:
>
> * ELC_TOKEN_URL
> * ELC_API_URL
> * ELC_AFSS_URL

**Example: EU Geography**
```shell
ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ELC_TOKEN_URL=https://auth.crt.nuance.eu/oauth2/token ELC_API_URL=https://log.api.nuance.eu ELC_AFSS_URL=https://log.api.nuance.eu/api/v1/audio ./event-log-client
```

### Fetch earliest available logs

Create the following `config.earliest.yaml` file

**config.earliest.yaml**
```yaml
fetcher:
    config:
        consumer_options:
            auto.offset.reset: earliest
```

**Start `event-log-collector` with the config file**
```shell
ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client -c config.earliest.yaml
```
### `/event-logs` folder structure explained
```shell
event-logs/
└── 02-01-2023
    ├── asr
    │   ├── 3562d9ff-ff89-46a6-9373-84777b0c24da <<--- THIS ASR SESSION ID CONTAINS RECO INIT, PARTIAL RESULTS, FINAL STATUS, ETC. FIND THE ASR CALL SUMMARY AND AUDIO UNDER THE DIALOG SESSION ID
    │   │   ├── 055729a3-ff52-4c8f-be7c-d7841fa598e7.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80829.json
    │   │   ├── <lines redacted>
    │   │   └── fb85d63a-cb60-4bcd-9521-3f59e747dac1.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.0.87842.json
    │   └── fcda13af-7cca-42a1-b8ba-675754be75f6
    │       ├── 067f7d47-2d08-47d6-92e3-33014a687f39.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80851.json
    │       ├── <lines redacted>
    │       └── ebc2701d-b6b8-47b6-88d9-f49ad89c624c.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80850.json
    ├── d1853ebb-c350-4f0e-a40d-85b9278f9bdd <<--- THIS IS A DIALOG SESSION ID. ASR, DLG, NII, NLU, AND TTS RECORDS ASSOCIATED WITH THIS SESSION ARE BROKEN OUT BELOW
    │   ├── asr
    │   │   ├── 3562d9ff-ff89-46a6-9373-84777b0c24da <<--- THIS ASR SESSION ID MAPS TO THE ONE ABOVE UNDER /asr. THIS FOLDER CONTAINS THE ASR CALL SUMMARY AND AUDIO
    │   │   │   ├── audio_0.wav
    │   │   │   └── f1f28541-e88c-42f2-8b84-fc1ad13cfca6.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80836.json
    │   │   └── fcda13af-7cca-42a1-b8ba-675754be75f6
    │   │       ├── audio_0.wav
    │   │       └── f9830b50-49f5-47bb-9b8b-d7df2a613ee2.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.0.87851.json
    │   ├── dlg
    │   │   ├── 0f8900cf-7a10-45ab-9145-466a3f43e857.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80819.json
    │   │   ├── <lines redacted>
    │   │   └── fc737dc8-4285-46ed-8996-c01ed0149105.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80816.json
    │   ├── nii
    │   │   ├── 084e3c72-f7b3-459d-859f-2f86979faba4.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80825.json
    │   │   ├── <lines redacted>
    │   │   └── fa5db1de-0b4e-45a4-ad2e-8534b1816a36.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.0.87836.json
    │   └── nlu
    │       ├── 399dfe5a-d6ca-4eec-acf9-f515e1219916.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80853.json
    │       └── f31c71bd-a58d-4944-868d-e92de0e60ed9.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.1.80852.json
    └── dlg <<--- THIS FOLDER CONTAINS DIALOG-START-START EVENT LOGS
        └── a281e1e0-136c-43f0-a4f2-9bc86100559c.NMDPTRIAL_peter_freshman_nuance_com_20191015T195942962874.0.87829.json
```


## Sending Event Logs to Other Storage Providers

The out-of-box defaults are intended to make it quick and easy to get going with event log collection in your personal dev environment. But making sense of the raw event logs can be challenging, especially if you don't know what you're looking for.

That's why the `event-log-writer` component comes with some out-of-box storage providers to make it easier to query, visualize, and analyze the data. Refer to the [storage provider dependencies](deployment-guides/dependencies/storage-providers.md) guides for details on how to deploy each one locally.

### [MongoDB](deployment-guides/dependencies/storage-providers/mongodb.md)

Create the following `config.mongodb.yaml` file

**config.mongodb.yaml**
```yaml
writer:
    name: mongodb
    # config:  # Uncomment if you need to modify any default parameters
        ### Required Parameters

        # none

        ### Optional Parameters
        
        # The following uri assumes mongodb is running locally without any security
        # mongodb_uri: Default value is mongodb://localhost:27017/?directConnection=true
        # database_name: # Default value is event-log-collector
        # collection_name: # Default value is mix-event-logs

        # num_workers: 50   # Num Workers specifies how many event logs can be processed in parallel by log writer
        # rate_limit:       # Rate Limit controls how fast event logs can be written to storage
            # limit: 1000
            # burst: 10
```

**Start collecting and writing logs to MongoDB**
```shell
ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client -c config.mongodb.yaml
```

### [OpenSearch](deployment-guides/dependencies/storage-providers/opensearch.md)

Create the following `config.opensearch.yaml` file

**config.opensearch.yaml**
```yaml
writer:
    name: opensearch    
    # config:  # Uncomment if you need to modify any default parameters
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

**Start collecting and writing logs to OpenSearch**
```shell
ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client -c config.opensearch.yaml
```

### [ElasticSearch](deployment-guides/dependencies/storage-providers/elasticsearch.md)

Create the following `config.elasticsearch.yaml` file

**config.elasticsearch.yaml**
```yaml
writer:
    name: elasticsearch
    # config:  # Uncomment if you need to modify any default parameters
        ### Required Parameters

        # none

        ### Optional Parameters
        
        # doc_type: # Default value is mix3-record
        # index_prefix: # Default value is mix3-logs-v2
        # refresh: # Default value is true
        # addresses:
            # - http://localhost:9200   # this is the default

        # num_workers: 50   # Num Workers specifies how many event logs can be processed in parallel by log writer
        # rate_limit:       # Rate Limit controls how fast event logs can be written to storage
            # limit: 1000
            # burst: 10
```

**Start collecting and writing logs to ElasticSearch**
```shell
ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client -c config.elasticsearch.yaml
```

### [Fluentd](deployment-guides/dependencies/storage-providers/fluentd.md)

Create the following `config.fluentd.yaml` file

**config.fluentd.yaml**
```yaml
writer:
    name: fluentd
    # config:  # Uncomment if you need to modify any default parameters
        # buffered: # Default value is false
        # address: # Default value is 127.0.0.1:24224
        # buffer_limit: # 8MB default. Specify integer value. e.g. 8388608 == 8 * 1024 * 1024
```

**Start collecting and writing logs to fluentd**
```shell
ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client -c config.fluentd.yaml
```

## Additional Configuration and Deployment Options

> Please review the following two guides to learn more on the various ways `event-log-collector` and its `data pipeline` can be configured and deployed
>  * [Configuration Guide](./configuration-guide.md)
>  * [Deployment Guide](./deployment-guide.md)

## Building Event Log Collector From Source

### Pre-requisites

Building `mix-event-log-collector` requires the following:
* [golang](https://go.dev/learn/) 1.18+ (currently built with v1.19.6) for building the binaries
* bash shell (if using the scripts provided)
    
    * [installing bash on windows](https://www.howtogeek.com/790062/how-to-install-bash-on-windows-11/)
    * [installing cygwin](https://cygwin.com/install.html)

### Building `mix-event-log-collector`

To build `mix-event-log-collector`, run `make build` or run the script directly.

```
./scripts/setup.sh
````
This script will intialize golang mod vendoring and build the following:
* `event-log-fetcher`: The application that consumes logs from mix log.api
* `event-log-processor`: The application that validates, filters, and transforms event logs
* `event-log-writer`: The application that writes event logs to a storage provider
* `event-log-client`: A client that wraps all three of the above into a single stand-alone application.

## Running each component

**`event-log-fetcher`**

```
./bin/event-log-fetcher
```

**`event-log-processor`**
```
./bin/event-log-processor
```

**`event-log-writer`**
```
./bin/event-log-writer
```

**stand-alone `event-log-client`**
```
./bin/event-log-client
```

### Usage Details

```shell
./bin/event-log-client -h
```

```
  _   _
 | \ | |  _   _    __ _   _ __     ___    ___
 |  \| | | | | |  / _` | | '_ \   / __|  / _ \
 | |\  | | |_| | | (_| | | | | | | (__  |  __/
 |_| \_|  \__,_|  \__,_| |_| |_|  \___|  \___|
  _____                          _       _                          ____           _   _                 _
 | ____| __   __   ___   _ __   | |_    | |       ___     __ _     / ___|   ___   | | | |   ___    ___  | |_    ___    _ __
 |  _|   \ \ / /  / _ \ | '_ \  | __|   | |      / _ \   / _` |   | |      / _ \  | | | |  / _ \  / __| | __|  / _ \  | '__|
 | |___   \ V /  |  __/ | | | | | |_    | |___  | (_) | | (_| |   | |___  | (_) | | | | | |  __/ | (__  | |_  | (_) | | |
 |_____|   \_/    \___| |_| |_|  \__|   |_____|  \___/   \__, |    \____|  \___/  |_| |_|  \___|  \___|  \__|  \___/  |_|
                                                         |___/
          ____        _  _          ___
 __   __ |___ \      | || |        / _ \
 \ \ / /   __) |     | || |_      | | | |
  \ V /   / __/   _  |__   _|  _  | |_| |
   \_/   |_____| (_)    |_|   (_)  \___/

Usage: event-log-client [--logToFile] [--logfilePath LOGFILEPATH] [--debug] [--config CONFIG] [--port PORT] [--addr ADDR] [--scheme SCHEME] [--serverReadTimeout SERVERREADTIMEOUT] [--serverWriteTimeout SERVERWRITETIMEOUT] [--basePath BASEPATH]

Options:
  --logToFile, -l        Enable logging to file [default: false, env: ELC_ENABLE_LOGGING_TO_FILE]
  --logfilePath LOGFILEPATH, -f LOGFILEPATH
                         Location and name of file to log to [default: logs/event-log-collector.log, env: ELC_LOGFILE_PATH]
  --debug, -d            Specify this flag to enable debug logging level [env: ELC_ENABLE_DEBUG_LOGGING]
  --config CONFIG, -c CONFIG
                         Path to event logger configs. Can be either a directory or specific json config file [default: configs/, env: ELC_CONFIG]
  --port PORT, -p PORT   Port for the service to listen on [default: 8078, env: ELC_PORT]
  --addr ADDR, -a ADDR   Address of the service [default: localhost, env: ELC_ADDRESS]
  --scheme SCHEME, -s SCHEME
                         Scheme of the service (http or https [default: http, env: ELC_SCHEME]
  --serverReadTimeout SERVERREADTIMEOUT
                         Server read timeout in seconds [default: 60, env: ELC_SERVER_READ_TIMEOUT]
  --serverWriteTimeout SERVERWRITETIMEOUT
                         Server write timeout in seconds [default: 60, env: ELC_SERVER_WRITE_TIMEOUT]
  --basePath BASEPATH    Base path to prefix api routes. Use this when deployed behind a reverse proxy [env: ELC_BASE_PATH]
  --help, -h             display this help and exit
```
