BIN := "./bin/calendar"
DOCKER_IMG="calendar:develop"

GOLANGCI_LINT_VERSION := v1.63.4

GIT_HASH := $(shell git log --format="%h" -n 1)
LDFLAGS := -X main.release="develop" -X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%S) -X main.gitHash=$(GIT_HASH)

#Postrges
POSTGRES_USER ?= postgres
POSTGRES_PASSWORD ?= password
POSTGRES_DB ?= backend
POSTGRES_PORT ?= 5435
POSTGRES_CONTAINER := postgres-calendar
POSTGRES_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

#Rabbit
RABBIT_USER ?= rabbit
RABBIT_PASSWORD ?= password
RABBIT_PORT ?= 5672
RABBIT_PORT_UI ?= 15672
RABBIT_CONTAINER := rabbitmq
RABBIT_DSN := amqp://$(RABBIT_USER):$(RABBIT_PASSWORD)@127.0.0.1:$(RABBIT_PORT)/

#Protoc
PROTOC_GEN_GO_VERSION := v1.36.6
PROTOC_GEN_GO_GRPC_VERSION := v1.5.1
PROTO_BIN := $(CURDIR)/bin

COMPOSE := docker compose -f deployments/docker-compose.yaml

run-postgres:
	docker run -d --name $(POSTGRES_CONTAINER) \
	-e POSTGRES_USER=$(POSTGRES_USER) \
	-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
	-e POSTGRES_DB=$(POSTGRES_DB) \
	-p $(POSTGRES_PORT):5432 \
	-v postgres-data:/var/lib/postgresql/data \
	postgres:latest

run-rabbit:
	docker run -d --name $(RABBIT_CONTAINER) \
	-p $(RABBIT_PORT):5672 \
	-p $(RABBIT_PORT_UI):15672 \
	-e RABBITMQ_DEFAULT_USER=$(RABBIT_USER) \
	-e RABBITMQ_DEFAULT_PASS=$(RABBIT_PASSWORD) \
	rabbitmq:management

build:
	go build -v -o $(BIN) -ldflags "$(LDFLAGS)" ./cmd/calendar

run: build
	$(BIN) -config ./configs/config.yaml

build-scheduler:
	go build -v -o ./bin/calendar_scheduler \
		-ldflags "$(LDFLAGS)" \
		./cmd/calendar_scheduler

run-scheduler: build-scheduler
	./bin/calendar_scheduler -config ./configs/scheduler.yaml

build-img:
	docker build \
		--build-arg=LDFLAGS="$(LDFLAGS)" \
		-t $(DOCKER_IMG) \
		-f build/Dockerfile .

run-img: build-img
	docker run $(DOCKER_IMG)

version: build
	$(BIN) version

test:
	go test -race ./...

install-lint-deps:
	@command -v golangci-lint >/dev/null || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

lint: install-lint-deps
	golangci-lint run --concurrency=2 --timeout=5m ./...

db-up:
	$(COMPOSE) up -d --wait postgres

db-down:
	$(COMPOSE) stop postgres

db-logs:
	$(COMPOSE) logs -f postgres

db-shell:
	$(COMPOSE) exec postgres \
		psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

rabbit-up:
	$(COMPOSE) up -d --wait rabbitmq

rabbit-down:
	$(COMPOSE) stop rabbitmq

rabbit-logs:
	$(COMPOSE) logs -f rabbitmq

infra-up: db-up rabbit-up

infra-down:
	$(COMPOSE) down

migrate:
	goose -dir migrations postgres "$(POSTGRES_DSN)" up

migrate-down:
	goose -dir migrations postgres "$(POSTGRES_DSN)" down

migrate-status:
	goose -dir migrations postgres "$(POSTGRES_DSN)" status


install-proto-tools:
	GOBIN=$(PROTO_BIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(PROTO_BIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

generate:
	@mkdir -p pkg/calendarpb
	PATH="$(PROTO_BIN):$$PATH" go generate ./cmd/calendar

.PHONY: build run build-scheduler run-scheduler build-img run-img version test lint rabbit-up run-rabbit run-postgres rabbit-down rabbit-logs infra-up infra-down db-up db-down db-logs db-shell install-proto-tools generate
