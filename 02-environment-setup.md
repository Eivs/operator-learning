# 第 2 章：环境准备

## 2.1 你需要什么

| 工具 | 版本要求 | 用途 |
|------|---------|------|
| Go | ≥ 1.21 | 编写 Controller 逻辑 |
| Docker | ≥ 24.x | 构建镜像、运行本地 Registry |
| kubectl | ≥ 1.28 | 与 K8S 集群交互 |
| kind 或 minikube | 最新版 | 本地 K8S 开发集群 |
| Kubebuilder | ≥ 3.x | 脚手架、代码生成、测试 |

## 2.2 安装 Go

```bash
# macOS
brew install go

# Linux (从官方下载)
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# 验证
go version  # 应该输出 go1.22.0 或更高
```

## 2.3 安装 kubectl

```bash
# macOS
brew install kubectl

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/

# 验证
kubectl version --client
```

## 2.4 搭建本地 K8S 集群（二选一）

### 方案 A：kind（推荐，更轻量）

```bash
# macOS
brew install kind

# Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.22.0/kind-linux-amd64
chmod +x kind
sudo mv kind /usr/local/bin/

# 创建集群
kind create cluster --name crd-dev

# 验证
kubectl cluster-info
kubectl get nodes
```

### 方案 B：minikube

```bash
# macOS
brew install minikube

# 启动（需要 Docker Desktop 或 VirtualBox）
minikube start --driver=docker --memory=4096 --cpus=2

# 验证
kubectl get nodes
```

### 配置文件（可选）

如果使用 kind，可以自定义集群配置：

```yaml
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
```

```bash
kind create cluster --name crd-dev --config kind-config.yaml
```

## 2.5 安装 Kubebuilder

Kubebuilder 是 Kubernetes SIG 官方维护的 Operator 开发框架：

```bash
# macOS / Linux
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
chmod +x kubebuilder
sudo mv kubebuilder /usr/local/bin/

# 验证
kubebuilder version
```

## 2.6 安装 cert-manager（Webhook 需要）

Webhook 需要 TLS 证书，cert-manager 会自动管理：

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.4/cert-manager.yaml

# 等待部署完成
kubectl wait --for=condition=Available deployment/cert-manager-webhook -n cert-manager --timeout=120s
```

## 2.7 初始化项目目录

```bash
mkdir -p ~/redis-operator
cd ~/redis-operator
go mod init github.com/YOUR_USERNAME/redis-operator
```

## 2.8 环境验证清单

运行以下命令确认一切就绪：

```bash
#!/bin/bash
echo "=== 环境检查 ==="

command -v go      >/dev/null && echo "✅ Go: $(go version)"      || echo "❌ Go 未安装"
command -v kubectl >/dev/null && echo "✅ kubectl: $(kubectl version --client --short 2>/dev/null)" || echo "❌ kubectl 未安装"
command -v kind    >/dev/null && echo "✅ kind: 已安装"            || echo "⚠️  kind 未安装 (可选)"
command -v docker  >/dev/null && echo "✅ Docker: $(docker --version)" || echo "❌ Docker 未安装"

# 检查 K8S 集群
kubectl cluster-info >/dev/null 2>&1 && echo "✅ K8S 集群: 可用" || echo "❌ 没有可用的 K8S 集群"
```

## 2.9 本章小结

现在你的机器上应该有：
- Go 编译环境
- 一个本地 K8S 集群（kind 或 minikube）
- kubectl 命令行工具
- Kubebuilder 框架

下一章，我们将不写任何代码，纯用 YAML 定义一个 CRD，理解它的每一个字段。
