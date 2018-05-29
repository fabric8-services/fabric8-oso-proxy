package middlewares

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/containous/traefik/log"
)

const (
	Authorization = "Authorization"
	UserIDHeader  = "Impersonate-User"
)

const (
	api     = "api"
	che     = "che"
	metrics = "metrics"
	console = "console"
	logs    = "logs"
)

type TenantLocator interface {
	GetTenant(token string) (namespace, error)
	GetTenantById(token, userID string) (namespace, error)
}

type TenantTokenLocator interface {
	GetTokenWithUserToken(userToken, location string) (string, error)
	GetTokenWithSAToken(saToken, location string) (string, error)
}

type SrvAccTokenLocator func() (string, error)

type SecretLocator interface {
	GetName(clusterUrl, clusterToken, nsName, nsType string) (string, error)
	GetSecret(clusterUrl, clusterToken, nsName, secretName string) (string, error)
}

type SrvAccTokenChecker func(string) (bool, error)

type cacheData struct {
	Token     string
	Namespace namespace
}

type OSIOAuth struct {
	RequestTenantLocation TenantLocator
	RequestTenantToken    TenantTokenLocator
	RequestSrvAccToken    SrvAccTokenLocator
	RequestSecretLocation SecretLocator
	CheckSrvAccToken      SrvAccTokenChecker
	cache                 *Cache
}

func NewPreConfiguredOSIOAuth() *OSIOAuth {
	authTokenKey := os.Getenv("AUTH_TOKEN_KEY")
	if authTokenKey == "" {
		panic("Missing AUTH_TOKEN_KEY")
	}
	tenantURL := os.Getenv("TENANT_URL")
	if tenantURL == "" {
		panic("Missing TENANT_URL")
	}
	authURL := os.Getenv("AUTH_URL")
	if authURL == "" {
		panic("Missing AUTH_URL")
	}

	srvAccID := os.Getenv("SERVICE_ACCOUNT_ID")
	if len(srvAccID) <= 0 {
		panic("Missing SERVICE_ACCOUNT_ID")
	}
	srvAccSecret := os.Getenv("SERVICE_ACCOUNT_SECRET")
	if len(srvAccSecret) <= 0 {
		panic("Missing SERVICE_ACCOUNT_SECRET")
	}
	return NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret)
}

func NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret string) *OSIOAuth {
	return &OSIOAuth{
		RequestTenantLocation: CreateTenantLocator(http.DefaultClient, tenantURL),
		RequestTenantToken:    CreateTenantTokenLocator(http.DefaultClient, authURL),
		RequestSrvAccToken:    CreateSrvAccTokenLocator(authURL, srvAccID, srvAccSecret),
		RequestSecretLocation: CreateSecretLocator(http.DefaultClient),
		CheckSrvAccToken:      CreateSrvAccTokenChecker(http.DefaultClient, authURL),
		cache:                 &Cache{},
	}
}

func cacheResolverByID(tenantLocator TenantLocator, tokenLocator TenantTokenLocator, srvAccTokenLocator SrvAccTokenLocator, secretLocator SecretLocator, token, userID string) Resolver {
	return func() (interface{}, error) {
		namespace, err := tenantLocator.GetTenantById(token, userID)
		if err != nil {
			log.Errorf("Failed to locate tenant, %v", err)
			return cacheData{}, err
		}
		osoProxySAToken, err := srvAccTokenLocator()
		if err != nil {
			log.Errorf("Failed to locate service account token, %v", err)
			return cacheData{}, err
		}
		clusterToken, err := tokenLocator.GetTokenWithSAToken(osoProxySAToken, namespace.ClusterURL)
		if err != nil {
			log.Errorf("Failed to locate cluster token, %v", err)
			return cacheData{}, err
		}
		secretName, err := secretLocator.GetName(namespace.ClusterURL, clusterToken, namespace.Name, namespace.Type)
		if err != nil {
			log.Errorf("Failed to locate secret name, %v", err)
			return cacheData{}, err
		}
		osoToken, err := secretLocator.GetSecret(namespace.ClusterURL, clusterToken, namespace.Name, secretName)
		if err != nil {
			log.Errorf("Failed to get secret, %v", err)
			return cacheData{}, err
		}
		return cacheData{Namespace: namespace, Token: osoToken}, nil
	}
}

