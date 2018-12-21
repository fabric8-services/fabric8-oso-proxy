package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containous/traefik/integration/common"
	"github.com/containous/traefik/integration/try"
	"github.com/containous/traefik/log"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/go-check/check"
	checker "github.com/vdemeester/shakers"
)

type OSIOMiddlewareSuite struct{ BaseSuite }

func (s *OSIOMiddlewareSuite) TestOSIO(c *check.C) {
	// configure OSIO
	os.Setenv("TENANT_URL", common.TenantURL)
	os.Setenv("AUTH_URL", common.AuthURL)
	os.Setenv("SERVICE_ACCOUNT_ID", "any-id")
	os.Setenv("SERVICE_ACCOUNT_SECRET", "anysecret")
	os.Setenv("AUTH_TOKEN_KEY", "secret")
	tenantServer := common.StartServer(9090, common.ServeTenantRequest)
	defer tenantServer.Close()
	authServer := common.StartServer(9091, common.ServerAuthRequest(serverMiddlewareCluster))
	defer authServer.Close()

	// Start OSIO servers
	ts1 := common.StartServer(8081, nil)
	defer ts1.Close()
	ts2 := common.StartServer(8082, nil)
	defer ts2.Close()

	// Start Traefik
	cmd, display := s.traefikCmd(withConfigFile("fixtures/osio_middleware_config.toml"))
	defer display(c)
	err := cmd.Start()
	c.Assert(err, checker.IsNil)
	defer cmd.Process.Kill()
	defer checkTraefikLogs(c, cmd.Stdout)

	// Make some requests
	req, _ := http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "1111"})))
	res, err := try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("Test server found, res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8081)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "2222"})))
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("Test server found, res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8082)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "3333"})))
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("Test no server found, res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 404)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "4444"})))
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("Test user not authorized, res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 401)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	// req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "1111"})))
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("Test no Authorization found, res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 401)

	req, _ = http.NewRequest("OPTIONS", "http://127.0.0.1:8000/test", nil)
	// req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "1111"})))
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("Test default server found for OPTIONS request, res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8081)

	url := fmt.Sprintf("http://127.0.0.1:8000/test?access_token=%s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "1111"}))
	req, _ = http.NewRequest("GET", url, nil)
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("Test server found, res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8081)
}

func checkPort(c *check.C, res *http.Response, expectedPort int) {
	c.Assert(res, check.NotNil)
	body := make([]byte, res.ContentLength)
	res.Body.Read(body)
	if !strings.HasSuffix(string(body), strconv.Itoa(expectedPort)) {
		c.Errorf("Served by wrong port, want:%d, got:%s", expectedPort, string(body))
	}
}

func serverMiddlewareCluster() string {
	return common.TwoClusterData()
}

func checkTraefikLogs(c *check.C, w io.Writer) {
	if logBytes, ok := w.(*bytes.Buffer); ok {
		logStr := logBytes.String()
		if strings.Contains(logStr, "access_token") {
			c.Errorf("'access_token' found in logs")
		}
	}
}
