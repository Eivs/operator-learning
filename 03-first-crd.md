# 第 3 章：第一个 CRD

本章我们不写一行 Go 代码，纯用 YAML 理解 CRD 的完整结构。

## 3.1 我们的目标资源

先明确要定义什么。我们想要一个资源类型叫 `Redis`，使用方式是这样的：

```yaml
apiVersion: cache.example.com/v1
kind: Redis
metadata:
  name: my-cache
spec:
  version: "7.0"
  memory: "2Gi"
  replicas: 1
status:
  phase: "Running"
  address: "my-cache.default.svc:6379"
```

## 3.2 CRD 定义详解

CRD 本身也是 K8S 的一个资源（`apiextensions.k8s.io/v1` 组的 `CustomResourceDefinition`）。下面是完整的 CRD 定义，我们逐段拆解。

```yaml
# 01-redis-crd.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  # CRD 的名字必须遵循规则: <plural>.<group>
  name: redis.cache.example.com
spec:
  # 这个 CRD 属于哪个 API 组
  group: cache.example.com

  # 这个 CRD 支持哪些 API 版本
  versions:
    - name: v1
      # 是否作为存储版本
      storage: true
      # 是否为这个版本提供 API
      served: true
      # 这个版本下，CR 的结构定义
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                version:
                  type: string
                  description: "Redis version, e.g. 7.0"
                  # 枚举限制
                  enum: ["6.2", "7.0", "7.2"]
                memory:
                  type: string
                  description: "Memory limit, e.g. 2Gi"
                  pattern: '^\d+(Ki|Mi|Gi)$'
                replicas:
                  type: integer
                  description: "Number of replicas"
                  minimum: 1
                  maximum: 10
                  default: 1
              required: ["version"]
            status:
              type: object
              properties:
                phase:
                  type: string
                  enum:
                    - Pending
                    - Running
                    - Failed
                address:
                  type: string
                  description: "Redis connection address"
          # 哪些字段必须存在
          required: ["spec"]

  # 这个资源的命名空间范围
  # namespaced: 每个 namespace 独立
  # cluster: 整个集群唯一
  scope: Namespaced
  names:
    # 复数形式（用于 API 路径: /apis/cache.example.com/v1/redis）
    plural: redis
    # 单数形式
    singular: redis
    # kind（YAML 中的 kind:）
    kind: Redis
    # kubectl 的简写
    shortNames:
      - rs

  # 版本转换策略
  # None: 不做转换
  # Webhook: 通过 Webhook 转换
  conversion:
    strategy: None
```

## 3.3 逐段拆解

### 3.3.1 group — API 组

```yaml
spec:
  group: cache.example.com
```

- 内置资源的 group 是 `""`（空字符串，也叫 core group），如 Pod、Service
- 自定义资源必须属于某个 group，通常是 `your-domain.com`
- 完整 API 路径：`/apis/cache.example.com/v1/namespaces/<ns>/redis/<name>`

### 3.3.2 versions — 版本管理

```yaml
spec:
  versions:
    - name: v1
      storage: true   # 数据在 etcd 中按这个版本的格式存储
      served: true    # 这个版本是否对外提供 API
```

CRD 支持多版本共存，这是 API 演进的基础：

```yaml
# 同时提供 v1alpha1（试验性）和 v1（稳定版）
versions:
  - name: v1alpha1
    storage: false
    served: true
  - name: v1
    storage: true
    served: true
```

- `storage: true` 只能有一个版本（数据存储格式）
- `served: false` 可以平滑下线旧版本

### 3.3.3 schema — 结构校验

```yaml
spec:
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                version:
                  type: string
                  enum: ["6.2", "7.0", "7.2"]
```

这部分定义了 CR 的 JSON Schema，K8S API Server 在接收请求时会自动校验：

```bash
# 这会失败，因为 version 不在 enum 列表中
kubectl apply -f - <<EOF
apiVersion: cache.example.com/v1
kind: Redis
metadata:
  name: test
spec:
  version: "5.0"   # ❌ 不在 ["6.2", "7.0", "7.2"] 中
EOF

# 错误信息：
# The Redis "test" is invalid: spec.version: Unsupported value: "5.0": supported values: "6.2", "7.0", "7.2"
```

### 3.3.4 scope — 作用域

```yaml
spec:
  scope: Namespaced  # 或 Cluster
```

| scope | 行为 | 典型例子 |
|-------|------|---------|
| Namespaced | 每个 namespace 独立，删除 namespace 时一起删除 | Deployment, Service, Pod |
| Cluster | 整个集群唯一，不绑定 namespace | Node, PersistentVolume, ClusterRole |

### 3.3.5 names — 命名约定

```yaml
spec:
  names:
    plural: redis          # API 路径使用
    singular: redis        # kubectl get <singular>
    kind: Redis            # YAML 中的 kind
    shortNames: ["rs"]     # kubectl get rs
```

