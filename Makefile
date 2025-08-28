.PHONY: gen deps

gen:
	buf generate proto
buf:
	cd ./gosdk && go install github.com/bufbuild/buf/cmd/buf@latest
get:
	cd ./gosdk && go mod download github.com/0xAtelerix/sdk/gosdk

tidy:
	cd ./gosdk && go mod tidy

tests:
	cd ./gosdk && go test -short -timeout 20m -failfast -shuffle=on -v ./... $(params)

race-tests:
	cd ./gosdk && go test -race -short -timeout 30m -failfast -shuffle=on -v ./... $(params)

VERSION_TAG_SCRIPT=sed -n 's/.*golangci-lint@\(v[0-9.]*\).*/\1/p' ./.gitlab-ci.yml
VERSION=$(shell $(VERSION_TAG_SCRIPT))

lints-docker: # 'sed' matches version in this string 'golangci-lint@xx.yy.zzz'
	echo "⚙️ Used lints version: " $(VERSION)
	docker run --rm -v $$(pwd):/app -w /app golangci/golangci-lint:$(VERSION) golangci-lint run -v  --timeout 10m

deps:
	cd ./gosdk && go mod download
	cd ./gosdk && go install github.com/bufbuild/buf/cmd/buf@latest
	cd ./gosdk && go get google.golang.org/grpc@v1.75.0
	cd ./gosdk && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(VERSION)

lints:
	cd ./gosdk && $$(go env GOPATH)/bin/golangci-lint run ./... -v --timeout 10m
