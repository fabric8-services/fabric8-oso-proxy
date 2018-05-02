package middlewares

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/containous/traefik/integration/common"
	"github.com/stretchr/testify/assert"
)

type testRedirectData struct {
	inputPath   string
	osioToken   string
	redirectURL string
}

var currTestInd int

var tables2 = []testRedirectData{
	// {
	// 	"/console/project/john-preview",
	// 	"1111",
	// 	"http://localhost:9090/console/project/john-preview",
	// },
	// {
	// 	"/console/project/sara-preview",
	// 	"2222",
	// 	"http://localhost:9091/console/project/sara-preview",
	// },
	{
		"/logs/project/john-preview?tab=logs",
		"1111",
		"http://localhost:9090/console/project/john-preview?tab=logs",
	},
	// {
	// 	"/log/project/sara-preview",
	// 	"2222",
	// 	"http://localhost:9091/console/project/sara-preview",
	// },
}

// TODO: NT: incomplete, need to add test for logs
func TestBasic(t *testing.T) {
	witServer := createServer(serverWITRequest2)
	defer witServer.Close()
	authServer := createServer(serverAuthRequest2)
	defer authServer.Close()
	witURL := "http://" + witServer.Listener.Addr().String() + "/"
	authURL := "http://" + authServer.Listener.Addr().String() + "/"

	osio := NewOSIOAuth(witURL, authURL)
	osioServer := createServer(serverOSIORequest(osio))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	osoServer1 := common.StartServer(9090, serverOSORequest)
	defer osoServer1.Close()
	osoServer2 := common.StartServer(9091, serverOSORequest)
	defer osoServer2.Close()

	for ind, table := range tables2 {
		currTestInd = ind

		currReqPath := table.inputPath
		currOsioToken := table.osioToken

		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set("Authorization", "Bearer "+currOsioToken)
		res, _ := http.DefaultClient.Do(req)
		assert.NotNil(t, res)
		if res != nil {
			h := res.Header
			actual := h.Get("Location")
			assert.NotNil(t, actual)
			assert.Equal(t, table.redirectURL, actual)
		}
	}
}

func serverWITRequest2(rw http.ResponseWriter, req *http.Request) {
	consoleURL := ""
	authHeader := req.Header.Get("Authorization")

	switch {
	case strings.HasSuffix(authHeader, "1111"):
		consoleURL = "http://localhost:9090/console"
	case strings.HasSuffix(authHeader, "2222"):
		consoleURL = "http://localhost:9091/console/"
	}

	res := fmt.Sprintf(`{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "myuser-preview-stage",
						"cluster-console-url": "%s",
						"cluster-logging-url": "%s"
					}
				]
			}
		}
	}`, consoleURL, consoleURL)
	rw.Write([]byte(res))
}

func serverAuthRequest2(rw http.ResponseWriter, req *http.Request) {
}

func serverOSIORequest2(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, nopHandler)
	}
}

func nopHandler(rw http.ResponseWriter, req *http.Request) {
}

func serverOSORequest(rw http.ResponseWriter, req *http.Request) {
	reqHeader := req.Header
	resHeader := rw.Header()
	fmt.Println(reqHeader, resHeader)
}
