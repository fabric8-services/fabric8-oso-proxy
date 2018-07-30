package osio

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*

user_list,	user_id,	space_id,	osio_token,	oso_token
=========================================================
user1, 		u1111, 		s1111,		ta1111,		tb1111
user2, 		u2222, 		s2222,		ta2222, 	tb2222
user3,		u3333,		s3333,		ta3333,		tb3333
=========================================================

- only one cluster cluster1.com
- u2222 is collaborator of u1111.s1111
- u3333 is NOT collaborator of u1111.s1111
- u1111 namespace 'stage' is already created while namespace 'run' is not created yet

*/

type testParamCtx struct {
	tables  []testParamData
	currInd int
}

type testParamData struct {
	inputPath    string
	inputAuth    string
	expectedCode int
	expectedPath string
}

var tokenMap = map[string]string{
	"ta1111": "tb1111",
	"ta2222": "tb2222",
	"ta3333": "tb3333",
}

var paramCtx = testParamCtx{tables: []testParamData{
	{
		"/api/v1/namespaces/ns;type=stage/pods/p;space=s1111?w=true",
		"ta2222",
		http.StatusOK,
		"/api/v1/namespaces/u1111-preview-stage/pods",
	},
	{
		"/api/v1/namespaces/ns;type=run/pods/p;space=s1111?w=false",
		"ta2222",
		http.StatusNotFound,
		"",
	},
	{
		"/api/v1/namespaces/ns;type=user/pods/p;space=s1111?w=false",
		"ta2222",
		http.StatusOK,
		"/api/v1/namespaces/u1111-preview-user/pods",
	},
	{
		"/api/v1/namespaces/ns;type=run/pods/p;space=s1111?w=true",
		"ta3333",
		http.StatusForbidden,
		"",
	},
	{
		"/api/v1/namespaces/ns;type=stage/pods?w=true",
		"ta1111",
		http.StatusOK,
		"/api/v1/namespaces/u1111-preview-stage/pods",
	},
	{
		"/api/v1/namespaces/ns;type=run/pods?w=false",
		"ta1111",
		http.StatusNotFound,
		"",
	},
}}

func tTestParam(t *testing.T) {
	os.Setenv("AUTH_TOKEN_KEY", "foo")

	tenantServer := createServer(paramCtx.serveTenantRequest)
	defer tenantServer.Close()
	authServer := createServer(paramCtx.serverAuthRequest)
	defer authServer.Close()

	tenantURL := "http://" + tenantServer.Listener.Addr().String() + "/"
	authURL := "http://" + authServer.Listener.Addr().String() + "/"
	srvAccID := "sa1"
	srvAccSecret := "secret"

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret)
	osio.RequestTokenType = paramCtx.testTokenTypeLocator
	osioServer := createServer(serverOSIORequest(osio, paramCtx.varifyHandler))
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range paramCtx.tables {
		paramCtx.currInd = ind
		currReqPath := table.inputPath
		currReqAuth := table.inputAuth
		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set("Authorization", "Bearer "+currReqAuth)

		res, err := http.DefaultClient.Do(req)
		assert.Nil(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, table.expectedCode, res.StatusCode)
	}
}

func (t testParamCtx) serveTenantRequest(rw http.ResponseWriter, req *http.Request) {
	var res string
	token, space, nsType, w := getParams(req)

	if token == "ta2222" && space == "s1111" && nsType == "stage" && w == "true" {
		res = fmt.Sprintf(`{
			"data": {
				"attributes": {
					"namespaces": [
						{
							"name": "u1111-preview-stage",
							"type": "stage",
							"space" : "s1111",
							"cluster-url": "http://api.cluster1.com"
						}
					]
				}
			}
		}`)
	} else if token == "ta2222" && space == "s1111" && nsType == "user" && w == "false" {
		res = fmt.Sprintf(`{
			"data": {
				"attributes": {
					"namespaces": [
						{
							"name": "u1111-preview-stage",
							"type": "user",
							"space" : "s1111",
							"cluster-url": "http://api.cluster1.com"
						}
					]
				}
			}
		}`)
	} else if token == "ta3333" && space == "s1111" {
		http.Error(rw, "", http.StatusForbidden)
		return
	}

	if res == "" {
		http.Error(rw, "", http.StatusNotFound)
		return
	}
	rw.Write([]byte(res))
}

func (t testParamCtx) serverAuthRequest(rw http.ResponseWriter, req *http.Request) {
	osioToken := getTokenFromRequest(req)
	osoToken := tokenMap[osioToken]
	if osoToken == "" {
		http.Error(rw, "", http.StatusNotFound)
		return
	}
	res := fmt.Sprintf(`{"token_type":"bearer", "scope":"user","access_token":"%s"}`, osoToken)
	rw.Write([]byte(res))
}

func (t testParamCtx) testTokenTypeLocator(token string) (TokenType, error) {
	return UserToken, nil
}

func (t testParamCtx) varifyHandler(rw http.ResponseWriter, req *http.Request) {
	expectedPath := paramCtx.tables[paramCtx.currInd].expectedPath
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

func getParams(req *http.Request) (token, space, nsType, w string) {
	token = getTokenFromRequest(req)
	q := req.URL.Query()
	space = q.Get("space")
	nsType = q.Get("type")
	w = q.Get("w")
	return
}
