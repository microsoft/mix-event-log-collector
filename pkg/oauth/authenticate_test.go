//go:build authentication
// +build authentication

package oauth_test

import (
	"encoding/json"
	"os"
	"testing"

	"nuance.xaas-logging.event-log-collector/pkg/cache"
	//config "nuance.xaas-logging.event-log-collector/pkg/config"
	"nuance.xaas-logging.event-log-collector/pkg/oauth"
)

var cred Credentials

/* [Note]
 * - Provide ClientID, ClientSecret to execute unit tests.
 */
func TestMain(m *testing.M) {
	// Create credentials object
	cred = Credentials{
		AuthDisabled:  false,
		TokenURL:      "https://auth.crt.nuance.com/oauth2/token",
		Scope:         "log.write",
		AuthTimeoutMS: 5000,
		ClientID:      "appID:XXXXXXX:geo:us:clientName:XXXXXXX",
		ClientSecret:  "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
	}
	os.Exit(m.Run())
}

func TestAuthenticatorIsTokenValidWithRedisCache(t *testing.T) {
	//Declare RedisCache Config content
	configContent := `
    {
      "addr": "localhost:6379",
      "password": "RedisIsGreat",
      "db": 0,
      "default_ttl": 900
    }`

	var config map[string]interface{}
	err := json.Unmarshal([]byte(configContent), &config)
	if err != nil {
		t.Fatalf("%v", err)
	}
	//Create redis cache object
	redisCache := cache.NewRedisCache(config)

	//Create create Authenticator object
	auth := oauth.NewAuthenticatorWithCache(cred, redisCache)

	validateTokenCache(t, auth)
}

func TestAuthenticatorIsTokenValidWithLocalCache(t *testing.T) {

	configContent :=
		`{
        "expiration": 2880,
        "cleanup_interval": 300
	 }`

	var config map[string]interface{}
	err := json.Unmarshal([]byte(configContent), &config)
	if err != nil {
		t.Fatalf("%v", err)
	}
	localCache := cache.NewLocalCache(config)
	auth := oauth.NewAuthenticatorWithCache(cred, localCache)
	validateTokenCache(t, auth)
}

func validateTokenCache(t *testing.T, auth *oauth.Authenticator) {
	//check if token is valid
	maxRetry := 5
	retry := 0
	key := auth.TokenCacheKey()
	var isCached bool
	// Process go if token is not cached
	for !isCached {
		if !auth.IsCachedTokenValid(key) {
			retry++
			t.Logf("No cached token found")
			//Generate token if not valid
			if _, err := auth.GenerateToken(); err != nil {
				t.Fatalf("%v", err)
			}
			t.Logf("Token generated")
			//cache token after generation
			auth.CacheToken()
			t.Logf("Token cached")
			if retry >= maxRetry {
				t.FailNow()
			}
			continue
		}
		isCached = true
		t.Logf("Cached Token : %v", auth.Token().String(true))
		//delete for next testing.
		auth.Cache.Delete(auth.TokenCacheKey())
		t.Logf("Cached token purged")
	}
}
