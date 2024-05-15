# kafka + kafka-ui

**Helpful Links**

*Kafka*

* https://kafka.apache.org/quickstart
* https://docs.confluent.io/platform/current/installation/docker/config-reference.html#

*Kafka UI*

* https://github.com/provectus/kafka-ui
* https://github.com/provectus/kafka-ui/blob/master/docker-compose.md

## Local Deployment

[Refer to the Kafka QuickStart](https://kafka.apache.org/quickstart)

## Docker-Compose

`docker-compose.kafka.yaml`

```yaml
version: '3'
services:

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    container_name: zookeeper
    restart: unless-stopped
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    volumes:
      - zookeeper-data:/var/lib/zookeeper/log
      - zookeeper-data:/var/lib/zookeeper/data
      - zookeeper-data:/etc/zookeeper/secrets
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

  kafka:
    image: confluentinc/cp-kafka:latest
    container_name: kafka
    hostname: kafka
    restart: unless-stopped
    ports:
      # To learn about configuring Kafka for access across networks see
      # https://www.confluent.io/blog/kafka-client-cannot-connect-to-broker-on-aws-on-docker-etc/
      - "9092:9092"
    depends_on:
      - zookeeper
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: 'zookeeper:2181'
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: INTERNAL:PLAINTEXT,CLIENT:PLAINTEXT
      KAFKA_ADVERTISED_LISTENERS: INTERNAL://localhost:9093,CLIENT://kafka:9092
      KAFKA_INTER_BROKER_LISTENER_NAME: CLIENT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_ADVERTISED_HOST_NAME: 127.0.0.1
      KAFKA_CFG_ADVERTISED_LISTENERS: INTERNAL://localhost:9093,CLIENT://kafka:9092
      KAFKA_CREATE_TOPICS: "queues.fetcher,queues.processor"
      KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_CFG_DELETE_TOPIC_ENABLE: "true"
      KAFKA_LOG_RETENTION_BYTES: 536870912
      KAFKA_LOG_RETENTION_CHECK_INTERVAL_MS: 60000
      #KAFKA_LOG_RETENTION_MS: 60000 # 1 hr
    volumes:
      - kafka-data:/var/run/docker.sock
      - kafka-data:/var/lib/kafka/data
      - kafka-data:/etc/kafka/secrets
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

  kafka-ui:
    image: provectuslabs/kafka-ui
    container_name: kafka-ui
    depends_on:
      - broker
    ports:
      - "8080:8080"
    restart: unless-stopped
    environment:
      - KAFKA_CLUSTERS_0_NAME=local
      - KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS=kafka:9092
      - KAFKA_CLUSTERS_0_ZOOKEEPER=zookeeper:2181
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

volumes:
  zookeeper-data:
    driver: local
  kafka-data:
    driver: local  

networks:
  event-log-collector:
    driver: bridge
```

**Running docker-compose**

```
docker-compose -f docker-compose.kafka.yaml up
```

## Kubernetes

**kafka**
```
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

NAMESPACE=analytics

helm upgrade --install --namespace $NAMESPACE --create-namespace \
    --set 'provisioning.enabled=true' \
    --set 'deleteTopicEnable=true' \
    --set 'heapOpts=-Xmx4096m -Xms4096m' \
    --set 'logRetentionCheckIntervalMs=60000' \
    --set 'logRetentionHours=1' \
    kafka bitnami/kafka
```

> To create topics and customize their settings during initialization, follow this example...
> 
> ```
> --set 'provisioning.topics.0.name=topic-name'
> --set 'provisioning.topics.0.partitions=2'
> --set 'provisioning.topics.0.replicationFactor=1'
> --set 'provisioning.topics.0.config.max.message.bytes=64000'
> ```

**kafka-ui**
```
helm repo add kafka-ui https://provectus.github.io/kafka-ui
helm repo update

NAMESPACE=analytics

helm upgrade --install --namespace $NAMESPACE --create-namespace \
    --set envs.config.KAFKA_CLUSTERS_0_NAME=local \
    --set envs.config.KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS=kafka:9092 \
    --set envs.config.KAFKA_CLUSTERS_0_ZOOKEEPER=kafka-zookeeper:2181 \
    kafka-ui kafka-ui/kafka-ui
```
