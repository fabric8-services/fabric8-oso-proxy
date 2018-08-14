package osio

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCacheCtx struct {
	tables         []testCacheData
	currInd        int
	currTenantCall int
	currAuthCall   int
}

type testCacheData struct {
	id              int
	tenantCallCount int
	authCallCount   int
}

var cacheCtx = testCacheCtx{tables: []testCacheData{
	{1001, 1, 1},
	{1002, 2, 2},
	{2001, 3, 3},
	{1002, 3, 3},
	{1003, 4, 4},
	{2002, 5, 5},
	{2001, 5, 5},
}}

func TestCache(t *testing.T) {
	tenantServer := createServer(cacheCtx.serveTenantRequest)
	defer tenantServer.Close()
	authServer := createServer(cacheCtx.serverAuthRequest)
	defer authServer.Close()

	tenantURL := "http://" + tenantServer.Listener.Addr().String() + "/"
	authURL := "http://" + authServer.Listener.Addr().String() + "/"
	srvAccID := "sa1"
	srvAccSecret := "secret"
	authTokenKey := "foo"

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey)
	osio.RequestTokenType = cacheCtx.testTokenTypeLocator
	osioServer := createServer(serverOSIORequest(osio, cacheCtx.varifyHandler))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range cacheCtx.tables {
		// NOTE: do NOT run in parallel
		t.Run(fmt.Sprintf("test-%d", (ind+1)), func(t *testing.T) {
			cacheCtx.currInd = ind
			currReqPath := fmt.Sprintf("/path%d", cacheCtx.currInd)
			currReqToken := fmt.Sprintf("ta-%d", table.id)
			req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
			req.Header.Set("Authorization", "Bearer "+currReqToken)

			res, err := http.DefaultClient.Do(req)
			assert.Nil(t, err)
			assert.NotNil(t, res)
			errMsg := res.Header.Get("err")
			assert.Empty(t, errMsg, errMsg)
			assert.Equal(t, table.tenantCallCount, cacheCtx.currTenantCall)
			assert.Equal(t, table.authCallCount, cacheCtx.currAuthCall)
		})
	}
}

func (t testCacheCtx) serverOSIORequest(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, cacheCtx.varifyHandler)
	}
}

func (t testCacheCtx) serveTenantRequest(rw http.ResponseWriter, req *http.Request) {
	cacheCtx.currTenantCall++

	token := req.Header.Get(Authorization)
	id, _ := strconv.Atoi(strings.Split(token, "-")[1])
	nsName := fmt.Sprintf("u-%d-preview", id)
	cluster := fmt.Sprintf("http://api.cluster%d.com", (id / 1000))

	res := fmt.Sprintf(`{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "%s",
						"type": "user",
						"cluster-url": "%s"
					}
				]
			}
		}
	}`, nsName, cluster)
	rw.Write([]byte(res))
}

func (t testCacheCtx) serverAuthRequest(rw http.ResponseWriter, req *http.Request) {
	cacheCtx.currAuthCall++

	token := req.Header.Get(Authorization)
	id, _ := strconv.Atoi(strings.Split(token, "-")[1])
	osoToken := fmt.Sprintf("tb-%d", id)

	res := fmt.Sprintf(`{"token_type":"bearer", "scope":"user","access_token":"%s"}`, osoToken)
	rw.Write([]byte(res))
}

func (t testCacheCtx) varifyHandler(rw http.ResponseWriter, req *http.Request) {
	expectedTarget := fmt.Sprintf("http://api.cluster%d.com", (cacheCtx.tables[cacheCtx.currInd].id / 1000))
	actualTarget := req.Header.Get("Target")
	if expectedTarget != actualTarget {
		rw.Header().Set("err", fmt.Sprintf("Target was incorrect, want:%s, got:%s", expectedTarget, actualTarget))
		return
	}

	expectedAuth := fmt.Sprintf("Bearer tb-%d", cacheCtx.tables[cacheCtx.currInd].id)
	actualAuth := req.Header.Get(Authorization)
	if expectedAuth != actualAuth {
		rw.Header().Set("err", fmt.Sprintf("Authorization was incorrect, want:%s, got:%s", expectedAuth, actualAuth))
		return
	}

	expectedPath := fmt.Sprintf("/path%d", cacheCtx.currInd)
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

func (t testCacheCtx) testTokenTypeLocator(token string) (TokenType, error) {
	return UserToken, nil
}