func cacheResolverByToken(tenantLocator TenantLocator, tokenLocator TenantTokenLocator, token string) Resolver {
	return func() (interface{}, error) {
		namespace, err := tenantLocator.GetTenant(token)
		if err != nil {
			log.Errorf("Failed to locate tenant, %v", err)
			return cacheData{}, err
		}
		osoToken, err := tokenLocator.GetTokenWithUserToken(token, namespace.ClusterURL)
		if err != nil {
			log.Errorf("Failed to locate token, %v", err)
			return cacheData{}, err
		}
		return cacheData{Namespace: namespace, Token: osoToken}, nil
	}
}

func (a *OSIOAuth) resolveByToken(token string) (cacheData, error) {
	key := cacheKey(token)
	val, err := a.cache.Get(key, cacheResolverByToken(a.RequestTenantLocation, a.RequestTenantToken, token)).Get()

	if data, ok := val.(cacheData); ok {
		return data, err
	}
	return cacheData{}, err
}

func (a *OSIOAuth) resolveByID(userID, token string) (cacheData, error) {
	plainKey := fmt.Sprintf("%s_%s", token, userID)
	key := cacheKey(plainKey)
	val, err := a.cache.Get(key, cacheResolverByID(a.RequestTenantLocation, a.RequestTenantToken, a.RequestSrvAccToken, a.RequestSecretLocation, token, userID)).Get()

	if data, ok := val.(cacheData); ok {
		return data, err
	}
	return cacheData{}, err
}

func (a *OSIOAuth) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	if a.RequestTenantLocation != nil {

		if r.Method != "OPTIONS" {
			token, err := getToken(r)
			if err != nil {
				log.Errorf("Token not found, %v", err)
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}

			isSerivce, err := a.CheckSrvAccToken(token)
			if err != nil {
				log.Errorf("Invalid token, %v", err)
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}

			var cached cacheData
			if isSerivce {
				userID := r.Header.Get(UserIDHeader)
				if userID == "" {
					log.Errorf("%s header is missing", UserIDHeader)
					rw.WriteHeader(http.StatusUnauthorized)
					return
				}
				cached, err = a.resolveByID(userID, token)
			} else {
				cached, err = a.resolveByToken(token)
			}

			if err != nil {
				log.Errorf("Cache resolve failed, %v", err)
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}

			reqType := getRequestType(r)
			stripRequestPathPrefix(r, reqType)
			targetURL := getTargetURL(cached.Namespace, reqType)
			targetURL = normalizeURL(targetURL)

			if isRedirectRequest(reqType) {
				redirectURL := targetURL + r.URL.Path
				if reqType == logs && r.URL.RawQuery != "" {
					redirectURL = strings.Join([]string{redirectURL, "?", r.URL.RawQuery}, "")
				}
				http.Redirect(rw, r, redirectURL, http.StatusTemporaryRedirect)
				return
			} else {
				r.Header.Set("Target", targetURL)
				r.Header.Set("Authorization", "Bearer "+cached.Token)
			}
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

func extractToken(auth string) (string, error) {
	auths := strings.Split(auth, " ")
	if len(auths) == 0 {
		return "", fmt.Errorf("Invalid auth")
	}
	return auths[len(auths)-1], nil
}

func getRequestType(req *http.Request) string {
	reqPath := req.URL.Path
	switch {
	case strings.HasPrefix(reqPath, "/"+api):
		return api
	case strings.HasPrefix(reqPath, "/"+metrics):
		return metrics
	case strings.HasPrefix(reqPath, "/"+console):
		return console
	case strings.HasPrefix(reqPath, "/"+logs):
		return logs
	default:
		return api
	}
}

func getTargetURL(ns namespace, reqType string) string {
	switch reqType {
	case api:
		return ns.ClusterURL
	case metrics:
		return ns.ClusterMetricsURL
	case console:
		return ns.ClusterConsoleURL
	case logs:
		return ns.ClusterLoggingURL
	default:
		return ns.ClusterURL
	}
}

func isRedirectRequest(reqType string) bool {
	if reqType == console || reqType == logs {
		return true
	}
	return false
}

func cacheKey(plainKey string) string {
	h := sha256.New()
	h.Write([]byte(plainKey))
	hash := hex.EncodeToString(h.Sum(nil))
	return hash
}

func stripRequestPathPrefix(r *http.Request, reqType string) {
	prefix := "/" + reqType
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

func normalizeURL(url string) string {
	return strings.TrimSuffix(url, "/")
}
