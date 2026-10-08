.PHONY: run build clean test proto generate-crud generate-db-crud generate-all migrate docker-up docker-down lint load-test pprof pprof-cpu helm-install helm-upgrade

MODULE ?= system
TABLES ?=

run:
	go run ./alexgo-server/cmd/main.go

build:
	go build -o bin/alexgo-server ./alexgo-server/cmd

proto:
	bash scripts/gen_proto.sh

generate-crud:
	go run ./scripts/crud_generator.go

generate-db-crud:
	go run ./tools/dbgen --module=$(MODULE) --tables=$(TABLES)

generate-all: proto

migrate:
	go run ./alexgo-server/cmd/main.go --migrate-only

docker-up:
	docker-compose -f deployments/docker-compose/docker-compose.yml up --build

docker-down:
	docker-compose -f deployments/docker-compose/docker-compose.yml down

lint:
	golangci-lint run

load-test:
	k6 run scripts/load_test.js

pprof:
	go tool pprof -http=:8081 http://localhost:8080/debug/pprof/profile

pprof-cpu:
	go tool pprof -http=:8081 "http://localhost:8080/debug/pprof/profile?seconds=30"

helm-install:
	helm install alexgo-cloud deployments/helm/alexgo-cloud --namespace alexgo-cloud --create-namespace

helm-upgrade:
	helm upgrade --install alexgo-cloud deployments/helm/alexgo-cloud --namespace alexgo-cloud --create-namespace

clean:
	rm -rf bin/

test:
	go test ./...
