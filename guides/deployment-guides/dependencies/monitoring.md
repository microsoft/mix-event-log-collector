# Grafana + Prometheus


## Local Deployment

Please refer to the online installation guides provided in the links below

* https://grafana.com/docs/grafana/latest/
* https://prometheus.io/docs/introduction/first_steps/
* https://prometheus.io/docs/prometheus/latest/installation/
* https://prometheus.io/download/

## Docker-Compose

`docker-compose.monitoring.yaml`

```
version: '3'

services:

  prometheus:
    image: prom/prometheus
    container_name: prometheus
    hostname: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ${PWD}/configs/prometheus/prometheus.yaml:/etc/prometheus/prometheus.yaml
      - monitoring-prometheus-data:/prometheus
    command: --config.file=/etc/prometheus/prometheus.yaml
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

  grafana:
    image: grafana/grafana-oss
    container_name: grafana
    hostname: grafana
    ports:
      - "3000:3000"
    volumes:
      - ${PWD}/configs/grafana/grafana.ini:/etc/grafana/grafana.ini
      - ${PWD}/configs/grafana/provisioning/datasources:/etc/grafana/provisioning/datasources
      - ${PWD}/configs/grafana/provisioning/dashboards:/etc/grafana/provisioning/dashboards
      - ${PWD}/configs/grafana/dashboards:/etc/grafana/dashboards
      - monitoring-grafana-data:/etc/grafana/
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

volumes:
  monitoring-grafana-data:
    driver: local
  monitoring-prometheus-data:
    driver: local

networks:
  event-log-collector:
    driver: bridge
```

**Running docker-compose**
```
docker-compose -f docker-compose.redis.yaml up
```

## Kubernetes

```
helm repo add grafana https://grafana.github.io/helm-charts
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

NAMESPACE=monitoring

LATEST=$(curl -s https://api.github.com/repos/prometheus-operator/prometheus-operator/releases/latest | jq -cr .tag_name)
curl -sL https://github.com/prometheus-operator/prometheus-operator/releases/download/${LATEST}/bundle.yaml | kubectl create -n ${NAMESPACE} -f -

helm upgrade --install --namespace $NAMESPACE --create-namespace \
    grafana grafana/grafana

helm upgrade --install --namespace $NAMESPACE --create-namespace \
    prometheus prometheus-community/prometheus
```