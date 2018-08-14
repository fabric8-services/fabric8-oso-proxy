package osio

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*

user_list,	space_id,	osio_token,	oso_token
==============================================
user1, 		space11,	ta-1111,	tb1111
user2, 		space21,	ta-2222,	tb2222
user3,		space31,	ta-3333,	tb3333
==============================================

- all user always have 'user' namespace
- user2 is collaborator of user1.space11 space
- user3 is NOT collaborator of user1.space11 space
- user1 namespace 'user' is already created while namespace 'run' and 'stage' is not created yet
- creation of new namespace uses cluster of 'user' namespace

*/

type testParamCtx struct {
	posTables []testParamData
	negTables []testParamNegData
	currInd   int
}

type testParamData struct {
	inputPath      string
	inputToken     string
	expectedCode   int
	expectedPath   string
	expectedTarget string
	expectedToken  string
}

type testParamNegData struct {
	inputPath    string
	inputToken   string
	expectedCode int
}

type paramUser struct {
	username      string
	osoToken      string
	spaces        map[string]bool
	collaborators map[string]bool
	namespaces    map[string]string // "user" namespace will always there
}

var paramSystem map[string]*paramUser

func setup() {
	paramSystem = make(map[string]*paramUser)

	paramSystem["ta-1111"] = &paramUser{
		username:      "user1-preview",
		osoToken:      "tb-1111",
		spaces:        map[string]bool{"space11": true},
		collaborators: map[string]bool{"user2-preview": true},
		namespaces: map[string]string{
			"user": "cluster1.com",
		},
	}

	paramSystem["ta-2222"] = &paramUser{
		username:      "user2-preview",
		osoToken:      "tb-2222",
		spaces:        map[string]bool{"space21": true},
		collaborators: map[string]bool{},
		namespaces: map[string]string{
			"user":  "cluster2.com",
			"stage": "cluster2.com",
		},
	}

	paramSystem["ta-3333"] = &paramUser{
		username:      "user3-preview",
		osoToken:      "tb-3333",
		spaces:        map[string]bool{"space31": true},
		collaborators: map[string]bool{},
		namespaces: map[string]string{
			"user":  "cluster1.com",
			"stage": "cluster2.com",
			"run":   "cluster3.com",
		},
	}
}

var paramCtx = testParamCtx{
	posTables: []testParamData{
		{
			"/api/v1/namespaces/ns;type=user;space=space11;w=true/pods",
			"ta-2222",
			http.StatusOK,
			"/api/v1/namespaces/user1-preview/pods",
			"https://api.cluster1.com",
			"tb-2222",
		},
		{
			"/api/v1/namespaces/ns;type=stage;space=space11;w=true/pods",
			"ta-2222",
			http.StatusOK,
			"/api/v1/namespaces/user1-preview-stage/pods",
			"https://api.cluster1.com",
			"tb-2222",
		},
		{
			"/api/v1/namespaces/ns;type=stage;space=space21;w=false/pods",
			"ta-2222",
			http.StatusOK,
			"/api/v1/namespaces/user2-preview-stage/pods",
			"https://api.cluster2.com",
			"tb-2222",
		},
		{
			"/api/v1/namespaces/ns;type=user;w=true/pods",
			"ta-1111",
			http.StatusOK,
			"/api/v1/namespaces/user1-preview/pods",
			"https://api.cluster1.com",
			"tb-1111",
		},
		{
			"/api/v1/namespaces/user1-preview/pods",
			"ta-1111",
			http.StatusOK,
			"/api/v1/namespaces/user1-preview/pods",
			"https://api.cluster1.com",
			"tb-1111",
		},
		{
			"/api/v1/namespaces/ns;type=user;space=space31;w=true/routes",
			"ta-3333",
			http.StatusOK,
			"/api/v1/namespaces/user3-preview/routes",
			"https://api.cluster1.com",
			"tb-3333",
		},
		{
			"/api/v1/namespaces/ns;type=stage;space=space31;w=true/deployments",
			"ta-3333",
			http.StatusOK,
			"/api/v1/namespaces/user3-preview-stage/deployments",
			"https://api.cluster2.com",
			"tb-3333",
		},
		{
			"/api/oapi/v1/namespaces/ns;type=run;space=space31;w=true/buildconfigs",
			"ta-3333",
			http.StatusOK,
			"/oapi/v1/namespaces/user3-preview-run/buildconfigs",
			"https://api.cluster3.com",
			"tb-3333",
		},
	},
	negTables: []testParamNegData{
		{
			"/api/v1/namespaces/ns;type=run;space=space11;w=false/pods",
			"ta-2222",
			http.StatusNotFound,
		},
		{
			"/api/v1/namespaces/ns;type=user;space=space11;w=true/pods",
			"ta-3333",
			http.StatusForbidden,
		},
		{
			"/api/v1/namespaces/ns;type=run;w=false/pods",
			"ta-1111",
			http.StatusNotFound,
		},
		{
			"/api/v1/namespaces/ns;w=true/pods",
			"ta-1111",
			http.StatusBadRequest,
		},
		{
			"/api/v1/namespaces/ns;type=user;w=TRUE/pods",
			"ta-1111",
			http.StatusBadRequest,
		},
	},
}

