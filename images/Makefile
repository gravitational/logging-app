PWD=$(shell pwd)
REPODIR=$(abspath $(PWD)/../)
export

ADAPTER_TAG := log-adapter:$(VERSION)
HOOK_TAG := log-hook:$(VERSION)
CONTAINERS := $(ADAPTER_TAG) $(HOOK_TAG)

REPO_URL ?= quay.io/gravitational

.PHONY: all
all: adapter hook

.PHONY: hook
hook:
	$(eval CHANGESET = $(shell echo $$VERSION | sed -e 's/[\.]//g'))
	if [ -z "$(CHANGESET)" ]; then \
	  echo "CHANGESET is not set"; exit 1; \
	fi;
	docker build --build-arg CHANGESET=log-$(CHANGESET) --pull -t $(HOOK_TAG) hook

.PHONY: adapter
adapter: adapter-build
	docker build --pull -t $(ADAPTER_TAG) adapter

.PHONY: adapter-build
adapter-build:
	$(MAKE) -C $(PWD)/adapter -e TARGET=adapter TARGETDIR=adapter -f $(PWD)/buildbox.mk

.PHONY: adapter-clean
adapter-clean:
	$(MAKE) -C $(PWD)/adapter clean

.PHONY: deploy
deploy:
	$(foreach ct,$(CONTAINERS), \
		docker tag $(ct) $(REPO_URL)/$(ct) ; \
		docker push $(REPO_URL)/$(ct) ; )

.PHONY: clean
clean: adapter-clean
