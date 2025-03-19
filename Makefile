.PHONY: gen deps

gen:
	buf generate proto

deps:
	go install github.com/bufbuild/buf/cmd/buf@latest