# Kubernetes CRD Operator 从入门到实战

## 适合人群

- 对 Kubernetes 有基本了解（知道 Pod、Deployment、Service）
- 有一定的 Go 语言基础
- 想学习如何扩展 Kubernetes 的开发者

## 你将学到

- 亲手创建 Custom Resource Definition（CRD）
- 理解 Controller/Operator 的工作原理
- 使用 Kubebuilder 框架构建完整的 Operator
- 实现 Webhook 验证和默认值
- 编写测试并部署到真实集群

## 目录

| 章节 | 标题 | 核心内容 |
|------|------|----------|
| 1 | [基础概念](./01-basic-concepts.md) | CRD 是什么、声明式 API、Operator 模式 |
| 2 | [环境准备](./02-environment-setup.md) | kind/minikube、Go、kubebuilder 安装 |
| 3 | [第一个 CRD](./03-first-crd.md) | YAML 定义 CRD、kubectl 创建资源 |
| 4 | [Controller 原理](./04-controller-principle.md) | Reconcile Loop、Informer、WorkQueue |
| 5 | [Kubebuilder 入门](./05-kubebuilder-setup.md) | 脚手架搭建、项目结构说明 |
| 6 | [设计 API](./06-api-design.md) | Spec/Status 设计、多版本 API |
| 7 | [实现 Controller](./07-controller-implementation.md) | Reconcile 逻辑、子资源管理 |
| 8 | [Webhook 验证](./08-webhook.md) | Validating/Mutating Webhook |
| 9 | [测试与调试](./09-testing-debugging.md) | EnvTest、本地调试技巧 |
| 10 | [部署与发布](./10-deployment.md) | Docker 镜像、Helm、OLM |

## 最终项目

我们将构建一个 **Redis Operator**，它能自动管理 Redis 实例的创建、扩缩容和备份。

```bash
# 最终效果：创建一条 YAML 即可拥有一个完整的 Redis 集群
kubectl apply -f - <<EOF
apiVersion: cache.example.com/v1
kind: Redis
metadata:
  name: my-redis
spec:
  replicas: 3
  version: "7.0"
  storage: 10Gi
EOF
```
