package osio

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
	Authorization    = "Authorization"
	UserIDHeader     = "Impersonate-User"
	UserIDParam      = "identity_id"
	ParamPathSegment = "ns"
	NamespaceType    = "type"
)

type RequestType string

const (
	api      RequestType = "api"
	metrics  RequestType = "metrics"
	console  RequestType = "console"
	logs     RequestType = "logs"
	undefine RequestType = ""
)

const (
	// service maps to token type, if not a service token then it maps to UserToken
	CheToken  TokenType = "che"
	UserToken TokenType = "user"
)

var TokenTypeMap = map[string]TokenType{
	"rh-che": CheToken,
}

var (
	apiPrefix  = api.path() + api.path()
	oapiPrefix = api.path() + "/oapi"
)

type TokenType string

type TenantLocator interface {
	GetTenant(token string, params paramMap) (namespace, error)
	GetTenantById(token string, userID string, params paramMap) (namespace, error)
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

type TokenTypeLocator func(string) (TokenType, error)

type cacheData struct {
	Token     string
	Namespace namespace
}

type paramMap map[string]string

type OSIOAuth struct {
	RequestTenantLocation TenantLocator
	RequestTenantToken    TenantTokenLocator
	RequestSrvAccToken    SrvAccTokenLocator
	RequestSecretLocation SecretLocator
	RequestTokenType      TokenTypeLocator
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
		RequestTokenType:      CreateTokenTypeLocator(http.DefaultClient, authURL),
		cache:                 &Cache{},
	}
}

func (a *OSIOAuth) cacheResolverByID(token string, userID string, params paramMap) Resolver {
	return func() (interface{}, error) {
		namespace, err := a.RequestTenantLocation.GetTenantById(token, userID, params)
		if err != nil {
			log.Errorf("Failed to locate tenant, %v", err)
			return cacheData{}, err
		}
		osoProxySAToken, err := a.RequestSrvAccToken()
		if err != nil {
			log.Errorf("Failed to locate service account token, %v", err)
			return cacheData{}, err
		}
		clusterToken, err := a.RequestTenantToken.GetTokenWithSAToken(osoProxySAToken, namespace.ClusterURL)
		if err != nil {
			log.Errorf("Failed to locate cluster token, %v", err)
			return cacheData{}, err
		}
		secretName, err := a.RequestSecretLocation.GetName(namespace.ClusterURL, clusterToken, namespace.Name, namespace.Type)
		if err != nil {
			log.Errorf("Failed to locate secret name, %v", err)
			return cacheData{}, err
		}
		osoToken, err := a.RequestSecretLocation.GetSecret(namespace.ClusterURL, clusterToken, namespace.Name, secretName)
		if err != nil {
			log.Errorf("Failed to get secret, %v", err)
			return cacheData{}, err
		}
		return cacheData{Namespace: namespace, Token: osoToken}, nil
	}
}

func (a *OSIOAuth) cacheResolverByToken(token string, params paramMap) Resolver {
	return func() (interface{}, error) {
		namespace, err := a.RequestTenantLocation.GetTenant(token, params)
		if err != nil {
			log.Errorf("Failed to locate tenant, %v", err)
			return cacheData{}, err
		}
		osoToken, err := a.RequestTenantToken.GetTokenWithUserToken(token, namespace.ClusterURL)
		if err != nil {
			log.Errorf("Failed to locate token, %v", err)
			return cacheData{}, err
		}
		return cacheData{Namespace: namespace, Token: osoToken}, nil
	}
}

func (a *OSIOAuth) resolveByToken(token string, params paramMap) (cacheData, error) {
	key := cacheKey(token)
	val, err := a.cache.Get(key, a.cacheResolverByToken(token, params)).Get()

	if data, ok := val.(cacheData); ok {
		return data, err
	}
	return cacheData{}, err
}

