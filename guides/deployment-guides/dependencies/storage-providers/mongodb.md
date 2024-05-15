# MongoDB

**Helpful Links**

* https://www.mongodb.com/try/download/community
* https://github.com/mongodb/mongo
* https://www.mongodb.com/compatibility/docker
* https://www.howtogeek.com/devops/how-to-run-mongodb-in-a-docker-container/

## Docker-Compose

`docker-compose.mongodb.yaml`

```yaml
version: '3'

services:
  mongodb:
    image: mongodb/mongodb-community-server:latest
    container_name: mongodb
    hostname: mongodb
    ports:
      - 27017:27017
    volumes:
      - mongodb-data:/data/db
      - mongodb-data:/data/configdb
    security_opt:
      - no-new-privileges:true
    networks:
      - event-log-collector

volumes:
  mongodb-data:
    driver: local

networks:
  event-log-collector:
    driver: bridge
```

**Running docker-compose**
```
docker-compose -f docker-compose.mongodb.yaml up
```

## Azure Cosmos DB for MongoDB

[Refer to Azure Docs for Setting up a Managed instance of MongoDB](https://learn.microsoft.com/en-us/azure/cosmos-db/mongodb/quickstart-go#setup-azure-cosmos-db)

## MongoDB Clients

### 1. MongoDB Compass

* https://www.mongodb.com/products/compass
* https://www.mongodb.com/docs/compass/current/install/

  **macOS**
  ```shell
  brew install --cask mongodb-compass
  ```

### 3. Metabase

* https://github.com/metabase/metabase
* https://www.metabase.com/docs/latest/installation-and-operation/start