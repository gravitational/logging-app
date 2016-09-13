export VERSION := $(shell git describe --tags)
REPOSITORY := gravitational.io
NAME := logging-app
OPS_URL ?= https://opscenter.localhost.localdomain:33009
OUT ?= $(NAME).tar.gz
GRAVITY ?= gravity
UPDATE_IMAGE_OPTS := \
	--set-image=log-collector:$(VERSION) --set-image=log-forwarder:$(VERSION) \
	--set-image=log-tailer:$(VERSION) --set-image=log-linker:$(VERSION)
UPDATE_METADATA_OPTS := --repository=$(REPOSITORY) --name=$(NAME) --version=$(VERSION)
S3_BUCKET = s3://builds.gravitational.io/logging-app

BUILDDIR = ./build
LOGGING_APP_OUT := $(BUILDDIR)/logging-app.tar.gz


.PHONY: package
package:
	$(MAKE) -C images all


.PHONY: forwarder
forwarder:
	$(MAKE) -C images forwarder

.PHONY: deploy
deploy:
	$(MAKE) -C images deploy

.PHONY: import
import: package
	-$(GRAVITY) app delete --ops-url=$(OPS_URL) $(REPOSITORY)/$(NAME):$(VERSION) \
		--force --insecure
	$(GRAVITY) app import --debug --insecure --vendor \
		--ops-url=$(OPS_URL) \
		$(UPDATE_IMAGE_OPTS) \
		$(UPDATE_METADATA_OPTS) \
		--include=resources --include=registry .

.PHONY: clean
clean:
	$(MAKE) -C images clean

# Publish processed tarball to S3
.PHONY: publish
publish: TMPDIR := $(shell mktemp -d)
publish: local-import local-export
	aws cp --region us-east-1 $(LOGGING_APP_OUT) $(S3_BUCKET)/$(VERSION)
	rm -rf $(TMPDIR)

.PHONY: local-import
local-import: package
	$(GRAVITY) app import --insecure --vendor \
		--state-dir=$(TMPDIR) \
		$(UPDATE_IMAGE_OPTS) \
		$(UPDATE_METADATA_OPTS) \
		--include=resources --include=registry .

.PHONY: local-export
local-export:
	mkdir -p $(BUILDDIR)
	$(GRAVITY) --state-dir=$(TMPDIR) package export $(REPOSITORY)/$(NAME):$(VERSION) $(LOGGING_APP_OUT)