func TestParam(t *testing.T) {
	tenantServer := createServer(paramCtx.serveTenantRequest)
	defer tenantServer.Close()
	authServer := createServer(paramCtx.serverAuthRequest)
	defer authServer.Close()

	tenantURL := "http://" + tenantServer.Listener.Addr().String() + "/"
	authURL := "http://" + authServer.Listener.Addr().String() + "/"
	srvAccID := "sa1"
	srvAccSecret := "secret"
	authTokenKey := "foo"

	runPosTests(t, tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey)
	runNegTests(t, tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey)
}

func runPosTests(t *testing.T, tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey string) {
	setup()

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey)
	osio.RequestTokenType = paramCtx.testTokenTypeLocator
	osioServer := createServer(serverOSIORequest(osio, paramCtx.varifyHandlerPos))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range paramCtx.posTables {
		t.Run(fmt.Sprintf("test-pos-%d", (ind+1)), func(t *testing.T) {
			paramCtx.currInd = ind
			currReqPath := table.inputPath
			currReqToken := table.inputToken
			req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
			req.Header.Set("Authorization", "Bearer "+currReqToken)

			res, err := http.DefaultClient.Do(req)
			assert.Nil(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, table.expectedCode, res.StatusCode)
			errMsg := res.Header.Get("err")
			assert.Empty(t, errMsg, errMsg)
		})
	}
}

func runNegTests(t *testing.T, tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey string) {
	setup()

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret, authTokenKey)
	osio.RequestTokenType = paramCtx.testTokenTypeLocator
	osioServer := createServer(serverOSIORequest(osio, paramCtx.varifyHandlerNeg))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range paramCtx.negTables {
		t.Run(fmt.Sprintf("test-neg-%d", (ind+1)), func(t *testing.T) {
			paramCtx.currInd = ind
			currReqPath := table.inputPath
			currReqToken := table.inputToken
			req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
			req.Header.Set("Authorization", "Bearer "+currReqToken)

			res, err := http.DefaultClient.Do(req)
			assert.Nil(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, table.expectedCode, res.StatusCode)
			errMsg := res.Header.Get("err")
			assert.Empty(t, errMsg, errMsg)
		})
	}
}

func (t testParamCtx) serveTenantRequest(rw http.ResponseWriter, req *http.Request) {
	var res string
	token, space, nsType, w := getParams(req)

	if paramSystem[token] == nil {
		http.Error(rw, "", http.StatusUnauthorized) // not in system, so unauthorized
		return
	}

	if nsType == "" {
		nsType = "user" // default choose "user" namespace
	}
	if w == "" {
		w = "true" // default create namespace if not available
	}

	var owner *paramUser
	if space != "" {
		var osioToken string
		osioToken, owner = findSpaceOwner(space)
		if osioToken != token {
			collaborator := paramSystem[token]
			if !owner.collaborators[collaborator.username] {
				http.Error(rw, "", http.StatusForbidden) // not collaborator, so forbidden
				return
			}
		}
	} else {
		owner = paramSystem[token]
	}

	if w == "false" {
		if owner.namespaces[nsType] == "" {
			http.Error(rw, "", http.StatusNotFound)
			return
		}
	} else if w == "true" {
		if owner.namespaces[nsType] == "" { // not found then create one
			owner.namespaces[nsType] = owner.namespaces["user"] // use cluster of "user" namespace
		}
	}

	nsName := owner.username
	if nsType != "user" {
		nsName = nsName + "-" + nsType
	}
	clusterURL := owner.namespaces[nsType]

	res = fmt.Sprintf(`{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "%s",
						"type": "%s",
						"cluster-url": "https://api.%s"
					}
				]
			}
		}
	}`, nsName, nsType, clusterURL)

	rw.Write([]byte(res))
}

func (t testParamCtx) serverAuthRequest(rw http.ResponseWriter, req *http.Request) {
	osioToken := getTokenFromRequest(req)

	user := paramSystem[osioToken]
	if user == nil {
		http.Error(rw, "", http.StatusUnauthorized)
		return
	}

	osoToken := user.osoToken
	res := fmt.Sprintf(`{"token_type":"bearer", "scope":"user","access_token":"%s"}`, osoToken)
	rw.Write([]byte(res))
}

func (t testParamCtx) testTokenTypeLocator(token string) (TokenType, error) {
	return UserToken, nil
}

func (t testParamCtx) varifyHandlerPos(rw http.ResponseWriter, req *http.Request) {
	expectedTarget := paramCtx.posTables[paramCtx.currInd].expectedTarget
	actualTarget := req.Header.Get("Target")
	if expectedTarget != actualTarget {
		rw.Header().Set("err", fmt.Sprintf("Target was incorrect, want:%s, got:%s", expectedTarget, actualTarget))
		return
	}

	expectedToken := paramCtx.posTables[paramCtx.currInd].expectedToken
	actualToken := getTokenFromRequest(req)
	if expectedToken != actualToken {
		rw.Header().Set("err", fmt.Sprintf("Token was incorrect, want:%s, got:%s", expectedToken, actualToken))
		return
	}

	expectedPath := paramCtx.posTables[paramCtx.currInd].expectedPath
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

func (t testParamCtx) varifyHandlerNeg(rw http.ResponseWriter, req *http.Request) {
}

func getParams(req *http.Request) (token, space, nsType, w string) {
	token = getTokenFromRequest(req)
	q := req.URL.Query()
	space = q.Get("space")
	nsType = q.Get("type")
	w = q.Get("w")
	return
}

func findSpaceOwner(space string) (string, *paramUser) {
	for key, value := range paramSystem {
		if value.spaces[space] == true {
			return key, value
		}
	}
	return "", nil
}
