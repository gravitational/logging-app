VER := 0.0.2

BOOTSTRAP_TAG := log-bootstrap:$(VER)
UNINSTALL_TAG := log-uninstall:$(VER)
COLLECTOR_TAG := log-collector:$(VER)
FORWARDER_TAG := log-forwarder:$(VER)

REPO_URL ?= quay.io/gravitational

KUBECTL_URL := https://storage.googleapis.com/kubernetes-release/release/v1.2.0/bin/linux/amd64/kubectl

.PHONY: all
all: bootstrap uninstall collector forwarder

kubectl:
	curl -o kubectl -L $(KUBECTL_URL)
	chmod +x kubectl

.PHONY: bootstrap
bootstrap: kubectl
	cp kubectl bootstrap/
	docker build --pull -t $(BOOTSTRAP_TAG) bootstrap

.PHONY: uninstall
uninstall: kubectl
	cp kubectl uninstall/
	docker build --pull -t $(UNINSTALL_TAG) uninstall

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

