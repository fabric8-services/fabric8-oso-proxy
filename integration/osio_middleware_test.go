package integration

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containous/traefik/integration/common"
	"github.com/containous/traefik/integration/try"
	"github.com/containous/traefik/log"
	"github.com/go-check/check"
	checker "github.com/vdemeester/shakers"
)

type OSIOMiddlewareSuite struct{ BaseSuite }

func (s *OSIOMiddlewareSuite) TestOSIO(c *check.C) {
	// configure OSIO
	os.Setenv("WIT_URL", common.WitURL)
	os.Setenv("AUTH_URL", common.AuthURL)
	os.Setenv("SERVICE_ACCOUNT_ID", "any-id")
	os.Setenv("SERVICE_ACCOUNT_SECRET", "anysecret")
	witServer := common.StartOSIOServer(9090, common.ServeWITRequest)
	defer witServer.Close()
	authServer := common.StartOSIOServer(9091, common.ServerAuthRequest(serverMiddlewareCluster))
	defer authServer.Close()

	// Start Traefik
	cmd, display := s.traefikCmd(withConfigFile("fixtures/osio_middleware_config.toml"))
	defer display(c)
	err := cmd.Start()
	c.Assert(err, checker.IsNil)
	defer cmd.Process.Kill()

	// Start OSIO servers
	ts1 := common.StartOSIOServer(8081, nil)
	defer ts1.Close()
	ts2 := common.StartOSIOServer(8082, nil)
	defer ts2.Close()

	// Make some requests
	req, _ := http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 1111")
	res, err := try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req1 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8081)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 2222")
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req2 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8082)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 3333")
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req3 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 404)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 4444")
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req4 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 401)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	// req.Header.Add("Authorization", "Bearer 1111")
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req5 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 401)

	req, _ = http.NewRequest("OPTIONS", "http://127.0.0.1:8000/test", nil)
	// req.Header.Add("Authorization", "Bearer 1111")
	res, err = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req6 res.StatusCode=%d", res.StatusCode)
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
