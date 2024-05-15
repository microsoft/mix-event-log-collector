package cache

import (
	"encoding/json"
	"time"

	local "github.com/jellydator/ttlcache/v3"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type OnEvicted func(string, interface{})

type LocalCache struct {
	cacheConfig CacheConfig
	Client      *local.Cache[string, any]
}

func NewLocalCache(config CacheConfig) Cache {
	log.Debugf("cache config: %+v", config)
	config.Expiration = config.Expiration * time.Second
	return &LocalCache{
		cacheConfig: config,
		Client: local.New(
			local.WithTTL[string, any](config.Expiration),
		),
	}
}

func (c *LocalCache) Set(k string, v interface{}, ttl time.Duration) error {

	// the library being used expects -1 for no ttl, and 0 for default
	switch ttl {
	case DefaultTTL:
		ttl = 0
	case NoTTL:
		ttl = -1
	default:
		// do nothing
	}
	c.Client.Set(k, v, ttl)
	return nil
}

func (c *LocalCache) Get(k string, v interface{}) bool {
	obj := c.Client.Get(k)
	if obj == nil {
		return false
	}
	data, err := json.Marshal(obj.Value())
	if err != nil {
		return false
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		log.Errorf(err.Error())
	}
	return err == nil
}

func (c *LocalCache) Delete(k string) {
	c.Client.Delete(k)
}
