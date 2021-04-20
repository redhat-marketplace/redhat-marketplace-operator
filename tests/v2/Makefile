.PHONY: test
test: ginkgo
	- $(MAKE) test-data

test-data-extract:
	$(shell data/prometheus-meterdef-query-error-202102121300/extract.sh)
	$(shell data/prometheus-license-multiple-meterdefs-20210331000000/extract.sh)

test-data: ginkgo test-data-extract
	$(GINKGO) --randomizeAllSpecs --randomizeSuites --cover --race --progress --trace ./integration/prometheus/...

ginkgo:
ifeq (, $(shell which ginkgo))
	@{ \
	set -e ;\
	GINKGO_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$GINKGO_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get -u github.com/onsi/ginkgo/ginkgo ;\
	rm -rf $$GINKGO_GEN_TMP_DIR ;\
	}
GINKGO=$(GOBIN)/ginkgo
else
GINKGO=$(shell which ginkgo)
endif
