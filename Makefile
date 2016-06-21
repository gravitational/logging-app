.PHONY: package build make-tarball clean pull-from-internet \
	start-registry stop-registry push-layers-to-registry \
	export-layers-from-registry

PULL_CONTAINERS=quay.io/gravitational/debian-tall:0.0.1 \
   quay.io/gravitational/debian-grande:0.0.1
CONTAINERS=$(PULL_CONTAINERS) gravitational.io/log-forwarder:0.0.1 \
	gravitational.io/log-collector:0.0.1 gravitational.io/log-uninstall:0.0.1 \
	gravitational.io/log-bootstrap:0.0.1


REGISTRYIMAGE=registry:2.1.1
REGISTRYPORT=5055
RUNNINGREG=$$(docker ps -q --filter="ancestor=$(REGISTRYIMAGE)")
REGADDRESS=127.0.0.1:$(REGISTRYPORT)

#
# Output: that is where the tarball will be created
#
OUT=build/logging-app.tar.gz
package: $(OUT)

$(OUT): build
	$(MAKE) start-registry
	$(MAKE) pull-from-internet
	$(MAKE) push-layers-to-registry
	$(MAKE) export-layers-from-registry
	$(MAKE) make-tarball
	$(MAKE) stop-registry


build:
	$(MAKE) -C images

#
# makes the final tarball out of 'app' subfolder
#
make-tarball:
	@echo "Making a tarball...."
	mkdir -p build
	tar -cvzf $(OUT) resources registry
	@echo "done ---> $(OUT)"


#
# removes everything
#
clean:
	$(MAKE) stop-registry
	rm -rf build
	rm -rf registry


#
# pushes images from local Docker to temporary registry. THIS TAKES A LOT OF TIME.
#
push-layers-to-registry:
	for container in $(CONTAINERS); do \
		echo "docker tag $$container $(REGADDRESS)/$$container" ;\
		docker tag $$container $(REGADDRESS)/$$container ;\
		docker push $(REGADDRESS)/$$container ;\
		docker rmi $(REGADDRESS)/$$container ;\
		echo "\n" ;\
	done


#
# exports /var/lib/registry from temporary registry container into 'registry' folder
#
export-layers-from-registry:
	@echo "Copying layers from temporary registry into registry folder..."
	@rm -rf registry
	@docker cp $(RUNNINGREG):/var/lib/registry registry


#
# starts the temporary docker registry
#
start-registry:
	@if [ -z "$(RUNNINGREG)" ]; then \
		docker run -d -p $(REGISTRYPORT):5000 $(REGISTRYIMAGE) ;\
		echo "Started temporary Docker registry on port $(REGISTRYPORT)\n" ;\
	else \
		echo "Temporary Docker registry is already listening on port $(REGISTRYPORT)\n" ;\
	fi


#
# stops the temporary docker registry
#
stop-registry:
	@if [ ! -z "$(RUNNINGREG)" ]; then \
		container=$(RUNNINGREG) ;\
		docker stop $$container >/dev/null && docker rm -v $$container >/dev/null ;\
	else \
		echo registry is not running ;\
	fi


#
# pulls images from the internet
#
pull-from-internet:
	echo "Pulling Docker images"
	@for container in $(PULL_CONTAINERS); do \
		docker pull $$container ;\
	done

