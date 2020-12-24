PROJECTS = operator authchecker metering reporter

export

.DEFAULT_GOAL := all

all: fmt vet generate build

skaffold-dev:
	make operator/skaffold-dev

skaffold-run:
	make operator/skaffold-run

build: vet fmt
	make operator/skaffold-build

vet: $(addsuffix /vet,$(PROJECTS))

fmt: $(addsuffix /fmt,$(PROJECTS))

test: $(addsuffix /test,$(PROJECTS))

generate: $(addsuffix /generate,$(PROJECTS))

operator/%:
	cd ./v2 && $(MAKE) $(@F)

reporter/%:
	cd ./reporter/v2 && $(MAKE) $(@F)

metering/%:
	cd ./metering/v2 && $(MAKE) $(@F)

authchecker/%:
	cd ./authchecker/v2 && $(MAKE) $(@F)

%:
	echo $@
ifeq ($@,)
	$(MAKE) operator/all
else
	$(MAKE) operator/$@
endif
