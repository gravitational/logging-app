ifndef VERSION
$(error 'VERSION not specified')
endif

COLLECTOR_TAG := log-collector:$(VERSION)
FORWARDER_TAG := log-forwarder:$(VERSION)

REPO_URL ?= quay.io/gravitational

.PHONY: all
all: collector forwarder

.PHONY: collector
collector:
	docker build --pull -t $(COLLECTOR_TAG) collector

# TODO: remote_syslog2 binary is in repo, make it build in Docker ct
.PHONY: forwarder
forwarder:
	docker build --pull -t $(FORWARDER_TAG) forwarder

.PHONY: deploy
deploy:
	$(foreach ct,$(COLLECTOR_TAG) $(FORWARDER_TAG), \
		docker tag $(ct) $(REPO_URL)/$(ct) ; \
		docker push $(REPO_URL)/$(ct) ; )

