.PHONY: gen deps

gen:
	buf generate proto
buf:
	go install github.com/bufbuild/buf/cmd/buf@latest
get:
	go mod download github.com/0xAtelerix/sdk/gosdk

tidy:
	go mod tidy

tests:
	go test -short -count=1 -timeout 20m -failfast -shuffle=on -v ./... $(params)

race-tests:
	go test -race -short -timeout 30m -failfast -shuffle=on -v ./... $(params)

VERSION=v2.4.0

lints-docker: # 'sed' matches version in this string 'golangci-lint@xx.yy.zzz'
	echo "⚙️ Used lints version: " $(VERSION)
	docker run --rm -v $$(pwd):/app -w /app golangci/golangci-lint:$(VERSION) golangci-lint run -v  --timeout 10m

deps-local:
	GOPRIVATE=github.com/0xAtelerix/* go mod download
	GOPRIVATE=github.com/0xAtelerix/* go install github.com/bufbuild/buf/cmd/buf@latest
	GOPRIVATE=github.com/0xAtelerix/* go get google.golang.org/grpc@v1.75.0
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(VERSION)

deps-ci:
	GOPRIVATE=github.com/0xAtelerix/* go mod download
	GOPRIVATE=github.com/0xAtelerix/* go install github.com/bufbuild/buf/cmd/buf@latest
	GOPRIVATE=github.com/0xAtelerix/* go get google.golang.org/grpc@v1.75.0

lints:
	$$(go env GOPATH)/bin/golangci-lint run ./gosdk/... -v --timeout 10m

lints-fix:
	$$(go env GOPATH)/bin/golangci-lint run ./gosdk/... -v --timeout 10m --fix
