.PHONY: build test test-e2e e2e clean

build:
	go build -o bin/campfire .

test:
	go test -v ./pkg/... ./cmd/...

# Run E2E tests against the currently active Kubernetes cluster
test-e2e: build
	go test -v -parallel 8 -timeout 10m ./test/e2e/...

# Spin up a fresh KinD cluster, deploy Agent Sandbox, run E2E tests, and tear down
e2e:
	./scripts/e2e.sh

clean:
	rm -rf bin/
