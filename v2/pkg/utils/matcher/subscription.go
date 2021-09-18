package matcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
)

var (
	ErrPackageNameNotFound    error = errors.New("could not parse package name")
	ErrNoMatchForCSVToSub     error = errors.New("could not match csv to an rhm subscription")
	ErrSubscriptionIsUpdating error = errors.New("subscription is updating, CurrentCSV and InstalledCSV don't match")
)

const (
	csvProp string = "operatorframework.io/properties"
)

func MatchCsvToSub(catalogName string, packageName string, subs []olmv1alpha1.Subscription, CSV *olmv1alpha1.ClusterServiceVersion) (*olmv1alpha1.Subscription, error) {
	csvSplitName := strings.Split(CSV.Name, ".")[0]

	if len(subs) > 0 {
		for _, s := range subs {
			//try to match the csv without using packageName provided by olm.properties annotation
			if packageName == "" {
				if s.Status.InstalledCSV == CSV.Name && s.Spec.CatalogSource == catalogName {
					return &s, nil

				} else if strings.HasPrefix(s.Status.CurrentCSV, csvSplitName) && s.Status.InstalledCSV != CSV.Name {
					err := fmt.Errorf("CurrentCSV: %s InstalledCSV: %s %w", s.Status.CurrentCSV, s.Status.InstalledCSV, ErrSubscriptionIsUpdating)
					return nil, err
				}
			}

			if packageName == s.Spec.Package && s.Status.InstalledCSV == CSV.Name && s.Spec.CatalogSource == catalogName {
				return &s, nil

			} else if packageName == s.Spec.Package && strings.HasPrefix(s.Status.CurrentCSV, csvSplitName) && s.Status.InstalledCSV != CSV.Name {
				err := fmt.Errorf("CurrentCSV: %s InstalledCSV: %s %w", s.Status.CurrentCSV, s.Status.InstalledCSV, ErrSubscriptionIsUpdating)
				return nil, err
			}
		}
	}

	return nil, nil
}

func CheckOperatorTag(foundSub *olmv1alpha1.Subscription) bool {
	if foundSub != nil {
		if value, ok := foundSub.GetLabels()[utils.OperatorTag]; ok {
			if value == utils.OperatorTagValue {
				return true
			}
		}
	}

	return false
}

func ParsePackageName(csv *olmv1alpha1.ClusterServiceVersion) (string, error) {
	csvProps, ok := csv.GetAnnotations()[csvProp]
	if !ok {
		return "", errors.New("could not find csv properties annotations")
	}

	var unmarshalledProps map[string]interface{}
	err := json.Unmarshal([]byte(csvProps), &unmarshalledProps)
	if err != nil {
		return "", err
	}

	properties := unmarshalledProps["properties"].([]interface{})
	for _, _prop := range properties {
		p, ok := _prop.(map[string]interface{})
		if !ok {
			return "", errors.New("type conversion error []Property")
		}

		if p["type"] == "olm.package" {
			/*
				{
					"type":"olm.package",
					"value":{
						"packageName":"memcached-operator-rhmp",
						"version":"0.0.1"
					}
				},
			*/

			value, ok := p["value"].(map[string]interface{})
			if !ok {
				return "", errors.New("type conversion error Property.Value")
			}

			packageName, ok := value["packageName"].(string)
			if !ok {
				return "", errors.New("type conversion error Property.Value.PackageName")
			}

			return packageName, nil
		}
	}

	return "", nil
}
