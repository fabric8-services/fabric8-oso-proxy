package osio

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testRedirectCtx struct {
	table            []testRedirectData
	authCallCount    int
	tenantCallCount  int
	nextHandlerCount int
	currInd          int
}

type testRedirectData struct {
	inputPath    string
	osioToken    string
	expectedHost string
	expectedURL  string
}

var redirectCtx = testRedirectCtx{table: []testRedirectData{
	{
		"/console/project/john-preview",
		"1000",
		"localhost:9090",
		"/console/project/john-preview",
	},
	{
		"/console/project/sara-preview",
		"2000",
		"localhost:9091",
		"/console/project/sara-preview",
	},
	{
		"/logs/project/john-preview?tab=logs",
		"1000",
		"localhost:9090",
		"/console/project/john-preview?tab=logs",
	},
	{
		"/logs/project/sara-preview",
		"2000",
		"localhost:9091",
		"/console/project/sara-preview",
	},
}}

func TestRedirect(t *testing.T) {
	authServer := redirectCtx.createServer(redirectCtx.serveAuthRequest)
	defer authServer.Close()
	tenantServer := redirectCtx.createServer(redirectCtx.serverTenantRequest)
	defer tenantServer.Close()

	authURL := "http://" + authServer.Listener.Addr().String()
	tenantURL := "http://" + tenantServer.Listener.Addr().String()
	srvAccID := "sa1"
	srvAccSecret := "secret"
	authTokenKey := "foo"

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey)
	osio.RequestTokenType = redirectCtx.testTokenTypeLocator
	osioServer := redirectCtx.createServer(redirectCtx.serveOSIORequest(osio))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	osoServer1 := redirectCtx.createServerAtPort(9090, redirectCtx.serveOSORequest)
	defer osoServer1.Close()
	osoServer2 := redirectCtx.createServerAtPort(9091, redirectCtx.serveOSORequest)
	defer osoServer2.Close()

	for ind, table := range redirectCtx.table {
		redirectCtx.currInd = ind
		currReqPath := table.inputPath
		currOsioToken := table.osioToken

		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set("Authorization", "Bearer "+currOsioToken)
		res, _ := http.DefaultClient.Do(req)
		assert.NotNil(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.NotNil(t, res.Body)
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		assert.Nil(t, err)
		actualBody := string(body)
		assert.Empty(t, actualBody, actualBody)
	}

	expecteTenantCalls := 2
	assert.Equal(t, expecteTenantCalls, redirectCtx.tenantCallCount, "Number of time Tenant server called was incorrect, want:%d, got:%d", expecteTenantCalls, redirectCtx.tenantCallCount)
	expecteAuthCalls := 2
	assert.Equal(t, expecteAuthCalls, redirectCtx.authCallCount, "Number of time Auth server called was incorrect, want:%d, got:%d", expecteAuthCalls, redirectCtx.authCallCount)
	expecteNextHandlerCalls := 0
	assert.Equal(t, expecteNextHandlerCalls, redirectCtx.nextHandlerCount, "Number of time Next handler called was incorrect, want:%d, got:%d", expecteNextHandlerCalls, redirectCtx.nextHandlerCount)
}

func (t testRedirectCtx) createServer(handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	return httptest.NewServer(mux)
}

func (t testRedirectCtx) createServerAtPort(port int, handler func(w http.ResponseWriter, r *http.Request)) (ts *httptest.Server) {
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

func (t testRedirectCtx) serverTenantRequest(rw http.ResponseWriter, req *http.Request) {
	redirectCtx.tenantCallCount++

	user := ""
	consoleURL := ""
	authHeader := req.Header.Get("Authorization")

	switch {
	case strings.HasSuffix(authHeader, "1000"):
		user = "1000"
		consoleURL = "http://localhost:9090/console"
	case strings.HasSuffix(authHeader, "2000"):
		user = "2000"
		consoleURL = "http://localhost:9091/console/"
	}

	res := fmt.Sprintf(`{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "myuser%s-preview-stage",
						"type": "user",
						"cluster-console-url": "%s",
						"cluster-logging-url": "%s"
					}
				]
			}
		}
	}`, user, consoleURL, consoleURL)
	rw.Write([]byte(res))
}

func (t testRedirectCtx) serveAuthRequest(rw http.ResponseWriter, req *http.Request) {
	redirectCtx.authCallCount++

	token := "1001"
	if strings.HasSuffix(req.Header.Get(Authorization), "1000") {
		token = "1001"
	} else if strings.HasSuffix(req.Header.Get(Authorization), "2000") {
		token = "2001"
	}
	res := fmt.Sprintf(`{"token_type":"bearer", "scope":"user","access_token":"%s"}`, token)
	rw.Write([]byte(res))
}

func (t testRedirectCtx) serveOSIORequest(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, nopHandler)
	}
}

func (t testRedirectCtx) serveOSORequest(rw http.ResponseWriter, req *http.Request) {
	expectedHost := redirectCtx.table[redirectCtx.currInd].expectedHost
	actualHost := req.Host
	if expectedHost != actualHost {
		err := fmt.Sprintf("Host was incorrect, want:%s, got:%s", expectedHost, actualHost)
		rw.Write([]byte(err))
		return
	}
	expectedURL := redirectCtx.table[redirectCtx.currInd].expectedURL
	actualURL := req.URL.Path
	if req.URL.RawQuery != "" {
		actualURL = actualURL + "?" + req.URL.RawQuery
	}
	if expectedURL != actualURL {
		err := fmt.Sprintf("URL was incorrect, want:%s, got:%s", expectedURL, actualURL)
		rw.Write([]byte(err))
		return
	}
}

func (t testRedirectCtx) testTokenTypeLocator(token string) (TokenType, error) {
	return UserToken, nil
}

func nopHandler(rw http.ResponseWriter, req *http.Request) {
	redirectCtx.nextHandlerCount++
}
