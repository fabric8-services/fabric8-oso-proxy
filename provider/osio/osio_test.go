package osio

import (
	"context"
	"testing"

	"github.com/containous/traefik/safe"

	"github.com/containous/traefik/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testProvider struct {
	Provider
}

func (fp *testProvider) init(configChan chan<- types.ConfigMessage) {
}

func (fp *testProvider) fetchToken() error {
	return nil
}

type testClient struct {
}

func (fc *testClient) callTokenAPI(tokenAPI string, tokenReq *tokenRequest) (*tokenResponse, error) {
	return &tokenResponse{"1111", "bearer"}, nil
}

func (fc *testClient) callClusterAPI(clusterAPIURL string, tokenResp *tokenResponse) (*clusterResponse, error) {
	return &clusterResponse{}, nil
}

func TestScheduleConfigPull(t *testing.T) {
	fp := &testProvider{}
	fp.RefreshSeconds = 100
	fp.client = &testClient{}
	configChan := make(chan types.ConfigMessage)
	pool := safe.NewPool(context.Background())
	fp.schedule(configChan, pool)
	config := <-configChan
	assert.NotNil(t, config)
}

func TestLoadRules(t *testing.T) {
	provider := &Provider{}
	clusters := []clusterData{
		{APIURL: "https://api.starter-us-east-2.openshift.com"},
	}
	clusterResp := &clusterResponse{Clusters: clusters}
	config := provider.loadRules(clusterResp)
	checkConfig(t, config, 2)
}
func TestLoadRulesDefaultChange(t *testing.T) {
	provider := &Provider{}

	tables := []struct {
		clusters      []clusterData
		expectedRules int
		expectedURL   string
	}{
		{
			[]clusterData{
				{APIURL: "http://localhost:9090"},
				{APIURL: "http://localhost:9091"},
			},
			3,
			"http://localhost:9090",
		},
		{
			[]clusterData{
				{APIURL: "http://localhost:9091"},
				{APIURL: "http://localhost:9092"},
			},
			3,
			"http://localhost:9091",
		},
		{
			[]clusterData{
				{APIURL: "http://localhost:9093"},
				{APIURL: "http://localhost:9091"},
			},
			3,
			"http://localhost:9091",
		},
	}

	for _, table := range tables {
		config := provider.loadRules(&clusterResponse{table.clusters})
		checkConfig(t, config, table.expectedRules)
		checkDefaultBackendURL(t, config, table.expectedURL)
	}
}

func TestCreateFrontend(t *testing.T) {
	url := "https://api.starter-us-east-2.openshift.com"
	backend := "backend1"
	actual := createFrontend(url, backend)
	require.NotNil(t, actual)
	require.NotNil(t, actual.Routes)
	assert.Equal(t, 1, len(actual.Routes), "Mis-match no of routes, want:%d, got:%d", 1, len(actual.Routes))
	routes1 := actual.Routes["test_1"]
	require.NotZero(t, routes1)
	assert.Contains(t, routes1.Rule, "HeadersRegexp:Target")
	assert.Contains(t, routes1.Rule, url)
}
func TestCreateBackend(t *testing.T) {
	url := "https://api.starter-us-east-2.openshift.com"
	actual := createBackend(url)
	require.NotNil(t, actual)
	require.NotNil(t, actual.Servers)
	assert.Equal(t, 1, len(actual.Servers), "Mis-match no of backend servers, want:%d, got:%d", 1, len(actual.Servers))
	server1 := actual.Servers["server1"]
	require.NotZero(t, server1)
	assert.Equal(t, url, server1.URL, "Mis-match server url, want:%s, got:%s", url, server1.URL)
}

func checkConfig(t *testing.T, config *types.Configuration, ruleCount int) {
	require.NotNil(t, config)
	require.NotNil(t, config.Frontends)
	require.NotNil(t, config.Backends)
	assert.Equal(t, ruleCount, len(config.Frontends), "Mis-match no of frontends, want:%d, got:%d", ruleCount, len(config.Frontends))
	assert.Equal(t, ruleCount, len(config.Backends), "Mis-match no of backends, want:%d, got:%d", ruleCount, len(config.Frontends))
}

func checkDefaultBackendURL(t *testing.T, config *types.Configuration, expectedURL string) {
	defaultBackend := config.Backends["default"]
	require.NotZero(t, defaultBackend)
	server1URL := defaultBackend.Servers["server1"].URL
	assert.Equal(t, expectedURL, server1URL)
}