## 3.4 动手实践

```bash
# 1. 保存上面的 CRD YAML 并应用到集群
kubectl apply -f 01-redis-crd.yaml

# 2. 查看已注册的 CRD
kubectl get crd
# NAME                     CREATED AT
# redis.cache.example.com  2024-01-15T10:30:00Z

# 3. 查看 CRD 详情
kubectl describe crd redis.cache.example.com

# 4. 查看 CRD 的 API 资源
kubectl api-resources | grep redis
# NAME    SHORTNAMES  APIVERSION              NAMESPACED  KIND
# redis   rs          cache.example.com/v1    true        Redis
```

### 创建第一个 CR（Custom Resource）

```bash
# 5. 创建一个 Redis CR
kubectl apply -f - <<EOF
apiVersion: cache.example.com/v1
kind: Redis
metadata:
  name: my-first-redis
spec:
  version: "7.0"
  memory: "2Gi"
  replicas: 1
EOF

# 6. 查看 CR
kubectl get redis
# NAME            VERSION   REPLICAS   AGE
# my-first-redis  7.0       1          10s

# 7. 查看详细信息（包括 status 默认值）
kubectl describe redis my-first-redis

# 8. 查看 YAML（会看到 status 字段，即使我们没填）
kubectl get redis my-first-redis -o yaml

# 9. 尝试创建不合法的 CR（会失败）
kubectl apply -f - <<EOF
apiVersion: cache.example.com/v1
kind: Redis
metadata:
  name: bad-redis
spec:
  version: "5.0"   # ❌ 校验失败
EOF
```

## 3.5 此时 CR 能做什么？

答案是：**什么也做不了**。它只是一条存在 etcd 中的记录。

```bash
kubectl get redis my-first-redis -o yaml
# spec 中有 version: "7.0"，但并没有真正创建 Redis 实例
```

这就是我们需要 Controller 的原因。控制器负责"看到"CR 的创建/更新/删除，然后执行实际操作。

## 3.6 CRD 的额外字段

### 子资源（Subresources）

```yaml
spec:
  versions:
    - name: v1
      # ... schema ...
      subresources:
        # 启用 /status 子资源（status 更新和 spec 更新权限可分开控制）
        status: {}
        # 启用 /scale 子资源（HPA 需要）
        scale:
          specReplicasPath: .spec.replicas
          statusReplicasPath: .status.replicas
          labelSelectorPath: .status.labelSelector
```

启用 `status` 子资源后：
- `kubectl get redis my-redis -o yaml` 会显示 status 字段
- `kubectl replace` 操作 status 和 spec 需要分开更新
- RBAC 可以分别控制 spec 和 status 的写权限

### 打印列（Additional Printer Columns）

```yaml
spec:
  versions:
    - name: v1
      additionalPrinterColumns:
        - name: Version
          type: string
          jsonPath: .spec.version
        - name: Phase
          type: string
          jsonPath: .status.phase
        - name: Age
          type: date
          jsonPath: .metadata.creationTimestamp
```

这样 `kubectl get redis` 会显示自定义列：

```
NAME            VERSION   PHASE     AGE
my-first-redis  7.0       Running   5m
```

## 3.7 完整的增强版 CRD

结合所有知识点，这是最终的 CRD 定义：

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: redis.cache.example.com
spec:
  group: cache.example.com
  names:
    plural: redis
    singular: redis
    kind: Redis
    shortNames: ["rs"]
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      subresources:
        status: {}
      additionalPrinterColumns:
        - name: Version
          type: string
          jsonPath: .spec.version
        - name: Replicas
          type: integer
          jsonPath: .spec.replicas
        - name: Phase
          type: string
          jsonPath: .status.phase
        - name: Address
          type: string
          jsonPath: .status.address
        - name: Age
          type: date
          jsonPath: .metadata.creationTimestamp
      schema:
        openAPIV3Schema:
          type: object
          required: ["spec"]
          properties:
            spec:
              type: object
              required: ["version"]
              properties:
                version:
                  type: string
                  enum: ["6.2", "7.0", "7.2"]
                memory:
                  type: string
                  pattern: '^\d+(Ki|Mi|Gi)$'
                replicas:
                  type: integer
                  minimum: 1
                  maximum: 10
                  default: 1
            status:
              type: object
              properties:
                phase:
                  type: string
                  enum: ["Pending", "Running", "Failed"]
                address:
                  type: string
```

## 3.8 本章小结

- CRD 就是告诉 K8S "我想注册一个新资源类型"，附带 JSON Schema 做校验
- 光有 CRD，CR 只是 etcd 里的一条数据，不会有任何实际效果
- `scope`、`subresources`、`additionalPrinterColumns` 是常用增强选项
- 下一章，我们深入 Controller 的工作原理
