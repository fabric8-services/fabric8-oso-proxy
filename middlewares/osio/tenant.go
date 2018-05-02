package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func CreateTenantLocator(client *http.Client, tenantBaseURL string) TenantLocator {
	return func(token, service string) (namespace, error) {
		return locateTenant(client, tenantBaseURL, token)
	}
}

type response struct {
	Data data `json:"data"`
}

type data struct {
	Attributes attributes `json:"attributes"`
}

type attributes struct {
	Namespaces []namespace `json:"namespaces"`
}

type namespace struct {
	ClusterURL        string `json:"cluster-url"`
	ClusterMetricsURL string `json:"cluster-metrics-url"`
	ClusterConsoleURL string `json:"cluster-console-url"`
	ClusterLoggingURL string `json:"cluster-logging-url"`
}

func getNamespace(resp response) (ns namespace, err error) {
	if len(resp.Data.Attributes.Namespaces) == 0 {
		return ns, fmt.Errorf("unable to locate namespace")
	}

	return resp.Data.Attributes.Namespaces[0], nil
}

func locateTenant(client *http.Client, tenantBaseURL, osioToken string) (ns namespace, err error) {

	req, err := http.NewRequest("GET", tenantBaseURL+"/user/services", nil)
	if err != nil {
		return ns, err
	}
	req.Header.Set(Authorization, "Bearer "+osioToken)
	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return ns, fmt.Errorf("Unknown status code " + resp.Status)
	}
	defer resp.Body.Close()

	var r response
	json.NewDecoder(resp.Body).Decode(&r)
	return getNamespace(r)
}
