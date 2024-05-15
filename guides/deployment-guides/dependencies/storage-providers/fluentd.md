# fluentd

**Helpful Links**

* https://docs.fluentd.org/quickstart
* https://github.com/fluent/helm-charts

## Docker-Compose

`docker-compose.fluentd.yaml`

```yaml
version: "3"

services:
  fluentd:
    image: fluent/fluentd:v1.15-1
    container_name: fluentd
    hostname: fluentd
    volumes:
      - ${PWD}/configs/fluentd/fluent.conf:/fluentd/etc/fluent.conf
      - fluentd-data:/fluentd
    ports:
      - "24224:24224"
      - "24224:24224/udp"
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

volumes:
  fluentd-data:
    driver: local

networks:
  event-log-collector:
    driver: bridge
```

**Running docker-compose**
```
docker-compose -f docker-compose.fluentd.yaml up
```

> The `docker-compose.fluend.yaml` refers to a `fluent.conf` volume mapping. The content of `fluent.conf` is:

```
<source>
  @type forward
  port 24224
  bind 0.0.0.0
</source>

<match **>
  @type stdout
</match>
```