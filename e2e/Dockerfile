# Builds the service and goose into one small image for the e2e stack.
FROM golang:1.26.5 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
RUN CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/policy-service ./cmd/policy-service

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/policy-service /usr/local/bin/policy-service
COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY migrations /migrations
ENTRYPOINT ["/usr/local/bin/policy-service"]
