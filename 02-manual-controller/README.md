# 02 — 手动控制器：用 Shell 理解 Reconcile 循环

## 什么是 Controller/Operator？

**Controller（控制器）** 是一个循环进程，它持续观察资源的状态，并将"实际状态"调整为"期望状态"。

这个模式叫做 **Reconcile Loop（调谐循环）**：

```mermaid
flowchart TD
    Observe["观察 (Watch)
获取期望状态"] --> Diff["对比 (Diff)
期望 vs 实际"]
    Diff --> Decision{"有差异？"}
    Decision -->|"无差异"| Wait["等待
下次触发"]
    Decision -->|"有差异"| Adjust["调整
创建 / 更新 / 删除资源"]
    Adjust --> Observe
    Wait --> Observe

    style Observe fill:#1565c0,color:#ffffff,stroke:#0d47a1
    style Diff fill:#f57f17,color:#ffffff,stroke:#e65100
    style Decision fill:#c62828,color:#ffffff,stroke:#b71c1c
    style Wait fill:#2e7d32,color:#ffffff,stroke:#1b5e20
    style Adjust fill:#6a1b9a,color:#ffffff,stroke:#4a148c
```

## 关键理解

在 Kubernetes 中，所有的内置控制器都遵循这个模式：

| 资源 | 控制器 | 做什么 |
|------|--------|--------|
| Deployment | Deployment Controller | 创建 ReplicaSet |
| ReplicaSet | ReplicaSet Controller | 创建 Pod |
| Node | Node Controller | 监控节点健康 |
| Service | Endpoints Controller | 管理 Endpoints |

**Operator = CRD + Controller**，只是把这种模式扩展到了自定义资源。

## 实验 2：Shell 控制器

我们将写一个 Shell 脚本，手动执行 Reconcile 循环。这不是生产级方案，但能帮助你深刻理解控制器的工作原理。

### 控制器的职责

当用户创建一个 `AppService` 时，控制器需要：

1. **创建/更新 Deployment** — 根据 `spec.image` 和 `spec.replicas`
2. **创建/更新 Service** — 根据 `spec.port`
3. **更新 Status** — 将实际状态写回 CR 的 `status` 字段

### reconcile.sh 的工作流程

```mermaid
flowchart TD
    Start["获取所有 AppService 资源"] --> ForEach{"对每个 AppService"}

    ForEach --> CheckDeploy{"对应的 Deployment
是否存在？"}
    CheckDeploy -->|"否"| CreateDeploy["创建 Deployment"]
    CheckDeploy -->|"是"| CompareDeploy["比较 replicas / image
如有变更则更新"]
    CreateDeploy --> CheckSvc
    CompareDeploy --> CheckSvc

    CheckSvc{"对应的 Service
是否存在？"}
    CheckSvc -->|"否"| CreateSvc["创建 Service"]
    CheckSvc -->|"是"| CompareSvc["比较 port
如有变更则更新"]
    CreateSvc --> UpdateStatus
    CompareSvc --> UpdateStatus

    UpdateStatus["更新 status.availableReplicas"]

    style Start fill:#1565c0,color:#ffffff,stroke:#0d47a1
    style ForEach fill:#f57f17,color:#ffffff,stroke:#e65100
    style CheckDeploy fill:#c62828,color:#ffffff,stroke:#b71c1c
    style CheckSvc fill:#c62828,color:#ffffff,stroke:#b71c1c
    style CreateDeploy fill:#2e7d32,color:#ffffff,stroke:#1b5e20
    style CompareDeploy fill:#0d47a1,color:#ffffff,stroke:#1565c0
    style CreateSvc fill:#2e7d32,color:#ffffff,stroke:#1b5e20
    style CompareSvc fill:#0d47a1,color:#ffffff,stroke:#1565c0
    style UpdateStatus fill:#6a1b9a,color:#ffffff,stroke:#4a148c
```

