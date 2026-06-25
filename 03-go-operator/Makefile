# AppService Operator Makefile

# 镜像仓库配置
IMG ?= appservice-operator:latest
REGISTRY ?= your-registry.example.com

# 获取所有 Go 源文件
GO_SRCS = $(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: all build run test clean docker-build docker-push deploy

# 默认目标
all: build

# 编译
build: $(GO_SRCS)
	go build -o bin/operator main.go

# 本地运行
run:
	go run main.go

# 测试
test:
	go test ./...

# 生成 CRD 和 RBAC 清单
generate:
	controller-gen crd paths=./api/v1 output:crd:artifacts:config=config/crd
	controller-gen rbac:roleName=appservice-operator paths=./internal/controller output:rbac:artifacts:config=config/rbac

# 代码格式化
fmt:
	go fmt ./...

# 代码检查
vet:
	go vet ./...

# 下载依赖
tidy:
	go mod tidy

# 构建 Docker 镜像
docker-build:
	docker build -t $(IMG) .

# 推送镜像
docker-push:
	docker tag $(IMG) $(REGISTRY)/$(IMG)
	docker push $(REGISTRY)/$(IMG)

# 部署 CRD 到集群
deploy-crd:
	kubectl apply -f config/crd/bases.yaml

# 部署 RBAC
deploy-rbac:
	kubectl apply -f config/rbac/role.yaml

# 完整部署
deploy: deploy-crd deploy-rbac
	@echo "CRD 和 RBAC 已部署"
	@echo "下一步: 部署 Operator Deployment 或本地运行 make run"

# 清理部署
undeploy:
	kubectl delete -f config/crd/bases.yaml --ignore-not-found
	kubectl delete -f config/rbac/role.yaml --ignore-not-found
