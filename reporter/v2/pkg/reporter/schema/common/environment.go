package common

const (
	ReportProductionEnv ReportEnvironment = "production"
	ReportSandboxEnv    ReportEnvironment = "stage"
)

type ReportEnvironment string

func (m ReportEnvironment) MarshalText() ([]byte, error) {
	return []byte(string(m)), nil
}

func (m *ReportEnvironment) UnmarshalText(data []byte) error {
	str := ReportEnvironment(string(data))
	*m = str
	return nil
}

func (m ReportEnvironment) String() string {
	return string(m)
}
