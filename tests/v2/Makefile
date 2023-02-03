include ../../utils.Makefile

.PHONY: test
test: ginkgo
	- $(MAKE) test-data

test-data-extract:
	$(shell data/prometheus-meterdef-query-error-202102121300/extract.sh)
	$(shell data/prometheus-license-multiple-meterdefs-20210331000000/extract.sh)
	$(shell data/prometheus-robin-datatest-20210608/extract.sh)

test-data: ginkgo test-data-extract
	$(GINKGO) --randomize-all --randomize-suites --cover --race --show-node-events --trace ./integration/prometheus/...
