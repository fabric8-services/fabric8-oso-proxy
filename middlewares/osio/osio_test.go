package middlewares

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	witCalls  = 0
	authCalls = 0
)

type testData struct {
	inputPath      string
	expectedTarget string
	expectedPath   string
}

var tables = []testData{
	{
		"/",
		"http://api.cluster1.com/",
		"/",
	},
	{
		"/api",
		"http://api.cluster1.com/",
		"/",
	},
	{
		"/api/anything",
		"http://api.cluster1.com/",
		"/anything",
	},
	{
		"/metrics",
		"http://metrics.cluster1.com/",
		"/",
	},
	{
		"/metrics/anything",
		"http://metrics.cluster1.com/",
		"/anything",
	},
	{
		"/restall",
		"http://api.cluster1.com/",
		"/restall",
	},
}

var currTestNo int

func TestBasic(t *testing.T) {
	witServer := createServer(serverWITRequest)
	authServer := createServer(serverAuthRequest)
	witURL := "http://" + witServer.Listener.Addr().String() + "/"
	authURL := "http://" + authServer.Listener.Addr().String() + "/"

	osio := NewOSIOAuth(witURL, authURL)
	osioServer := createServer(serverOSIORequest(osio))
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range tables {
		currTestNo = ind

		currReqPath := table.inputPath
		currOsioToken := "1000"

		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set("Authorization", "Bearer "+currOsioToken)
		res, _ := http.DefaultClient.Do(req)
		err := res.Header.Get("err")
		assert.Empty(t, err, err)
	}

	// validate cache is used
	expectedWITCalls := 2
	expectedAuthCalls := 2
	assert.Equal(t, expectedWITCalls, witCalls, "Number of time WIT server called was incorrect, want:%d, got:%d", expectedWITCalls, witCalls)
	assert.Equal(t, expectedAuthCalls, authCalls, "Number of time Auth server called was incorrect, want:%d, got:%d", expectedAuthCalls, authCalls)
}

func createServer(handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	return httptest.NewServer(mux)
}

func serverWITRequest(rw http.ResponseWriter, req *http.Request) {
	witCalls++
	res := `{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "myuser-preview-stage",
						"cluster-metrics-url": "http://metrics.cluster1.com/",
						"cluster-url": "http://api.cluster1.com/"
					}
				]
			}
		}
	}`
	rw.Write([]byte(res))
}

func serverAuthRequest(rw http.ResponseWriter, req *http.Request) {
	authCalls++
	res := `{"token_type":"bearer", "scope":"user","access_token":"1001"}`
	rw.Write([]byte(res))
}

func serverOSIORequest(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, varifyHandler)
	}
}

func varifyHandler(rw http.ResponseWriter, req *http.Request) {
	expectedTarget := tables[currTestNo].expectedTarget
	actualTarget := req.Header.Get("Target")
	if expectedTarget != actualTarget {
		rw.Header().Set("err", fmt.Sprintf("Target was incorrect, want:%s, got:%s", expectedTarget, actualTarget))
		return
	}

	expectedPath := tables[currTestNo].expectedPath
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
