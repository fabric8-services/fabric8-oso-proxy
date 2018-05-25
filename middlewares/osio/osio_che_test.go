package middlewares

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCheData struct {
	inputPath      string
	userID         string
	expectedTarget string
	expectedToken  string
}

var cheDataTables = []testCheData{
	{
		"/api",
		"john",
		"127.0.0.1:9091/",
		"1000_che_secret",
	},
	{
		"/api",
		"john",
		"127.0.0.1:9091/",
		"1000_che_secret",
	},
}

var currCheTestInd int
var cheTenantCalls int

func TestBasic(t *testing.T) {
	os.Setenv("AUTH_TOKEN_KEY", "foo")

	authServer := createServer(serveAuthRequest)
	defer authServer.Close()
	tenantServer := createServer(serverTenantRequest)
	defer tenantServer.Close()

	authURL := "http://" + authServer.Listener.Addr().String()
	tenantURL := "http://" + tenantServer.Listener.Addr().String()
	srvAccID := "sa1"
	srvAccSecret := "secret"

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret)
	osio.CheckSrvAccToken = testSrvAccToken
	osioServer := createServer(serverOSIORequest(osio))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range cheDataTables {
		currCheTestInd = ind
		cluster := startServer(table.expectedTarget, serverClusterRequest)

		currReqPath := table.inputPath
		cheSAToken := "1000_che_sa_token"

		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set("Authorization", "Bearer "+cheSAToken)
		q := req.URL.Query()
		q.Add("identity_id", table.userID)
		req.URL.RawQuery = q.Encode()
		res, _ := http.DefaultClient.Do(req)
		assert.NotNil(t, res)
		err := res.Header.Get("err")
		assert.Empty(t, err, err)

		cluster.Close()
	}
	expecteTenantCalls := 1
	assert.Equal(t, expecteTenantCalls, cheTenantCalls, "Number of time Tenant server called was incorrect, want:%d, got:%d", expecteTenantCalls, cheTenantCalls)
}

func createServer(handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	return httptest.NewServer(mux)
}

func serveAuthRequest(rw http.ResponseWriter, req *http.Request) {
	var res string
	if strings.HasSuffix(req.URL.Path, "/token") && req.Method == "POST" {
		res = `{
			"access_token": "1000_oso_proxy_sa_token",
			"token_type": "bearer"
		}`
	} else if strings.HasSuffix(req.URL.Path, "/token") && req.Method == "GET" {
		res = `{
			"access_token": "jA0ECQMCtCG1bfGEQbxg0kABEQ6nh/A4tMGGkHMHJtLDtFLyXh28IuLvoyGjsZtWPV0LHwN+EEsTtu90BQGbWFdBv+2Wiedk9eE3h08lwA8m",
			"scope": "<unknown>",
			"token_type": "bearer",
			"username": "dsaas"
		}`
	}
	rw.Write([]byte(res))
}

func serverTenantRequest(rw http.ResponseWriter, req *http.Request) {
	cheTenantCalls++
	var res string
	if strings.HasSuffix(req.URL.Path, "/tenants/john") {
		res = `{
			"data": {
				"attributes": {
					"namespaces": [
						{
							"name": "john-preview-che",
							"type": "che",
							"cluster-url": "http://127.0.0.1:9091/"
						}
					]
				}
			}
		}`
	}
	rw.Write([]byte(res))
}

func serverClusterRequest(rw http.ResponseWriter, req *http.Request) {
	res := ""
	if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/john-preview-che/serviceaccounts/che") {
		res = `{
			"kind": "ServiceAccount",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che",
			  "namespace": "john-preview-che",
			  "selfLink": "/api/v1/namespaces/john-preview-che/serviceaccounts/che",
			  "uid": "f9dfcc84-2cfa-11e8-a71f-024db754f2d2",
			  "resourceVersion": "117908057",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "labels": {
				"app": "fabric8-tenant-che-mt",
				"group": "io.fabric8.tenant.packages",
				"provider": "fabric8",
				"version": "2.0.82"
			  }
			},
			"secrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  },
			  {
				"name": "che-token-x6x6x"
			  }
			],
			"imagePullSecrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  }
			]
		  }
		  `
	} else if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/john-preview-che/secrets/che-token-x6x6x") {
		res = `{
			"kind": "Secret",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che-token-x6x6x",
			  "namespace": "john-preview-che",
			  "selfLink": "/api/v1/namespaces/john-preview-che/secrets/che-token-x6x6x",
			  "uid": "f9e3f05e-a71f-024db754f2d2",
			  "resourceVersion": "117908051",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "annotations": {
				"kubernetes.io/service-account.name": "che",
				"kubernetes.io/service-account.uid": "f9dfcc84-xxx-024db754f2d2"
			  }
			},
			"data": {
			  "ca.crt": "xxxxx=",
			  "namespace": "xxxxx==",
			  "service-ca.crt": "xxxxx=",
			  "token": "1000_che_secret"
			},
			"type": "kubernetes.io/service-account-token"
		  }`
	}
	rw.Write([]byte(res))
}

func serverOSIORequest(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, varifyHandler)
	}
}

func varifyHandler(rw http.ResponseWriter, req *http.Request) {
	expectedTarget := cheDataTables[currCheTestInd].expectedTarget
	actualTarget := req.Header.Get("Target")
	if !strings.HasSuffix(actualTarget, expectedTarget) {
		rw.Header().Set("err", fmt.Sprintf("Target was incorrect, want:%s, got:%s", expectedTarget, actualTarget))
		return
	}
	expectedToken := cheDataTables[currCheTestInd].expectedToken
	actualToken := req.Header.Get(Authorization)
	if !strings.HasSuffix(actualToken, expectedToken) {
		rw.Header().Set("err", fmt.Sprintf("Token was incorrect, want:%s, got:%s", expectedToken, actualToken))
		return
	}
}

func startServer(url string, handler func(w http.ResponseWriter, r *http.Request)) (ts *httptest.Server) {
	if listener, err := net.Listen("tcp", strings.Replace(url, "/", "", -1)); err != nil {
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

func testSrvAccToken(token string) (bool, error) {
	return true, nil
}
