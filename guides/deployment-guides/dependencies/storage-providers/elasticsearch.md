# ElasticSearch

## Docker-Compose

`docker-compose.elasticsearch.yaml`

```yaml
version: '3.7'

services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:7.4.0
    container_name: elasticsearch
    hostname: elasticsearch
    environment:
      - xpack.security.enabled=false
      - discovery.type=single-node
      - "ELASTICSEARCH_JAVA_OPTS=-Xms512m -Xmx512m" # minimum and maximum Java heap size, recommend setting both to 50% of system RAM
    ulimits:
      memlock:
        soft: -1
        hard: -1
      nofile:
        soft: 65536
        hard: 65536
    cap_add:
      - IPC_LOCK
    ports:
      - 9200:9200
      - 9300:9300
    volumes:
      - elasticsearch-data:/usr/share/elasticsearch/data
    networks:
      - event-log-collector
    security_opt:
      - no-new-privileges:true

  kibana:
    image: docker.elastic.co/kibana/kibana:7.4.0
    container_name: kibana
    environment:
      - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
    ports:
      - 5601:5601
    depends_on:
      - elasticsearch
    networks:
      - event-log-collector
    security_opt:
      - no-new-privileges:true

volumes:    
  elasticsearch-data:
    driver: local
  elasticsearch-dashboards:
    driver: local
    
networks:
  event-log-collector:
    driver: bridge
```


**Running docker-compose**
```shell
docker-compose -f docker-compose.elasticsearch.yaml up
```