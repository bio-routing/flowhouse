.PHONY: default test vendor vendor-deps container push gitlab_ci_check apply-vendor-lock prepare-vendor-updates

all: bindata build

build:
	cd cmd/flowhouse; go build

bindata:
	cd pkg/frontend; ~/go/bin/go-bindata -pkg frontend assets/
