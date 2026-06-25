# 实践指南：在 f2e-k8s-cluster 上逐步操作

本文档带你在真实的 Kubernetes 集群（`f2e-k8s-cluster`）上一步一步实践 CRD 和 Operator。

---

## 预备检查

```bash
# 1. 确认当前集群上下文
kubectl config current-context
# 应该输出: f2e-k8s-cluster

# 2. 确认集群可访问
kubectl cluster-info

# 3. 确认你有足够的权限
kubectl auth can-i create crd
kubectl auth can-i create deployment
kubectl auth can-i create service
```

---

## 场景一：纯 CRD 体验（不需要 Operator 代码）

### Step 1: 部署 CRD

```bash
cd /Users/hy/Dev/crds/01-crd-basics

# 部署 CRD 定义
kubectl apply -f appservice-crd.yaml

# 验证 CRD 已注册
kubectl get crd appservices.example.com -o wide

# 查看可用的 API 资源
kubectl api-resources | grep example.com
```

**预期输出：**
```
appservices    as    example.com/v1    true    AppService
```

### Step 2: 创建自定义资源实例

```bash
# 创建 3 个 AppService 实例
kubectl apply -f appservice-instance.yaml

# 查看你的自定义资源
kubectl get as
# 或
kubectl get appservices

# 查看详情（由于没有 Operator，status 会是空的）
kubectl describe appservice my-nginx
```

**预期输出：**
```
NAME       IMAGE           REPLICAS   PORT   AVAILABLE   AGE
my-api     nginx:alpine    3          8080               10s
my-nginx   nginx:latest    2          80                 10s
my-webapp  nginx:stable    2          3000               10s
```
> 注意：AVAILABLE 列在有 Operator 写入 status 之前会显示为空。

### Step 3: 体验 Schema 校验

```bash
# 尝试创建无效资源（会失败）
kubectl apply -f - <<EOF
apiVersion: example.com/v1
kind: AppService
metadata:
  name: invalid-test
spec:
  replicas: 1
  # 缺少必填字段 image
EOF

# 预期错误输出：
# Error from server (Invalid): ...spec.image in body is required

# 尝试无效值（会失败）
kubectl apply -f - <<EOF
apiVersion: example.com/v1
kind: AppService
metadata:
  name: invalid-test-2
spec:
  image: nginx
  replicas: -1   # 小于最小值 1
  port: 99999    # 超过最大值 65535
EOF

# 预期错误输出：
# Error from server (Invalid): spec.replicas: Invalid value: -1
# Error from server (Invalid): spec.port: Invalid value: 99999
```

### Step 4: CRUD 操作

```bash
# 更新
kubectl patch appservice my-nginx --type merge -p '{"spec":{"replicas":5}}'
kubectl get appservice my-nginx

# 删除单个
kubectl delete appservice my-webapp
kubectl get appservices

# 查看 YAML
kubectl get appservice my-nginx -o yaml
```

---

## 场景二：Shell 控制器体验

### Step 1: 运行手动调谐器

```bash
cd /Users/hy/Dev/crds/02-manual-controller

# 执行调谐脚本
chmod +x reconcile.sh
./reconcile.sh
```

> **观察**：脚本会自动发现所有 AppService 实例，为每个创建 Deployment 和 Service。

### Step 2: 验证自动创建的资源

```bash
# 检查 Deployment
kubectl get deployment
kubectl get deployment my-nginx -o yaml

# 检查 Service
kubectl get svc
kubectl get svc my-nginx -o yaml

# 检查 Pod
kubectl get pods
kubectl get pods -l app=my-nginx
```

### Step 3: 测试变更响应

```bash
# 修改镜像
kubectl patch appservice my-nginx --type merge -p '{"spec":{"image":"nginx:alpine"}}'

# 再次执行 reconcile
./reconcile.sh

# 验证 Deployment 镜像是否更新
kubectl get deployment my-nginx -o jsonpath='{.spec.template.spec.containers[0].image}'

# 修改副本数
kubectl patch appservice my-nginx --type merge -p '{"spec":{"replicas":3}}'
./reconcile.sh

# 验证 Pod 数量
kubectl get pods -l app=my-nginx
```

### Step 4: 清理测试资源

