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

type testCheCtx struct {
	tables          []testCheData
	authCallCount   int
	tenantCallCount int
	currInd         int
}

type testCheData struct {
	inputPath      string
	userID         string
	expectedTarget string
	expectedToken  string
}

var cheCtx = testCheCtx{tables: []testCheData{
	{
		"/api",
		"john",
		"127.0.0.1:9091",
		"1000_che_secret",
	},
	{
		"/api",
		"john",
		"127.0.0.1:9091",
		"1000_che_secret",
	},
}}

func TestChe(t *testing.T) {
	os.Setenv("AUTH_TOKEN_KEY", "foo")

	authServer := cheCtx.createServer(cheCtx.serveAuthRequest)
	defer authServer.Close()
	tenantServer := cheCtx.createServer(cheCtx.serveTenantRequest)
	defer tenantServer.Close()

	authURL := "http://" + authServer.Listener.Addr().String()
	tenantURL := "http://" + tenantServer.Listener.Addr().String()
	srvAccID := "sa1"
	srvAccSecret := "secret"

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret)
	osio.CheckSrvAccToken = testSrvAccToken
	osioServer := cheCtx.createServer(cheCtx.serverOSIORequest(osio))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range cheCtx.tables {
		cheCtx.currInd = ind
		cluster := cheCtx.startServer(table.expectedTarget, cheCtx.serveClusterRequest)

		currReqPath := table.inputPath
		cheSAToken := "1000_che_sa_token"

		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set(Authorization, "Bearer "+cheSAToken)
		req.Header.Set(UserIDHeader, table.userID)
		res, _ := http.DefaultClient.Do(req)
		assert.NotNil(t, res)
		err := res.Header.Get("err")
		assert.Empty(t, err, err)

		cluster.Close()
	}
	expecteTenantCalls := 1
	assert.Equal(t, expecteTenantCalls, cheCtx.tenantCallCount, "Number of time Tenant server called was incorrect, want:%d, got:%d", expecteTenantCalls, cheCtx.tenantCallCount)
}

func (t testCheCtx) createServer(handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	return httptest.NewServer(mux)
}

func (t testCheCtx) serveAuthRequest(rw http.ResponseWriter, req *http.Request) {
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

func (t testCheCtx) serveTenantRequest(rw http.ResponseWriter, req *http.Request) {
	cheCtx.tenantCallCount++
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

func (t testCheCtx) serveClusterRequest(rw http.ResponseWriter, req *http.Request) {
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
			  "token": "MTAwMF9jaGVfc2VjcmV0"
			},
			"type": "kubernetes.io/service-account-token"
		  }`
	}
	rw.Write([]byte(res))
}

func (t testCheCtx) serverOSIORequest(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, cheCtx.varifyHandler)
	}
}

func (t testCheCtx) varifyHandler(rw http.ResponseWriter, req *http.Request) {
	expectedTarget := cheCtx.tables[cheCtx.currInd].expectedTarget
	actualTarget := req.Header.Get("Target")
	if !strings.HasSuffix(actualTarget, expectedTarget) {
		rw.Header().Set("err", fmt.Sprintf("Target was incorrect, want:%s, got:%s", expectedTarget, actualTarget))
		return
	}
	expectedToken := cheCtx.tables[cheCtx.currInd].expectedToken
	actualToken := req.Header.Get(Authorization)
	if !strings.HasSuffix(actualToken, expectedToken) {
		rw.Header().Set("err", fmt.Sprintf("Token was incorrect, want:%s, got:%s", expectedToken, actualToken))
		return
	}
}

func (t testCheCtx) startServer(url string, handler func(w http.ResponseWriter, r *http.Request)) (ts *httptest.Server) {
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
