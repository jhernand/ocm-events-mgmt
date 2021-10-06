#
# Copyright (c) 2021 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Enable Go modules:
export GO111MODULE=on
export GOPROXY=https://proxy.golang.org
export GOPRIVATE=gitlab.cee.redhat.com

# Disable CGO so that we always generate static binaries:
export CGO_ENABLED=0

# Import path of the project:
import_path:=gitlab.cee.redhat.com/service/ocm-events-mgmt

# The namespace is calculated from the name of the user to avoid clashes in
# shared infrastructure:
namespace:=ocm-${USER}

# The version needs to be different for each deployment because otherwise the
# cluster will not pull the new image from the internal registry:
version:=$(shell date +%s)

# Set the linker flags so that the version will be included in the binaries:
ldflags:=-X $(import_path)/pkg/info.Version=$(version)

# The name of the image repository needs to start with the name of an existing
# namespace because when the image is pushed to the internal registry of a
# cluster it will assume that that namespace exists and will try to create a
# corresponding image stream inside that namespace. If the namespace doesn't
# exist the push fails. This doesn't apply when the image is pushed to a public
# repository, like `docker.io` or `quay.io`.
image_repository:=$(namespace)/ocm-events-mgmt

# Tag for the image:
image_tag:=$(version)

# In the development environment we are pushing the image directly to the image
# registry inside the development cluster. That registry has a different name
# when it is accessed from outside the cluster and when it is accessed from
# inside the cluster. We need the external name to push the image, and the
# internal name to pull it.
external_image_registry:=default-route-openshift-image-registry.apps-crc.testing
internal_image_registry:=image-registry.openshift-image-registry.svc:5000

# URL of the gateway
gateway_url:=https://api.integration.openshift.com

# Database connection details:
db_host:=events-mgmt-db.$(namespace).svc
db_port:=5432
db_name:=db
db_user:=server
db_password:=redhat123
db_admin_user:=postgres
db_admin_password:=redhat123

# Client identifier and secret:
client_id:=${CLIENT_ID}
client_secret:=${CLIENT_SECRET}

# Install indicates if the server and producer should try to create the topics that they need. In
# production environments this should be false, and the topics should be created in advance.
install:=true

# In development environments we need to use an Envoy image different to the
# one that we use in production, because that is private.
envoy_image:=envoyproxy/envoy:v1.18.3

.PHONY: cmds
cmds:
	for cmd in $$(ls cmd); do \
		go build \
			-ldflags="$(ldflags)" \
			-o "$${cmd}" \
			"./cmd/$${cmd}" \
			|| exit 1; \
	done

.PHONY: lint
lint:
	# Mirror of docker.io/golangci/golangci-lint, Docker Hub is blocked in ci-int.
	podman run --rm --security-opt label=disable --volume="$(PWD):/app" --workdir=/app \
		"quay.io/app-sre/golangci-lint:v$(shell cat .golangciversion)" \
		run

.PHONY: tools
tools:
	which ginkgo || (cd /tmp; go get -v github.com/onsi/ginkgo/ginkgo)

.PHONY: fmt
fmt:
	gofmt -s -l -w ./pkg/ ./cmd/ ./test

.PHONY: test
test: tools
	ginkgo $(ginkgo_flags) -r cmd pkg

.PHONY: image
image: cmds
	podman build -t "$(external_image_registry)/$(image_repository):$(image_tag)" .

.PHONY: push
push: image
	podman push "$(external_image_registry)/$(image_repository):$(image_tag)"

%-template:
	oc process \
		--filename="$*-template.yaml" \
		--local="true" \
		--ignore-unknown-parameters="true" \
		--local="true" \
		--param="CLIENT_ID=$(client_id)" \
		--param="CLIENT_SECRET=$(client_secret)" \
		--param="DB_ADMIN_PASSWORD=$(db_admin_password)" \
		--param="DB_ADMIN_USER=$(db_admin_user)" \
		--param="DB_HOST=$(db_host)" \
		--param="DB_NAME=$(db_name)" \
		--param="DB_PASSWORD=$(db_password)" \
		--param="DB_PORT=$(db_port)" \
		--param="DB_USER=$(db_user)" \
		--param="ENVOY_IMAGE=$(envoy_image)" \
		--param="GATEWAY_URL=$(gateway_url)" \
		--param="IMAGE_REGISTRY=$(internal_image_registry)" \
		--param="IMAGE_REPOSITORY=$(image_repository)" \
		--param="IMAGE_TAG=$(image_tag)" \
		--param="INSTALL=$(install)" \
		--param="NAMESPACE=$(namespace)" \
		--param="VERSION=$(version)" \
	> "$*-template.json"

.PHONY: project
project:
	oc project "$(namespace)" || oc new-project "$(namespace)"

deploy-%: project %-template
	oc apply --filename="$*-template.json"

undeploy-%: project %-template
	oc delete --filename="$*-template.json"

.PHONY: deploy
deploy: \
	push \
	deploy-secrets \
	deploy-db \
	deploy-broker \
	deploy-debezium \
	deploy-producer \
	deploy-envoy \
	deploy-server \
	deploy-routes \
	$(NULL)

.PHONY: undeploy
undeploy: \
	undeploy-secrets \
	undeploy-db \
	undeploy-broker \
	undeploy-debezium \
	undeploy-producer \
	undeploy-envoy \
	undeploy-server \
	undeploy-routes \
	$(NULL)

.PHONY: clean
clean:
	rm -rf \
		$$(ls cmd) \
		*-template.json \
		$(NULL)
