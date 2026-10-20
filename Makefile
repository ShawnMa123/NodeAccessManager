BIN     := nam
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: build test vet fmt dist clean check

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BIN) ./cmd/nam

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

check: vet test

# 单文件二进制，scp 到服务器 /usr/local/bin/nam 即可
dist:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-linux-amd64 ./cmd/nam
	cd dist && sha256sum $(BIN)-linux-amd64 > $(BIN)-linux-amd64.sha256
	@ls -l dist

clean:
	rm -rf bin dist
