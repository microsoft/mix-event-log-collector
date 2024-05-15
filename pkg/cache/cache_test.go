package cache_test

import (
	"fmt"
	"testing"

	"nuance.xaas-logging.event-log-collector/pkg/cache"
)

type jsonObj = map[string]interface{}

func TestLocalCacheSet(t *testing.T) {
	// local cache config object
	config := cache.CacheConfig{
		Type:       "local",
		Expiration: 2880,
	}

	//create local cache using factory
	localCache, err := cache.CreateCache(config)
	if err != nil {
		t.Error(err)
	}
	//run set method
	err = localCache.Set("testkey", "testvalue", cache.DefaultTTL)
	if err != nil {
		t.Error(err)
	}
}

func TestLocalCacheGetStringValue(t *testing.T) {
	// local cache config object
	config := cache.CacheConfig{
		Type:       "local",
		Expiration: 2880,
	}

	//create local cache using factory
	localCache, err := cache.CreateCache(config)
	if err != nil {
		t.Error(err)
	}

	data := "testvalue"

	//run set method
	err = localCache.Set("testkey", data, cache.DefaultTTL)
	if err != nil {
		t.Error(err)
	}

	//run get method
	val := ""
	exist := localCache.Get("testkey", &val)
	if !exist {
		t.Logf("failed to get value with key %v", "testkey")
		t.Fail()
		return
	}
	fmt.Println("get value:", val)
}

func TestLocalCacheGetObjectValue(t *testing.T) {
	// local cache config object
	config := cache.CacheConfig{
		Type:       "local",
		Expiration: 2880,
	}

	//create local cache using factory
	localCache, err := cache.CreateCache(config)
	if err != nil {
		t.Error(err)
	}

	data := make(jsonObj, 1)
	data["example"] = 1

	//run set method
	err = localCache.Set("testkey", data, cache.DefaultTTL)
	if err != nil {
		t.Error(err)
	}

	//run get method
	val := make(jsonObj)
	exist := localCache.Get("testkey", &val)
	if !exist {
		t.Logf("failed to get value with key %v", "testkey")
		t.Fail()
		return
	}
	fmt.Println("get value:", val)
}