### 步骤 1：部署 CRD

```bash
# 确保 CRD 已部署
kubectl apply -f ../01-crd-basics/appservice-crd.yaml

# 创建测试实例
kubectl apply -f - <<EOF
apiVersion: example.com/v1
kind: AppService
metadata:
  name: my-nginx
spec:
  image: nginx:latest
  replicas: 2
  port: 80
EOF
```

### 步骤 2：运行 Shell 控制器

```bash
# 先看一下 reconcile 脚本
cat reconcile.sh

# 手动执行一次 reconcile
chmod +x reconcile.sh
./reconcile.sh
```

### 步骤 3：验证结果

```bash
# 检查 Deployment 是否被创建
kubectl get deployment

# 检查 Service 是否被创建
kubectl get svc

# 检查 Pod
kubectl get pods

# 查看 CR 的 status 是否被更新
# 提示：yq 是可选工具（brew install yq），也可用 kubectl 自带 jsonpath：
kubectl get appservice my-nginx -o yaml | yq '.status'
# 或者：
kubectl get appservice my-nginx -o jsonpath='{.status}'
```

如果你看到 Deployment 和 Service 被自动创建，说明你的第一个控制器工作正常！

### 步骤 4：测试变更响应

```bash
# 修改副本数从 2 → 3
kubectl patch appservice my-nginx --type merge -p '{"spec":{"replicas":3}}'

# 再次运行 reconcile
./reconcile.sh

# 检查 Deployment 副本数是否更新
kubectl get deployment my-nginx

# 修改镜像
kubectl patch appservice my-nginx --type merge -p '{"spec":{"image":"nginx:alpine"}}'

# 再次 reconcile
./reconcile.sh

# 检查 Deployment 镜像是否更新
kubectl get deployment my-nginx -o yaml | yq '.spec.template.spec.containers[0].image'
```

### 步骤 5：测试删除

```bash
# 删除 AppService CR
kubectl delete appservice my-nginx

# 注意：Shell 控制器不会自动清理关联资源
# 这就是 OwnerReference 的用途（后面会讲）
kubectl get deployment
kubectl get svc

# 手动清理
kubectl delete deployment my-nginx
kubectl delete svc my-nginx
```

## Shell 控制器的局限性

| 问题 | 说明 |
|------|------|
| ❌ 非实时 | 需要手动执行，不能自动 Watch |
| ❌ 无故障恢复 | 脚本失败不会重试 |
| ❌ 无并发安全 | 多实例运行可能冲突 |
| ❌ 无所有权管理 | 删除 CR 不会自动清理关联资源 |
| ❌ 无 Finalizer | 无法实现清理逻辑 |

这些正是我们在下一章要用 Go Operator 解决的问题。

## 控制器核心模式总结

```mermaid
flowchart TD
    S1["① 获取期望状态
从 CR 的 spec 字段读取"] --> S2
    S2["② 获取实际状态
从底层资源中读取"] --> S3
    S3{"③ 对比差异
期望 ≠ 实际？"} -->|"有差异"| S4
    S3 -->|"无差异"| S6
    S4["④ 执行操作
创建 / 更新 / 删除底层资源"] --> S5
    S5["⑤ 更新状态
将结果写回 CR status"] --> S6
    S6["⑥ 返回并等待
由下次 Watch 事件触发"]

    style S1 fill:#1565c0,color:#ffffff,stroke:#0d47a1
    style S2 fill:#2e7d32,color:#ffffff,stroke:#1b5e20
    style S3 fill:#c62828,color:#ffffff,stroke:#b71c1c
    style S4 fill:#e65100,color:#ffffff,stroke:#bf360c
    style S5 fill:#6a1b9a,color:#ffffff,stroke:#4a148c
    style S6 fill:#006064,color:#ffffff,stroke:#004d4d
```

---

下一章：[03-Go Operator 实战](../03-go-operator/README.md) — 构建生产级 Operator
