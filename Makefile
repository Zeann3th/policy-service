MODULE := github.com/zeann3th/policy-service
PROTOC ?= protoc
PROTO_FILES := $(wildcard proto/*.proto)

.PHONY: test grpc sqlc goose-up goose-down goose-status

test:
	go test ./...

sqlc:
	sqlc generate

grpc:
	$(PROTOC) \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		$(PROTO_FILES)
