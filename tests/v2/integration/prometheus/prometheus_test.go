// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"
	"text/template"

	"github.com/Masterminds/sprig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/common"
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

type PrometheusDockerTest struct {
	DataPath string

	session *gexec.Session
}

func MustTime(str string) time.Time {
	return utils.Must(func() (interface{}, error) {
		return time.Parse(time.RFC3339, str)
	}).(time.Time)
}

func (c *PrometheusDockerTest) Start() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path := filepath.Join(cwd, "..", "..", c.DataPath)
	cmd, err := NewPrometheusDockerRun(DockerRunArgs{
		Path:       path,
		LocalPort:  8080,
		RemotePort: 8080,
	})

	if err != nil {
		return err
	}

	session, err := gexec.Start(cmd.Cmd, GinkgoWriter, GinkgoWriter)

	if err != nil {
		return err
	}

	c.session = session

	defer session.Err.CancelDetects()
	select {
	case <-session.Err.Detect(`msg="Server is ready to receive web requests."`):
		return nil
	case <-time.NewTimer(10 * time.Second).C:
		return errors.New("failed to start server")
	}
}

func (c *PrometheusDockerTest) Stop() {
	if c.session != nil {
		c.session.Terminate().Wait(5 * time.Second)
	}
}

var server *PrometheusDockerTest

var _ = Describe("MeterefQuery", func() {
	var papi PrometheusAPI
	var start, end time.Time

	Context("data from 202002121300", func() {
		BeforeEach(func() {
			server = &PrometheusDockerTest{
				DataPath: filepath.Join("data", "prometheus-meterdef-query-error-202102121300"),
			}

			Eventually(func() error {
				return server.Start()
			}, time.Second*60, time.Second*5).Should(Succeed())

			start = MustTime("2021-02-12T00:00:00Z")
			end = MustTime("2021-02-13T00:00:00Z")

			client, _ := api.NewClient(api.Config{
				Address: "http://localhost:8080",
			})

			papi = PrometheusAPI{
				API: v1.NewAPI(client),
			}
		})

		// Ensuring we can query for the meterdefs for this time frame
		It("should query for meter defs and return", func() {
			values, warns, err := papi.QueryMeterDefinitions(&MeterDefinitionQuery{
				Start: start,
				End:   end,
				Step:  time.Hour,
			})

			Expect(err).To(Succeed())
			Expect(warns).To(BeEmpty())
			Expect(values.Type()).To(Equal(model.ValMatrix))
			mat, ok := values.(model.Matrix)
			Expect(ok).To(BeTrue())
			Expect(mat).To(HaveLen(3))
		})

		// With this data, this query used to error. Fixes were made to resolve this.
		It("should query for a duplicate data set and return", func() {
			labels := map[string]string{
				"date_label_override":  "{{ .Label.date}}",
				"meter_group":          "{{ .Label.productId}}.licensing.ibm.com",
				"meter_kind":           "IBMLicensing",
				"metric_aggregation":   "max",
				"metric_group_by":      "[\"metricId\",\"productId\"]",
				"metric_label":         "{{ .Label.metricId}}",
				"metric_period":        "24h0m0s",
				"metric_query":         "product_license_usage{}",
				"name":                 "ibm-licensing-service-product-instance",
				"namespace":            "ibm-common-services",
				"value_label_override": "{{ .Label.value}}",
				"workload_name":        "{{ .Label.productId}}.licensing.ibm.com",
				"workload_type":        "Service",
			}

			meterDefLabels := &common.MeterDefPrometheusLabels{}
			err := meterDefLabels.FromLabels(labels)
			Expect(err).To(Succeed())

			promQuery := NewPromQueryFromLabels(meterDefLabels, start, end)

			values, warns, err := papi.ReportQuery(promQuery)

			Expect(err).To(Succeed())
			Expect(warns).To(BeEmpty())
			Expect(values.Type()).To(Equal(model.ValMatrix))
			mat, ok := values.(model.Matrix)
			Expect(ok).To(BeTrue())
			Expect(mat).To(HaveLen(2))
		})

		AfterEach(func() {
			server.Stop()
		})
	})
})

var _ = AfterSuite(func() {
	if server != nil {
		server.Stop()
	}
	gexec.KillAndWait(10 * time.Second)
})