func (a *OSIOAuth) resolveByID(userID, token string, params paramMap) (cacheData, error) {
	plainKey := fmt.Sprintf("%s_%s", token, userID)
	key := cacheKey(plainKey)
	val, err := a.cache.Get(key, a.cacheResolverByID(token, userID, params)).Get()

	if data, ok := val.(cacheData); ok {
		return data, err
	}
	return cacheData{}, err
}

func (a *OSIOAuth) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	if a.RequestTenantLocation != nil {

		if r.Method != "OPTIONS" {
			// get token and token type
			token, err := getToken(r)
			if err != nil {
				log.Errorf("Token not found, %v", err)
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}
			tokenType, err := a.RequestTokenType(token)
			if err != nil {
				log.Errorf("Invalid token, %v", err)
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}

			params := getPathSegmentParam(ParamPathSegment, r.URL.Path)
			if params != nil && params[NamespaceType] == "" {
				// tokenType use to determine namespace
				params[NamespaceType] = string(tokenType)
			}

			// retrieve cache data
			var cached cacheData
			if tokenType != UserToken {
				userID := extractUserID(r)
				if userID == "" {
					log.Errorf("user identity is missing")
					rw.WriteHeader(http.StatusUnauthorized)
					return
				}
				cached, err = a.resolveByID(userID, token, params)
			} else {
				cached, err = a.resolveByToken(token, params)
			}
			if err != nil {
				log.Errorf("Cache resolve failed, %v", err)
				rw.WriteHeader(http.StatusUnauthorized)
				return
			}

			// routing or redirect
			reqType := getRequestType(r)
			reqType.stripPathPrefix(r)
			newReqPath := replacePathSegment(r.URL.Path, ParamPathSegment, cached.Namespace.Name)
			setReqPath(r, newReqPath)
			targetURL := normalizeURL(reqType.getTargetURL(cached.Namespace))

			if reqType.isRedirectRequest() {
				redirectURL := reqType.getRedirectURL(targetURL, r)
				http.Redirect(rw, r, redirectURL, http.StatusTemporaryRedirect)
				return
			} else {
				r.Header.Set("Target", targetURL)
				r.Header.Set("Authorization", "Bearer "+cached.Token)
				if tokenType != UserToken {
					removeUserID(r)
				}
			}
		} else {
			r.Header.Set("Target", "default")
		}
	}
	next(rw, r)
}

func getRequestType(req *http.Request) RequestType {
	reqPath := req.URL.Path
	switch {
	case strings.HasPrefix(reqPath, api.path()):
		return api
	case strings.HasPrefix(reqPath, metrics.path()):
		return metrics
	case strings.HasPrefix(reqPath, console.path()):
		return console
	case strings.HasPrefix(reqPath, logs.path()):
		return logs
	default:
		return undefine
	}
}

func (r RequestType) path() string {
	return "/" + string(r)
}

func (r RequestType) getTargetURL(ns namespace) string {
	switch r {
	case api:
		return ns.ClusterURL
	case metrics:
		return ns.ClusterMetricsURL
	case console:
		return ns.ClusterConsoleURL
	case logs:
		return ns.ClusterLoggingURL
	case undefine:
		return ns.ClusterURL
	default:
		return ns.ClusterURL
	}
}

func (r RequestType) stripPathPrefix(req *http.Request) {
	var pathPrefix, stripPath string
	switch r {
	case api:
		if strings.HasPrefix(req.URL.Path, oapiPrefix) {
			pathPrefix = oapiPrefix
		} else {
			pathPrefix = apiPrefix
		}
		stripPath = r.path()
	case metrics, console, logs:
		pathPrefix = r.path()
		stripPath = r.path()
	case undefine:
		return
	}
	stripRequestPathPrefix(req, pathPrefix, stripPath)
}

func (r RequestType) isRedirectRequest() bool {
	if r == console || r == logs {
		return true
	}
	return false
}

