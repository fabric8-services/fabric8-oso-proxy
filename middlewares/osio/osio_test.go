package middlewares

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testMiddlewareCtx struct {
	tables          []testMiddlewareData
	authCallCount   int
	tenantCallCount int
	currInd         int
}

type testMiddlewareData struct {
	inputPath      string
	inputAuth      string
	expectedTarget string
	expectedAuth   string
	expectedPath   string
}

var mwCtx = testMiddlewareCtx{tables: []testMiddlewareData{
	{
		"/",
		"Bearer 1000",
		"http://api.cluster1.com",
		"Bearer 1001",
		"/",
	},
	{
		"/api",
		"Bearer 1000",
		"http://api.cluster1.com",
		"Bearer 1001",
		"/",
	},
	{
		"/api/anything",
		"Bearer 1000",
		"http://api.cluster1.com",
		"Bearer 1001",
		"/anything",
	},
	{
		"/metrics",
		"Bearer 1000",
		"http://metrics.cluster1.com",
		"Bearer 1001",
		"/",
	},
	{
		"/metrics/anything",
		"Bearer 1000",
		"http://metrics.cluster1.com",
		"Bearer 1001",
		"/anything",
	},
	{
		"/restall",
		"Bearer 1000",
		"http://api.cluster1.com",
		"Bearer 1001",
		"/restall",
	},
	{
		"/api",
		"Bearer 2000",
		"http://api.cluster1.com",
		"Bearer 2001",
		"/",
	},
	{
		"/metrics",
		"Bearer 2000",
		"http://metrics.cluster1.com",
		"Bearer 2001",
		"/",
	},
}}

func TestMiddleware(t *testing.T) {
	os.Setenv("AUTH_TOKEN_KEY", "foo")

	tenantServer := mwCtx.createServer(mwCtx.serveTenantRequest)
	defer tenantServer.Close()
	authServer := mwCtx.createServer(mwCtx.serverAuthRequest)
	defer authServer.Close()

	tenantURL := "http://" + tenantServer.Listener.Addr().String() + "/"
	authURL := "http://" + authServer.Listener.Addr().String() + "/"
	srvAccID := "sa1"
	srvAccSecret := "secret"

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret)
	osio.CheckSrvAccToken = mwCtx.testSrvAccToken
	osioServer := mwCtx.createServer(mwCtx.serverOSIORequest(osio))
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range mwCtx.tables {
		mwCtx.currInd = ind

		currReqPath := table.inputPath
		currAuth := table.inputAuth

		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set("Authorization", currAuth)
		res, _ := http.DefaultClient.Do(req)
		err := res.Header.Get("err")
		assert.Empty(t, err, err)
	}

	// validate cache is used
	expectedTenantCalls := 2
	expectedAuthCalls := 2
	assert.Equal(t, expectedTenantCalls, mwCtx.tenantCallCount, "Number of time Tenant server called was incorrect, want:%d, got:%d", expectedTenantCalls, mwCtx.tenantCallCount)
	assert.Equal(t, expectedAuthCalls, mwCtx.authCallCount, "Number of time Auth server called was incorrect, want:%d, got:%d", expectedAuthCalls, mwCtx.authCallCount)
}

func (t testMiddlewareCtx) createServer(handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	return httptest.NewServer(mux)
}

func (t testMiddlewareCtx) serveTenantRequest(rw http.ResponseWriter, req *http.Request) {
	mwCtx.tenantCallCount++
	res := `{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "myuser-preview-stage",
						"cluster-metrics-url": "http://metrics.cluster1.com",
						"cluster-url": "http://api.cluster1.com"
					}
				]
			}
		}
	}`
	rw.Write([]byte(res))
}

func (t testMiddlewareCtx) serverAuthRequest(rw http.ResponseWriter, req *http.Request) {
	mwCtx.authCallCount++
	token := "1001"
	if strings.HasSuffix(req.Header.Get(Authorization), "1000") {
		token = "1001"
	} else if strings.HasSuffix(req.Header.Get(Authorization), "2000") {
		token = "2001"
	}
	res := fmt.Sprintf(`{"token_type":"bearer", "scope":"user","access_token":"%s"}`, token)
	rw.Write([]byte(res))
}

func (t testMiddlewareCtx) serverOSIORequest(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, mwCtx.varifyHandler)
	}
}

func (t testMiddlewareCtx) varifyHandler(rw http.ResponseWriter, req *http.Request) {
	expectedTarget := mwCtx.tables[mwCtx.currInd].expectedTarget
	actualTarget := req.Header.Get("Target")
	if expectedTarget != actualTarget {
		rw.Header().Set("err", fmt.Sprintf("Target was incorrect, want:%s, got:%s", expectedTarget, actualTarget))
		return
	}

	expectedAuth := mwCtx.tables[mwCtx.currInd].expectedAuth
	actualAuth := req.Header.Get(Authorization)
	if expectedAuth != actualAuth {
		rw.Header().Set("err", fmt.Sprintf("Authorization was incorrect, want:%s, got:%s", expectedAuth, actualAuth))
		return
	}

	expectedPath := mwCtx.tables[mwCtx.currInd].expectedPath
	actualPath := req.URL.Path
	if expectedPath != actualPath {
		rw.Header().Set("err", fmt.Sprintf("Path was incorrect, want:%s, got:%s", expectedPath, actualPath))
		return
	}

	actualPath = req.RequestURI
	if expectedPath != actualPath {
		rw.Header().Set("err", fmt.Sprintf("Path was incorrect, want:%s, got:%s", expectedPath, actualPath))
		return
	}
}

func (t testMiddlewareCtx) testSrvAccToken(token string) (bool, error) {
	return false, nil
}
