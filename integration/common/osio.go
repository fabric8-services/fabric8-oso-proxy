package common

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	jose "gopkg.in/square/go-jose.v1"
)

const (
	TenantURL = "http://127.0.0.1:9090/api"
	AuthURL   = "http://127.0.0.1:9091/api"
)

var keyID = "test-key"

var TestTokenManager = NewTestTokenManager()

func StartServer(port int, handler func(w http.ResponseWriter, r *http.Request)) (ts *httptest.Server) {
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "port=%d", port)
		}
	}
	if listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
		panic(err)
	} else {
		ts = &httptest.Server{
			Listener: listener,
			Config:   &http.Server{Handler: http.HandlerFunc(handler)},
		}
		ts.Start()
	}
	return
}

func ServeTenantRequest(rw http.ResponseWriter, req *http.Request) {
	authHeader := req.Header.Get("Authorization")
	token := strings.Split(authHeader, " ")[1]
	jwtToken := TestTokenManager.ToJwtToken(token)
	sub, _ := jwtToken.Claims.(jwt.MapClaims)["sub"].(string)

	metricsHost := ""
	apiHost := ""
	switch {
	case strings.HasSuffix(sub, "1111"):
		metricsHost = "http://127.0.0.1:7071"
		apiHost = "http://127.0.0.1:8081"
	case strings.HasSuffix(sub, "2222"):
		metricsHost = "http://127.0.0.1:7072"
		apiHost = "http://127.0.0.1:8082"
	case strings.HasSuffix(sub, "3333"):
		metricsHost = "http://127.0.0.1:7073"
		apiHost = "http://127.0.0.1:8083" // :8083 is not present in toml file
	case strings.HasSuffix(sub, "4444"):
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	res := fmt.Sprintf(`{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "myuser-preview-stage",
						"cluster-metrics-url": "%s",
						"cluster-url": "%s"
					}
				]
			}
		}
	}`, metricsHost, apiHost)
	rw.Write([]byte(res))
}

func ServerAuthRequest(serverClusterAPI func() string) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/api/token" {
			tokenAPIResponse := `{"access_token": "1111","token_type": "bearer"}`
			rw.Write([]byte(tokenAPIResponse))
		} else if path == "/api/clusters" {
			clustersAPIResponse := serverClusterAPI()
			rw.Write([]byte(clustersAPIResponse))
		} else if path == "/api/token/keys" {
			keyJSON, _ := TestTokenManager.GetPublicKeyJSON()
			rw.Write(keyJSON)
		}
	}
}

func TwoClusterData() string {
	// "api-url": "https://api.starter-us-east-2.openshift.com/",
	// "api-url": "https://api.starter-us-east-2a.openshift.com/",
	res := `{
		"data": [
			{
				"api-url": "http://127.0.0.1:8081/",
				"app-dns": "8a09.starter-us-east-2.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2.openshift.com/console/",
				"metrics-url": "http://127.0.0.1:7071/",
				"name": "us-east-2"
			},
			{
				"api-url": "http://127.0.0.1:8082/",
				"app-dns": "b542.starter-us-east-2a.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2a.openshift.com/console/",
				"metrics-url": "http://127.0.0.1:7072/",
				"name": "us-east-2a"
			}
		]
	}`
	return res
}

func OneClusterData() string {
	// "api-url": "https://api.starter-us-east-2.openshift.com/",
	res := `{
		"data": [
			{
				"api-url": "http://localhost:8081/",
				"app-dns": "8a09.starter-us-east-2.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2.openshift.com/console/",
				"metrics-url": "http://127.0.0.1:7071/",
				"name": "us-east-2"
			}
		]
	}`
	return res
}

type testTokenManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewTestTokenManager() *testTokenManager {
	priKey := loadRSAPrivateKeyFromDisk("./common/sample_key")
	pubKey := loadRSAPublicKeyFromDisk("./common/sample_key.pub")
	return &testTokenManager{privateKey: priKey, publicKey: pubKey}
}

func (t *testTokenManager) ToTokenString(c jwt.Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	token.Header["kid"] = keyID
	s, e := token.SignedString(t.privateKey)
	if e != nil {
		panic(e.Error())
	}
	return s
}

func (t *testTokenManager) ToJwtToken(tokenStr string) *jwt.Token {
	jwtToken, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return t.publicKey, nil
	})
	if err != nil {
		panic(err)
	}
	return jwtToken
}

func (t *testTokenManager) GetPublicKeyJSON() ([]byte, error) {
	jsonKey, _ := toJSONWebKeys(keyID, t.publicKey)
	return json.Marshal(jsonKey)
}

func loadRSAPrivateKeyFromDisk(location string) *rsa.PrivateKey {
	keyData, e := ioutil.ReadFile(location)
	if e != nil {
		panic(e.Error())
	}
	key, e := jwt.ParseRSAPrivateKeyFromPEM(keyData)
	if e != nil {
		panic(e.Error())
	}
	return key
}

func loadRSAPublicKeyFromDisk(location string) *rsa.PublicKey {
	keyData, e := ioutil.ReadFile(location)
	if e != nil {
		panic(e.Error())
	}
	key, e := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if e != nil {
		panic(e.Error())
	}
	return key
}

type JSONKeys struct {
	Keys []interface{} `json:"keys"`
}

func toJSONWebKeys(keyID string, key *rsa.PublicKey) (*JSONKeys, error) {
	var result []interface{}
	jwkey := jose.JsonWebKey{Key: key, KeyID: keyID, Algorithm: "RS256", Use: "sig"}
	keyData, err := jwkey.MarshalJSON()
	if err != nil {
		return &JSONKeys{}, err
	}
	var raw interface{}
	err = json.Unmarshal(keyData, &raw)
	if err != nil {
		return &JSONKeys{}, err
	}
	result = append(result, raw)
	return &JSONKeys{Keys: result}, nil
}
