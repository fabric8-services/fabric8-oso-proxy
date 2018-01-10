package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func CreateTenantLocator(client *http.Client, tenantBaseURL string) TenantLocator {
	return func(token string) (string, error) {
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
	ClusterURL string `json:"cluster-url"`
}

func getClusterURL(resp response) (string, error) {
	if len(resp.Data.Attributes.Namespaces) == 0 {
		return "", fmt.Errorf("unable to locate cluster url")
	}

	return resp.Data.Attributes.Namespaces[0].ClusterURL, nil
}

func locateTenant(client *http.Client, tenantBaseURL, osioToken string) (string, error) {

	req, err := http.NewRequest("GET", tenantBaseURL+"/user/services", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(Authorization, "Bearer "+osioToken)
	//req = req.WithContext(context.)
	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unknown status code " + resp.Status)
	}
	defer resp.Body.Close()

	var r response
	json.NewDecoder(resp.Body).Decode(&r)
	return getClusterURL(r)
}