func (r RequestType) getRedirectURL(targetURL string, req *http.Request) string {
	redirectURL := targetURL + req.URL.Path
	if r == logs && req.URL.RawQuery != "" {
		redirectURL = strings.Join([]string{redirectURL, "?", req.URL.RawQuery}, "")
	}
	return redirectURL
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

func cacheKey(plainKey string) string {
	h := sha256.New()
	h.Write([]byte(plainKey))
	hash := hex.EncodeToString(h.Sum(nil))
	return hash
}

func stripRequestPathPrefix(req *http.Request, pathPrefix, stripPath string) {
	if strings.HasPrefix(req.URL.Path, pathPrefix) {
		path := stripPrefix(req.URL.Path, stripPath)
		setReqPath(req, path)
	}
}

func setReqPath(req *http.Request, path string) {
	req.URL.Path = path
	req.RequestURI = req.URL.RequestURI()
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

func extractUserID(req *http.Request) string {
	userID := ""
	if req.Header.Get(UserIDHeader) != "" {
		userID = req.Header.Get(UserIDHeader)
		log.Infof("Got header '%s' with value '%s'", UserIDHeader, userID)
	} else if req.URL.Query().Get(UserIDParam) != "" {
		userID = req.URL.Query().Get(UserIDParam)
		log.Infof("Got query param '%s' with value '%s'", UserIDParam, userID)
		if strings.Contains(userID, "/") {
			endInd := strings.Index(userID, "/")
			userID = userID[:endInd]
		}
	}
	return userID
}

func removeUserID(req *http.Request) {
	if req.Header.Get(UserIDHeader) != "" {
		req.Header.Del(UserIDHeader)
	}
	userID := req.URL.Query().Get(UserIDParam)
	if userID != "" {
		if strings.Contains(userID, "/") {

			// Processing query params
			rawQuery := req.URL.RawQuery
			if strings.Contains(rawQuery, "?") {
				queryIndex := strings.LastIndex(rawQuery, "?")
				rawQuery = rawQuery[queryIndex+1:]
			} else if strings.Contains(rawQuery, "&") {
				ampersandIndex := strings.Index(rawQuery, "&")
				rawQuery = rawQuery[ampersandIndex+1:]
			} else {
				rawQuery = ""
			}
			req.URL.RawQuery = rawQuery

			// Removing query params from userID
			if strings.Contains(userID, "?") {
				indexOfQuery := strings.Index(userID, "?")
				userID = userID[:indexOfQuery]
			}

			// obtaining path
			indexOfFirstSlash := strings.Index(userID, "/")
			path := userID[indexOfFirstSlash:]

			req.URL.Path = path
			req.RequestURI = req.URL.RequestURI()
		} else {
			q := req.URL.Query()
			q.Del(UserIDParam)
			req.URL.RawQuery = q.Encode()
		}
	}
}

// /api/v1/namespaces/ns;type=stage;space=997f146d-b0f4-4a97-ab20-6414878d9508;w=true/pods
func getPathSegmentParam(pathSegName, path string) paramMap {
	var params paramMap = make(map[string]string)
	segments := strings.Split(path, "/")
	for _, segment := range segments {
		parts := strings.Split(segment, ";")
		if len(parts) >= 2 {
			segName := parts[0]
			if segName == pathSegName {
				for i := 1; i < len(parts); i++ {
					paramParts := strings.Split(parts[i], "=")
					if len(paramParts) == 2 {
						params[paramParts[0]] = paramParts[1]
					}
				}
				if len(params) > 0 {
					return params
				}
			}
		}
	}
	return params
}

func replacePathSegment(path, pathSegName, newPathSeg string) string {
	targetSegInd := -1
	newPath := path

	segments := strings.Split(path, "/")
	for ind, segment := range segments {
		parts := strings.Split(segment, ";")
		if len(parts) >= 2 {
			segName := parts[0]
			if segName == pathSegName {
				targetSegInd = ind
				break
			}
		}
	}
	if targetSegInd != -1 {
		if newPathSeg != "" {
			segments[targetSegInd] = newPathSeg
		} else {
			segments = append(segments[:targetSegInd], segments[targetSegInd+1:]...)
		}
		newPath = strings.Join(segments, "/")
	}
	return newPath
}
