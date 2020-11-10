package clusterregistration

import (
	"io/ioutil"
	"net/http"
	"net/url"

	"emperror.dev/errors"
	yaml "github.com/ghodss/yaml"
)

type HttpClusterConfig struct {
	Url         string
	Token       string
	AccountId   string
	ClusterUuid string
}

type HttpClusterResponse struct {
	StatusCode int32
	ClusterDef string
}

func ClusterRegistrationStatus(httpClusterConfig *HttpClusterConfig) (bool, error) {
	httpClient := &http.Client{}
	accountId := httpClusterConfig.AccountId
	clusterUuid := httpClusterConfig.ClusterUuid
	u, err := url.Parse(httpClusterConfig.Url)
	if err != nil {
		return false, err
	}
	q := u.Query()
	q.Set("accountId", accountId)
	q.Set("uuid", clusterUuid)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Add("Authorization", httpClusterConfig.Token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	clusterDef, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == 200 && string(clusterDef) != "" {
		return true, nil
	}
	return false, err
}

func GetMarketPlaceSecret(httpClusterConfig *HttpClusterConfig) ([]byte, error) {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", httpClusterConfig.Url, nil)
	req.Header.Add("Authorization", httpClusterConfig.Token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rhOperatorSecretDef, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return rhOperatorSecretDef, err
	}
	if resp.StatusCode != 200 {
		return rhOperatorSecretDef, errors.Wrap(err, string(rhOperatorSecretDef))
	}

	data, err := yaml.YAMLToJSON(rhOperatorSecretDef) //Converting Yaml to JSON so it can parse to secret object
	if err != nil {
		return data, err
	}

	return data, nil
}