```bash
# 删除 CR
kubectl delete appservice --all

# 注意：关联的 Deployment 和 Service 不会自动删除
# 需要手动清理（这是 Shell 控制器的局限）
kubectl delete deployment --all
kubectl delete svc --all
```

---

## 场景三：Go Operator 体验

### Step 1: 编译 Operator

```bash
cd /Users/hy/Dev/crds/03-go-operator

# 下载依赖
go mod tidy

# 编译
go build -o bin/operator main.go

# 验证
ls -la bin/operator
```

### Step 2: 本地运行 Operator

```bash
# 在一个终端中运行 Operator
go run main.go
```

你会看到类似输出：
```
INFO    setup   Starting AppService Operator    {"version": "v0.1.0"}
INFO    setup   Watching AppService resources   {"group": "example.com", "version": "v1"}
INFO    controller-runtime.metrics  Starting metrics server
INFO    Starting EventSource    {"controller": "appservice", "source": "kind source: *v1.AppService"}
INFO    Starting Controller     {"controller": "appservice"}
INFO    Starting workers        {"controller": "appservice", "worker count": 1}
```

### Step 3: 在另一个终端测试

```bash
# 确保 CRD 已部署
kubectl apply -f /Users/hy/Dev/crds/03-go-operator/config/crd/bases.yaml
# 或者使用之前部署的
kubectl apply -f /Users/hy/Dev/crds/01-crd-basics/appservice-crd.yaml

# 创建 AppService
kubectl apply -f - <<EOF
apiVersion: example.com/v1
kind: AppService
metadata:
  name: go-test
spec:
  image: nginx:latest
  replicas: 2
  port: 8080
EOF

# 观察 Operator 日志（第一个终端），你会看到：
# INFO  Reconciling AppService  {"image": "nginx:latest", "replicas": 2, "port": 8080}
# INFO  Creating Deployment      {"name": "go-test"}
# INFO  Creating Service         {"name": "go-test"}
# INFO  Reconcile completed      {"deployment": "go-test", "service": "go-test"}

# 验证资源已自动创建
kubectl get deployment go-test
kubectl get svc go-test
kubectl get pods -l app=go-test

# 查看 Status 是否被更新
# 方法1：使用 yq（需要单独安装: brew install yq）
kubectl get appservice go-test -o yaml | yq '.status'
# 方法2：纯 kubectl 方式（无需额外工具）
kubectl get appservice go-test -o jsonpath='{.status}' | python3 -m json.tool
```

### Step 4: 测试自动响应

```bash
# 修改副本数（Operator 会自动响应！）
kubectl patch appservice go-test --type merge -p '{"spec":{"replicas":4}}'

# 等几秒，然后检查
kubectl get deployment go-test
# 副本数应该自动变为 4

# 试试删除 Deployment（Operator 会重新创建！）
kubectl delete deployment go-test

# 等几秒，再检查
kubectl get deployment go-test
# Deployment 被自动重建了！

# 这是因为 controller-runtime 的 Owns() Watch
# 当关联的 Deployment 发生变化时，会触发 Reconcile
```

### Step 5: 查看 OwnerReference

```bash
# 查看 Deployment 的 OwnerReference
kubectl get deployment go-test -o jsonpath='{.metadata.ownerReferences}' | python3 -m json.tool

# 应该能看到 AppService 是这个 Deployment 的 Owner
```

### Step 6: 级联删除测试

```bash
# 删除 CR
kubectl delete appservice go-test

# 检查关联资源是否被自动清理
kubectl get deployment go-test  # 应该显示 NotFound
kubectl get svc go-test          # 应该显示 NotFound
```

---

## 场景四：探索你的 K8s 集群中已有的 CRD

```bash
# 列出集群中所有 CRD
kubectl get crd

# 按 API 组分组查看
kubectl get crd --sort-by=.spec.group

# 查看某个 CRD 的详情
kubectl describe crd <crd-name>

# 列出所有非内置的 API 资源
kubectl api-resources --verbs=list -o wide | grep -v "^NAME"
```

---

## 下一步学习

1. 修改 Operator 代码，添加你自己的逻辑
2. 尝试添加 Finalizer 实现优雅清理
3. 添加 Webhook 实现准入校验
4. 将 Operator 打包为 Helm Chart 部署到集群

---

返回 [主教程](./README.md)
