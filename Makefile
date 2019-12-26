# Enable GO111MODULE=on explicitly, disable it with GO111MODULE=off when necessary.
export GO111MODULE := on
GOOS := $(if $(GOOS),$(GOOS),linux)
GOARCH := $(if $(GOARCH),$(GOARCH),amd64)
GOENV  := GO15VENDOREXPERIMENT="1" CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO     := $(GOENV) go
GOTEST := TEST_USE_EXISTING_CLUSTER=false go test

all: dashboard images

# Run go fmt against code
fmt:
	$(GO) fmt ./...

# Run go vet against code
vet:
	$(GO) vet ./...

dashboard: fmt vet
	$(GO) build -ldflags '$(LDFLAGS)' -o images/chaos-dashboard/bin/chaos-dashboard ./cmd/chaos-dashboard/*.go

dashboard-server-frontend:
	cd images/chaos-dashboard; yarn build

images: dashboard dashboard-server-frontend
	docker build -t pingcap/chaos-grafana images/grafana
	docker build -t pingcap/chaos-dashboard images/chaos-dashboard

.PHONY: images