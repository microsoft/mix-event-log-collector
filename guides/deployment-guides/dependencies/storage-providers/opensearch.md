# OpenSearch

## Docker-Compose

`docker-compose.opensearch.yaml`

```yaml
version: '3'

services:
  opensearch:
    image: opensearchproject/opensearch:2.6.0
    container_name: opensearch
    hostname: opensearch
    environment:
      - plugins.security.disabled=true
      - discovery.type=single-node
      - bootstrap.memory_lock=true # along with the memlock settings below, disables swapping
      - "OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m" # minimum and maximum Java heap size, recommend setting both to 50% of system RAM
      #- network.host=0.0.0.0 # required if not using the demo security configuration
    ulimits:
      memlock:
        soft: -1
        hard: -1
      nofile:
        soft: 65536 # maximum number of open files for the OpenSearch user, set to at least 65536 on modern systems
        hard: 65536
    ports:
      - 9200:9200
      - 9600:9600 # required for Performance Analyzer
    volumes:
      - opensearch-data:/usr/share/opensearch/data
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector
  
  opensearch-dashboards:
    image: opensearchproject/opensearch-dashboards:2.6.0
    container_name: opensearch-dashboards
    ports:
      - 5601:5601
    expose:
      - "5601"
    environment:
      OPENSEARCH_HOSTS: '["http://opensearch:9200"]' # must be a string with no spaces when specified as an environment variable
      DISABLE_SECURITY_DASHBOARDS_PLUGIN: "true"
    volumes:
      - opensearch-dashboards:/usr/share/opensearch-dashboards/config/
    #  - ./custom-opensearch_dashboards.yml:/usr/share/opensearch-dashboards/config/opensearch_dashboards.yml
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

volumes:    
  opensearch-data:
    driver: local
  opensearch-dashboards:
    driver: local
    
networks:
  event-log-collector:
    driver: bridge
```

**Running docker-compose**
```shell
docker-compose -f docker-compose.opensearch.yaml up
```