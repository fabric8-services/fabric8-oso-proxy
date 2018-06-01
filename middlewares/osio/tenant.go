package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type tenantData struct {
}

type tenantLocator struct {
	client    *http.Client
	tenantURL string
}

func (t *tenantLocator) GetTenant(token string, tokenType TokenType) (namespace, error) {
	url := fmt.Sprintf("%s/tenant", t.tenantURL)
	return locateTenant(t.client, url, token, tokenType)
}

func (t *tenantLocator) GetTenantById(token string, tokenType TokenType, userID string) (namespace, error) {
	url := fmt.Sprintf("%s/tenants/%s", t.tenantURL, userID)
	return locateTenant(t.client, url, token, tokenType)
}

func CreateTenantLocator(client *http.Client, tenantBaseURL string) TenantLocator {
	return &tenantLocator{client: client, tenantURL: tenantBaseURL}
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
	Name              string `json:"name"`
	Type              string `json:"type"`
	ClusterURL        string `json:"cluster-url"`
	ClusterMetricsURL string `json:"cluster-metrics-url,omitempty"`
	ClusterConsoleURL string `json:"cluster-console-url,omitempty"`
	ClusterLoggingURL string `json:"cluster-logging-url,omitempty"`
}

func getNamespace(resp response, tokenType TokenType) (ns namespace, err error) {
	if len(resp.Data.Attributes.Namespaces) == 0 {
		return ns, fmt.Errorf("no namespace found")
	}
	for _, namespace := range resp.Data.Attributes.Namespaces {
		if namespace.Type == string(tokenType) {
			return namespace, nil
		}
	}
	return ns, fmt.Errorf("no namespace matched")
}

func locateTenant(client *http.Client, url, token string, tokenType TokenType) (ns namespace, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ns, err
	}
	req.Header.Set(Authorization, "Bearer "+token)
	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return ns, fmt.Errorf("Call to '%s' failed with status '%s'", url, resp.Status)
	}
	defer resp.Body.Close()

	var r response
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		return ns, err
	}
	return getNamespace(r, tokenType)
}
