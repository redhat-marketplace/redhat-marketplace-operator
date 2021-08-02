package common


type InstallMapping struct {
	Namespace                 string   `json:"namespace,omitempty"`
	CsvName                   string   `json:"csvName,omitempty"`
	CsvVersion                string   `json:"version,omitempty"`
	InstalledMeterdefinitions []string `json:"installedMeterdefinitions,omitempty"`
}

//TODO: mutex needed here ?
type MeterdefinitionStore struct {
	InstallMappings []InstallMapping `json:"installMappings"`
}
