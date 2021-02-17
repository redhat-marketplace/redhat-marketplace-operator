package prometheus_test

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"
	"text/template"

	"github.com/Masterminds/sprig"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	. "github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/prometheus"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
)

type DockerRun struct {
	*exec.Cmd
}

type DockerRunArgs struct {
	Path                  string
	LocalPort, RemotePort int
}

const prometheusRunStr = `docker run -p {{ .LocalPort }}:{{ .RemotePort }} -v {{ .Path }}/prometheus:/prometheus -v {{ .Path }}/etc/prometheus:/etc/prometheus --entrypoint /bin/sh prom/prometheus@sha256:ea420f6cd98e4e43e264a7a2e6e27e8328f47aa32d937e0c6e8e3b30fdefe6eb -c`
const cCommandStr = `/bin/prometheus --web.listen-address=0.0.0.0:{{ .RemotePort }} --web.enable-admin-api --config.file=/etc/prometheus/prometheus.yml --storage.tsdb.path=/prometheus --web.console.libraries=/usr/share/prometheus/console_libraries --web.console.templates=/usr/share/prometheus/consoles --storage.tsdb.retention.size=2GB`

var prometheusCommandTempl *template.Template = utils.Must(func() (interface{}, error) {
	return template.New("prometheusCommand").Funcs(sprig.GenericFuncMap()).Parse(prometheusRunStr)
}).(*template.Template)

var cCommandTempl *template.Template = utils.Must(func() (interface{}, error) {
	return template.New("cCommand").Funcs(sprig.GenericFuncMap()).Parse(cCommandStr)
}).(*template.Template)

func NewPrometheusDockerRun(args DockerRunArgs) (*DockerRun, error) {
	var script, cCommand strings.Builder
	err := prometheusCommandTempl.Execute(&script, args)

	if err != nil {
		return nil, err
	}

	err = cCommandTempl.Execute(&cCommand, args)

	if err != nil {
		return nil, err
	}

	cmdArgs := strings.Split(strings.ReplaceAll(script.String(), "\n", " "), " ")
	cmdArgs = append(cmdArgs, cCommand.String())

	dockerLocation, err := exec.Command("which", "docker").Output()
	if err != nil {
		return nil, err
	}
	dockerLocationStr := strings.TrimSpace(string(dockerLocation))
	cmd := exec.Command(dockerLocationStr, cmdArgs[1:]...)

	return &DockerRun{
		Cmd: cmd,
	}, nil
}

type DockerTest struct {
	DataPath   string
	Start, End time.Time
	Validate   func(model.Value, v1.Warnings, error)
}

func MustTime(str string) time.Time {
	return utils.Must(func() (interface{}, error) {
		return time.Parse(time.RFC3339, str)
	}).(time.Time)
}

var _ = Describe("MeterefQuery", func() {
	var papi PrometheusAPI

	BeforeEach(func() {
		client, err := api.NewClient(api.Config{
			Address: "http://localhost:9090",
		})

		Expect(err).To(Succeed())

		papi = PrometheusAPI{
			API: v1.NewAPI(client),
		}
	})

	DescribeTable("query meter definitions",
		func(c DockerTest) {
			cwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			path := filepath.Join(cwd, "..", "..", c.DataPath)
			cmd, err := NewPrometheusDockerRun(DockerRunArgs{
				Path:       path,
				LocalPort:  9090,
				RemotePort: 9090,
			})
			Expect(err).To(Succeed())

			session, err := gexec.Start(cmd.Cmd, GinkgoWriter, GinkgoWriter)
			defer func() {
				session.Terminate().Wait(10 * time.Second)
			}()
			Expect(err).To(Succeed())

			Eventually(session.Err, 10*time.Second).Should(gbytes.Say(`msg="Server is ready to receive web requests."`))

			model, warns, err := papi.QueryMeterDefinitions(&MeterDefinitionQuery{
				Start: c.Start,
				End:   c.End,
				Step:  time.Hour,
			})

			c.Validate(model, warns, err)
		},
		Entry("with many to many results should pass", DockerTest{
			DataPath: filepath.Join("data", "prometheus-meterdef-query-error-202002121300"),
			Start:    MustTime("2021-02-12T00:00:00Z"),
			End:      MustTime("2021-02-13T00:00:00Z"),
			Validate: func(values model.Value, warns v1.Warnings, err error) {
				Expect(err).To(Succeed())
				Expect(warns).To(BeEmpty())
				Expect(values.Type()).To(Equal(model.ValMatrix))
				mat, ok := values.(model.Matrix)
				Expect(ok).To(BeTrue())
				Expect(mat).To(HaveLen(3))
			},
		}),
	)
})

var _ = AfterSuite(func() {
	gexec.KillAndWait(10 * time.Second)
})
