package utils

type VersionedConfig interface {
	GetVersion() string
	Upgrade() (VersionedConfig, error)
}
