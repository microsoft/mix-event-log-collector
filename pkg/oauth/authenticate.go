package oauth

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nuance.xaas-logging.event-log-collector/pkg/cache"
	"nuance.xaas-logging.event-log-collector/pkg/httpclient"

	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	//config "nuance.xaas-logging.event-log-collector/pkg/pipeline/config"
)

const (
	TokenCacheDir      = ".tokens"
	TokenCacheTemplate = "%s/token.%s.%v.cache"
	GrantType          = "client_credentials"
	Scope              = "log log.write"
	DefaultTimeoutMS   = 5000

	ErrFailedToCacheToken = "failed to cache token: %v"
)

type Credentials struct {
	AuthDisabled  bool   `json:"auth_disabled"`
	TokenURL      string `json:"token_url,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	ClientSecret  string `json:"client_secret,omitempty"`
	Scope         string `json:"scope,omitempty"`
	AuthTimeoutMS int    `json:"auth_timeout_ms,omitempty"`
}

type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

// String returns a Token object as a json-formatted string
func (t *Token) String(pretty bool) string {
	var str []byte
	var err error

	if pretty {
		str, _ = json.MarshalIndent(t, "", "  ")
	} else {
		str, _ = json.Marshal(t)
	}

	if err != nil {
		log.Infof("Error marshalling token to json: %s", err)
	}

	return string(str)
}

// Authenticator manages oauth token generation and refresh
type Authenticator struct {
	config Credentials
	token  *Token
	Cache  cache.Cache
	http   httpclient.Http
	mu     sync.Mutex
}

// Token returns the authenticator's current token
func (a *Authenticator) Token() *Token {

	return a.token
}

func (a *Authenticator) TokenUrl() *url.URL {

	url, _ := url.Parse(a.config.TokenURL)
	return url

}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// TokenCacheKey returns the path to the cached token data
func (a *Authenticator) TokenCacheKey() string {
	return fmt.Sprintf(TokenCacheTemplate, TokenCacheDir, strings.ReplaceAll(a.Credentials().ClientID, ":", "-"), hash(a.Credentials().Scope))
}

func (a *Authenticator) Header(clientId string, clientSecret string) map[string][]string {
	auth := clientId + ":" + clientSecret
	return map[string][]string{
		"Authorization": {fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(auth)))},
		"Content-Type":  {"application/x-www-form-urlencoded"},
	}
}

func (a *Authenticator) GenerateToken() (*Token, error) {
	a.SetToken(nil)

	body := strings.NewReader(fmt.Sprintf("grant_type=%s&scope=%s", GrantType, a.config.Scope))

	authTimeoutMs := DefaultTimeoutMS
	if a.config.AuthTimeoutMS > 0 {
		authTimeoutMs = a.config.AuthTimeoutMS
	}
	a.http.HttpClient.Timeout = time.Duration(authTimeoutMs) * time.Millisecond

	header := a.Header(url.QueryEscape(a.config.ClientID), url.QueryEscape(a.config.ClientSecret))
	buf := new(bytes.Buffer)

	buf.ReadFrom(body)
	data := buf.Bytes()
	statusCode, resp, err := a.http.Post(a.TokenUrl(), header, data)

	if err != nil || statusCode >= 300 {
		return nil, errors.New(string(resp))
	}
	t := &Token{}
	err = json.Unmarshal(resp, t)
	if err != nil {
		return nil, err
	}

	a.token = t
	return a.token, nil
}

func (a *Authenticator) isTokenValid() bool {

	key := a.TokenCacheKey()

	if a.Cache != nil && a.IsCachedTokenValid(key) {
		return true
	}

	return a.isFileTokenValid(key)
}

func (a *Authenticator) IsCachedTokenValid(key string) bool {
	// Is token cached?
	t := &Token{}

	exists := a.Cache.Get(key, t)
	log.Debugf("Auth token cache hit: %v", exists)

	if !exists {
		return false
	}
	log.Debugf("Auth token fetched from cache: %v", *t)

	if len(t.AccessToken) == 0 {
		return false
	}
	a.SetToken(t)
	return true
}

func (a *Authenticator) isFileTokenValid(key string) bool {
	// Is token cached?
	info, err := os.Stat(key)
	if err != nil {
		return false
	}

	// Can token be read from file?
	source, err := os.ReadFile(filepath.Clean(key))
	if err != nil {
		return false
	}

	// Are contents of token valid?
	t := &Token{}
	err = json.Unmarshal(source, t)
	if err != nil || len(t.AccessToken) == 0 {
		return false
	}

	// Has token expired?
	lapsed := time.Since(info.ModTime())
	expiresIn := time.Duration(t.ExpiresIn) * time.Second
	if lapsed > expiresIn {
		return false
	}

	// All tests passed
	a.SetToken(t)
	return true
}

func (a *Authenticator) SetToken(t *Token) {

	a.token = t
}

func (a *Authenticator) CacheToken() {

	key := a.TokenCacheKey()
	tokenAsBytes, err := json.MarshalIndent(a.token, "", "  ")
	if err != nil {
		log.Infof(ErrFailedToCacheToken, err)
		return
	}

	if a.Cache != nil {
		drift, _ := rand.Int(rand.Reader, big.NewInt(60))
		allowance := 10
		expiration := a.token.ExpiresIn - allowance - int(drift.Int64())
		err := a.Cache.Set(key, a.token, time.Duration(time.Duration(expiration)*time.Second))
		if err == nil {
			return
		}
		log.Infof(ErrFailedToCacheToken, err)
	}

	// Cache to file if necessary
	err = os.WriteFile(key, tokenAsBytes, 0600)
	if err != nil {
		log.Infof(ErrFailedToCacheToken, err)
	}
}

// Authenticate generates a new token if there is not a valid one already available in cacheConfig
func (a *Authenticator) Authenticate() (*Token, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config.AuthDisabled {
		return &Token{}, nil
	}

	if a.isTokenValid() {
		return a.token, nil
	}

	if _, err := a.GenerateToken(); err != nil {
		return nil, err
	}

	a.CacheToken()
	return a.token, nil
}

// Credentials returns the configured credentials
func (a *Authenticator) Credentials() Credentials {
	return a.config
}

// NewAuthenticator returns an instance of Authenticator
func NewAuthenticator(config Credentials) *Authenticator {
	return NewAuthenticatorWithCache(config, nil)
}

// NewAuthenticatorWithCache returns an instance of Authenticator with a cache resource
func NewAuthenticatorWithCache(config Credentials, cache cache.Cache) *Authenticator {
	os.MkdirAll(TokenCacheDir, 0750)

	httpClient := http.Client{Timeout: time.Duration(config.AuthTimeoutMS) * time.Millisecond}
	Http := httpclient.NewHttp(httpClient)

	return &Authenticator{
		config: config,
		Cache:  cache,
		http:   *Http,
	}
}
