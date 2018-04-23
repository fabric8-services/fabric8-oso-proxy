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
	api           = "api"
	metrics       = "metrics"
)

type TenantLocator func(token, service string) (namespace, error)
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

func cacheResolver(locationLocator TenantLocator, tokenLocator TenantTokenLocator, osioToken, osoService string) Resolver {
	return func() (interface{}, error) {
		ns, err := locationLocator(osioToken, osoService)
		if err != nil {
			return cacheData{}, err
		}
		osoToken, err := tokenLocator(osioToken, ns.ClusterURL)
		if err != nil {
			return cacheData{}, err
		}
		loc := getURL(ns, osoService)
		return cacheData{Location: loc, Token: osoToken}, nil
	}
}

func (a *OSIOAuth) resolve(osioToken, osoService string) (cacheData, error) {
	key := cacheKey(osioToken, osoService)
	val, err := a.cache.Get(key, cacheResolver(a.RequestTenantLocation, a.RequestTenantToken, osioToken, osoService)).Get()

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
			osoService := getService(r.URL.Path)
			if err != nil {
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}

			cached, err := a.resolve(osioToken, osoService)
			if err != nil {
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}
			r.Header.Set("Target", cached.Location)
			r.Header.Set("Authorization", "Bearer "+cached.Token)
			stripPathPrefix(r, osoService)
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
	if t == "" {
		return "", fmt.Errorf("Missing auth")
	}
	return t, nil
}

func getService(reqPath string) string {
	switch {
	case strings.HasPrefix(reqPath, "/"+metrics):
		return metrics
	case strings.HasPrefix(reqPath, "/"+api):
		return api
	default:
		return api
	}
}

func extractToken(auth string) (string, error) {
	auths := strings.Split(auth, " ")
	if len(auths) == 0 {
		return "", fmt.Errorf("Invalid auth")
	}
	return auths[len(auths)-1], nil
}

func cacheKey(token, service string) string {
	h := sha256.New()
	key := fmt.Sprintf("%s_%s", token, service)
	h.Write([]byte(key))
	hash := hex.EncodeToString(h.Sum(nil))
	return hash
}

func getURL(ns namespace, service string) string {
	switch service {
	case metrics:
		return ns.ClusterMetricsURL
	case api:
		return ns.ClusterURL
	default:
		return ns.ClusterURL
	}
}

func stripPathPrefix(r *http.Request, osoService string) {
	prefix := "/" + osoService
	if strings.HasPrefix(r.URL.Path, prefix) {
		r.URL.Path = stripPrefix(r.URL.Path, prefix)
		r.RequestURI = r.URL.RequestURI()
	}
}

func stripPrefix(s, prefix string) string {
	return ensureLeadingSlash(strings.TrimPrefix(s, prefix))
}

func ensureLeadingSlash(str string) string {
	return "/" + strings.TrimPrefix(str, "/")
}
