# Redis

**Helpful Links**

* https://redis.io/docs/getting-started/

## Local Deployment

* [Linux](https://redis.io/docs/getting-started/installation/install-redis-on-linux/)

* [Windows](https://redis.io/docs/getting-started/installation/install-redis-on-windows/)

* [macOS](https://redis.io/docs/getting-started/installation/install-redis-on-mac-os/)

## Docker-Compose

`docker-compose.redis.yaml`

```
version: '3'

services:
  redis:
    image: redis:alpine
    container_name: redis
    hostname: redis
    ports:
      - 6379:6379
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

volumes:
  redis-data:
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
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

NAMESPACE=analytics

# If using a randomly generated password...
#helm upgrade --install --namespace $NAMESPACE --create-namespace redis bitnami/redis

#export REDIS_PASSWORD=$(kubectl get secret --namespace $NAMESPACE redis -o jsonpath="{.data.redis-password}" | base64 --decode)
#echo $REDIS_PASSWORD

# Otherwise, use the following and provide a password...
helm upgrade --install --namespace $NAMESPACE --create-namespace --set global.redis.password=RedisIsGreat redis bitnami/redis
```
