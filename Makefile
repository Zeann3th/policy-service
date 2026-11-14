MODULE := github.com/zeann3th/haphaestus
PROTOC ?= protoc
PROTO_FILES := $(wildcard proto/*.proto)

.PHONY: test grpc swag sqlc goose-up goose-down goose-status

test:
	go test ./...

sqlc:
	sqlc generate

grpc:
	$(PROTOC) \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		$(PROTO_FILES)
