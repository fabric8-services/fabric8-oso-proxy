package middlewares

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	Authorization = "Authorization"
)

type TenantLocator func(token string) (string, error)
type TenantTokenLocator func(token, location string) (string, error)

type cacheData struct {
	Token    string
	Location string
}

type OSIOAuth struct {
	RequestTenantLocation TenantLocator
	RequestTenantToken    TenantTokenLocator
	cache                 *Cache
}

func NewPreConfiguredOSIOAuth() *OSIOAuth {
	witURL := os.Getenv("WIT_URL")
	if witURL == "" {
		panic("Missing WIT_URL")
	}
	authURL := os.Getenv("AUTH_URL")
	if authURL == "" {
		panic("Missing AUTH_URL")
	}

	return NewOSIOAuth(witURL, authURL)
}

func NewOSIOAuth(witURL, authURL string) *OSIOAuth {
	return &OSIOAuth{
		RequestTenantLocation: CreateTenantLocator(http.DefaultClient, witURL),
		RequestTenantToken:    CreateTenantTokenLocator(http.DefaultClient, authURL),
		cache:                 &Cache{},
	}
}

func cacheResolver(locationLocator TenantLocator, tokenLocator TenantTokenLocator, osioToken string) Resolver {
	return func() (interface{}, error) {
		loc, err := locationLocator(osioToken)
		if err != nil {
			return cacheData{}, err
		}
		osoToken, err := tokenLocator(osioToken, loc)
		if err != nil {
			return cacheData{}, err
		}
		fmt.Println("resolved..")
		return cacheData{Location: loc, Token: osoToken}, nil
	}
}

func (a *OSIOAuth) resolve(osioToken string) (cacheData, error) {
	key := cacheKey(osioToken)
	val, err := a.cache.Get(key, cacheResolver(a.RequestTenantLocation, a.RequestTenantToken, osioToken)).Get()

	if data, ok := val.(cacheData); ok {
		return data, err
	}
	return cacheData{}, err
}

//
func (a *OSIOAuth) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if a.RequestTenantLocation != nil {

		if r.Method != "OPTIONS" {
			osioToken, err := getToken(r)
			if err != nil {
				rw.WriteHeader(401)
				return
			}

			cached, err := a.resolve(osioToken)
			if err != nil {
				rw.WriteHeader(401)
				return
			}
			r.Header.Set("Target", cached.Location)
			r.Header.Set("Authorization", "Bearer "+cached.Token)
		} else {
			r.Header.Set("Target", "default")
		}
	}
	next(rw, r)
}

func getToken(r *http.Request) (string, error) {
	if at := r.URL.Query().Get("access_token"); at != "" {
		r.URL.Query().Del("access_token")
		return at, nil
	}
	t, err := extractToken(r.Header.Get(Authorization))
	if err != nil {
		return "", err
	}
	return t, nil
}

func extractToken(auth string) (string, error) {
	auths := strings.Split(auth, " ")
	if len(auths) == 0 {
		return "", fmt.Errorf("Invalid auth")
	}
	return auths[len(auths)-1], nil
}

func cacheKey(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	hash := hex.EncodeToString(h.Sum(nil))
	return hash
}
