package cache

import (
	"context"
	"encoding/json"
	"time"

	redis "github.com/go-redis/redis/v8"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type RedisCache struct {
	cacheConfig CacheConfig
	Client      *redis.Client
}

func NewRedisCache(config CacheConfig) Cache {
	config.DefaultTTL = config.DefaultTTL * time.Second
	log.Debugf("cache config: %+v", config)

	c := &RedisCache{
		cacheConfig: config,
		Client: redis.NewClient(&redis.Options{
			Addr:               config.Addr,
			Password:           config.Password, // no password set
			DB:                 config.DB,       // use default DB
			IdleTimeout:        config.IdleTimeout,
			IdleCheckFrequency: config.IdleCheckFrequency,
		}),
	}

	pong, err := c.Client.Ping(context.Background()).Result()
	log.Debugf("Redis ping : %v, err %v", pong, err)

	return c
}

func (c *RedisCache) Set(k string, v interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		log.Debugf("time to set cache value: %d", time.Since(start).Milliseconds())
	}()

	var err error

	// Set Redis Cache
	if c.Client != nil {
		if ttl == DefaultTTL {
			ttl = c.cacheConfig.DefaultTTL
		}
		data, _ := json.Marshal(v)
		err = c.Client.Set(context.Background(), k, data, ttl).Err()
		if err != nil {
			log.Errorf("Failed to cache value (key = %s) in redis: %v", k, err)
		}
	}
	return err
}

func (c *RedisCache) Get(k string, v interface{}) bool {
	start := time.Now()
	defer func() {
		log.Debugf("time to get cache value: %d", time.Since(start).Milliseconds())
	}()

	if c.Client == nil {
		return false
	}

	data, err := c.Client.Get(context.Background(), k).Result()
	if err != nil {
		log.Debugf("Failed to fetch value (key = %s) from redis: %v", k, err)
		return false
	}
	err = json.Unmarshal([]byte(data), v)
	return err == nil
}

func (c *RedisCache) Delete(k string) {
	start := time.Now()
	defer func() {
		log.Debugf("time to delete cache value: %d", time.Since(start).Milliseconds())
	}()

	if c.Client != nil {
		c.Client.Del(context.Background(), k)
	}
}
