.PHONY: run run-system run-member build build-linux clean test proto swagger generate-crud generate-db-crud generate-all migrate docker-up docker-down lint load-test pprof pprof-cpu helm-install helm-upgrade codegen-smoke new-module

MODULE ?= system
TABLES ?=

run: ## mono 模式：单进程启动全部模块（默认）
	go run ./alexgo-server/cmd/main.go

run-system: ## micro 模式 system-server（含 gRPC TokenService）
	DEPLOYMENT_MODE=micro go run ./alexgo-server/cmd/main.go

run-member: ## micro 模式 member-server（:8081，委托 system 签发）
	DEPLOYMENT_MODE=micro HTTP_ADDR=:8081 SYSTEM_GRPC_ADDR=127.0.0.1:50051 go run ./modules/member/cmd/main.go

build:
	go build -o bin/alexgo-server ./alexgo-server/cmd

build-linux: ## 交叉编译 Linux amd64（GOARCH 可覆盖：make build-linux GOARCH=arm64）
	CGO_ENABLED=0 GOOS=linux GOARCH=$(or $(GOARCH),amd64) go build -ldflags '-s -w' -o bin/alexgo-server-linux ./alexgo-server/cmd
	CGO_ENABLED=0 GOOS=linux GOARCH=$(or $(GOARCH),amd64) go build -ldflags '-s -w' -o bin/member-server-linux ./modules/member/cmd

proto:
	bash scripts/gen_proto.sh

SWAG ?= $(shell go env GOPATH)/bin/swag
SWAG_MODULES := fishgame system scorecard member order

swagger: ## 每模块独立生成 API 文档到 docs/swagger/<模块>/ 并注入侧栏分组；需先 go install github.com/swaggo/swag/cmd/swag@v1.16.4
	@for m in $(SWAG_MODULES); do \
		echo "== swag init $$m =="; \
		$(SWAG) init -g swagger_doc.go --dir modules/$$m -o docs/swagger/$$m --instanceName $$m --parseDepth 3 || exit 1; \
	done
	python3 scripts/swagger_inject_taggroups.py

generate-crud: ## 从配置表生成已导入表（MODULE 过滤，默认 system；不覆盖已有文件）
	go run ./tools/codegen $(if $(MODULE),-module=$(MODULE))

generate-db-crud: ## 现场读元数据（未导入则先入配置表）再生成；TABLES 留空 = schema 全部表
	go run ./tools/codegen -module=$(MODULE) -tables=$(TABLES) -import

generate-all: proto generate-crud

codegen-smoke: ## M1 验收：真实表生成→编译→跑生成测试→清理（需本地 MySQL）
	go test -tags integration ./tools/codegen/ -run TestSmoke -v -count=1

new-module: ## 新建模块骨架（NAME=demo ENTITIES=item[,foo]；可加 ARGS="-dry-run"）
	go run ./tools/newmodule/cmd -name=$(NAME) -entities=$(ENTITIES) $(ARGS)

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
