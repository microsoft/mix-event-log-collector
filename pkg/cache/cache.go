package cache

import (
	"fmt"
	"strings"
	"time"

	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

func init() {
	Register("local", NewLocalCache)
	Register("redis", NewRedisCache)
}

const (
	DefaultTTL = time.Duration(-1)
	NoTTL      = time.Duration(0)
)

type CacheConfig struct {
	Type string `json:"type" yaml:"type"`

	/** local cache params **/
	Expiration time.Duration `json:"expiration,omitempty" yaml:"expiration"`

	/** redis cache params **/
	// The network type, either tcp or unix.
	// Default is tcp.
	Network string `json:"network,omitempty" yaml:"network"`
	// host:port address.
	Addr string `json:"addr,omitempty" yaml:"addr"`

	// Optional password. Must match the password specified in the
	// requirepass server configuration option.
	Password string `json:"password,omitempty" yaml:"password" mask:"password"`
	// Database to be selected after connecting to the server.
	DB int `json:"db,omitempty" yaml:"db"`

	// Maximum number of retries before giving up.
	// Default is to not retry failed commands.
	MaxRetries int `json:"max_retries,omitempty" yaml:"max_retries"`
	// Minimum backoff between each retry.
	// Default is 8 milliseconds; -1 disables backoff.
	MinRetryBackoff time.Duration `json:"min_retry_backoff,omitempty" yaml:"min_retry_backoff"`
	// Maximum backoff between each retry.
	// Default is 512 milliseconds; -1 disables backoff.
	MaxRetryBackoff time.Duration `json:"max_retry_backoff,omitempty" yaml:"max_retry_backoff"`

	// Dial timeout for establishing new connections.
	// Default is 5 seconds.
	DialTimeout time.Duration `json:"dial_timeout,omitempty" yaml:"dial_timeout"`
	// Timeout for socket reads. If reached, commands will fail
	// with a timeout instead of blocking. Use value -1 for no timeout and 0 for default.
	// Default is 3 seconds.
	ReadTimeout time.Duration `json:"read_timeout,omitempty" yaml:"read_timeout"`
	// Timeout for socket writes. If reached, commands will fail
	// with a timeout instead of blocking.
	// Default is ReadTimeout.
	WriteTimeout time.Duration `json:"write_timeout,omitempty" yaml:"write_timeout"`

	// Maximum number of socket connections.
	// Default is 10 connections per every CPU as reported by runtime.NumCPU.
	PoolSize int `json:"pool_size,omitempty" yaml:"pool_size"`
	// Minimum number of idle connections which is useful when establishing
	// new connection is slow.
	MinIdleConns int `json:"min_idle_conns,omitempty" yaml:"min_idle_conns"`
	// Connection age at which client retires (closes) the connection.
	// Default is to not close aged connections.
	MaxConnAge time.Duration `json:"max_conn_age,omitempty" yaml:"max_conn_age"`
	// Amount of time client waits for connection if all connections
	// are busy before returning an error.
	// Default is ReadTimeout + 1 second.
	PoolTimeout time.Duration `json:"pool_timeout,omitempty" yaml:"pool_timeout"`
	// Amount of time after which client closes idle connections.
	// Should be less than server's timeout.
	// Default is 900 seconds. -1 disables idle timeout check.
	IdleTimeout time.Duration `json:"idle_timeout,omitempty" yaml:"idle_timeout"`
	// Frequency of idle checks made by idle connections reaper.
	// Default is 1 minute. -1 disables idle connections reaper,
	// but idle connections are still discarded by the client
	// if IdleTimeout is set.
	IdleCheckFrequency time.Duration `json:"idle_check_frequency,omitempty" yaml:"idle_check_frequency"`

	DefaultTTL time.Duration `json:"default_ttl" yaml:"default_ttl"`
}

type Cache interface {
	Set(key string, val interface{}, ttl time.Duration) error
	Get(key string, val interface{}) bool
	Delete(key string)
}

type CacheFactory func(config CacheConfig) Cache

var cacheFactories = make(map[string]CacheFactory)

// Each cache implementation must Register itself
func Register(name string, factory CacheFactory) {
	log.Debugf("Registering cache factory for %s", name)
	if factory == nil {
		log.Panicf("Cache factory %s does not exist.", name)
	}
	_, registered := cacheFactories[name]
	if registered {
		log.Errorf("Cache factory %s already registered. Ignoring.", name)
	}
	cacheFactories[name] = factory
}

// CreateCache is a factory method that will create the named cache
func CreateCache(config CacheConfig) (Cache, error) {

	factory, ok := cacheFactories[config.Type]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available datastore factories for logging.
		availableCaches := make([]string, 0)
		for k := range cacheFactories {
			availableCaches = append(availableCaches, k)
		}
		return nil, fmt.Errorf("invalid cache name. must be one of: %s", strings.Join(availableCaches, ", "))
	}

	// Run the factory with the configuration.
	return factory(config), nil
}
