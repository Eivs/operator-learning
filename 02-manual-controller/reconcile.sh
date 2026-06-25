#!/bin/bash
# =============================================================================
# AppService 手动调谐器 (Manual Reconcile Script)
# =============================================================================
# 这是一个教学用的 Shell 控制器，手动执行一次 Reconcile 循环
# 它会：
#   1. 获取所有的 AppService 资源
#   2. 为每个 AppService 创建/更新对应的 Deployment 和 Service
#   3. 将当前状态写回 AppService 的 status 字段
# =============================================================================

set -euo pipefail

APP_GROUP="example.com"
APP_VERSION="v1"
APP_PLURAL="appservices"
NAMESPACE="${NAMESPACE:-default}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# ---------------------------------------------------------------------------
# 1. 创建/更新 Deployment
# ---------------------------------------------------------------------------
reconcile_deployment() {
    local name="$1" image="$2" replicas="$3" cpu="${4:-}" memory="${5:-}"

    local existing
    existing=$(kubectl get deployment "$name" -n "$NAMESPACE" -o name 2>/dev/null || echo "")

    if [[ -z "$existing" ]]; then
        log_info "创建 Deployment: $name (image=$image, replicas=$replicas)"

        # 注意：Shell 控制器不支持设置 OwnerReference 和资源限制
        # OwnerReference 需要知道 CR 的 UID，在 Shell 中处理较为复杂
        # 因此删除 CR 时不会级联删除关联资源（需手动清理）
        # 资源限制同样简化处理 — 完整的实现请参见 Go Operator
        kubectl create deployment "$name" \
            -n "$NAMESPACE" \
            --image="$image" \
            --replicas="$replicas" \
            --dry-run=client -o yaml | \
        kubectl apply -f -

    else
        # 检查是否需要更新
        local current_image current_replicas
        current_image=$(kubectl get deployment "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.template.spec.containers[0].image}')
        current_replicas=$(kubectl get deployment "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.replicas}')

        local needs_update=false

        if [[ "$current_image" != "$image" ]]; then
            log_info "镜像变更: $current_image → $image"
            kubectl set image deployment/"$name" -n "$NAMESPACE" \
                "*=$image"
            needs_update=true
        fi

        if [[ "$current_replicas" != "$replicas" ]]; then
            log_info "副本变更: $current_replicas → $replicas"
            kubectl scale deployment "$name" -n "$NAMESPACE" \
                --replicas="$replicas"
            needs_update=true
        fi

        if [[ "$needs_update" == "false" ]]; then
            log_info "Deployment $name 无需变更"
        fi
    fi
}

# ---------------------------------------------------------------------------
# 2. 创建/更新 Service
# ---------------------------------------------------------------------------
reconcile_service() {
    local name="$1" port="$2"

    local existing
    existing=$(kubectl get svc "$name" -n "$NAMESPACE" -o name 2>/dev/null || echo "")

    if [[ -z "$existing" ]]; then
        log_info "创建 Service: $name (port=$port)"
        kubectl create service clusterip "$name" \
            -n "$NAMESPACE" \
            --tcp="$port:$port" \
            --dry-run=client -o yaml | \
        kubectl apply -f -
    else
        # 只检查端口是否匹配
        local current_port
        current_port=$(kubectl get svc "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.ports[0].port}')
        if [[ "$current_port" != "$port" ]]; then
            log_info "Service 端口变更: $current_port → $port"
            kubectl patch svc "$name" -n "$NAMESPACE" \
                --type json -p "[{\"op\":\"replace\",\"path\":\"/spec/ports/0/port\",\"value\":$port}]"
        else
            log_info "Service $name 无需变更"
        fi
    fi
}

# ---------------------------------------------------------------------------
# 3. 更新 AppService 的 Status
# ---------------------------------------------------------------------------
update_status() {
    local name="$1"

    # 获取 Deployment 的可用副本数
    local available
    available=$(kubectl get deployment "$name" -n "$NAMESPACE" \
        -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
    available=${available:-0}

    # 更新 status
    kubectl patch appservice "$name" -n "$NAMESPACE" \
        --type merge \
        --subresource=status \
        -p "{\"status\":{\"availableReplicas\":$available}}" 2>/dev/null || {
        # 如果不支持 status 子资源（未启用 subresources），跳过
        log_warn "无法更新 status（可能未启用 subresources），跳过"
    }

    log_info "Status 已更新: availableReplicas=$available"
}

# ---------------------------------------------------------------------------
# 主流程：对每个 AppService 执行 Reconcile
# ---------------------------------------------------------------------------
main() {
    log_info "============================================"
    log_info "AppService Reconcile 开始"
    log_info "命名空间: $NAMESPACE"
    log_info "============================================"

    # 获取所有 AppService（按 name 输出）
    local items
    items=$(kubectl get "$APP_PLURAL" -n "$NAMESPACE" \
        -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || echo "")

    if [[ -z "$items" ]]; then
        log_warn "没有找到 AppService 资源"
        log_info "尝试创建一个："
        echo ""
        echo "  kubectl apply -f - <<EOF"
        echo "  apiVersion: $APP_GROUP/$APP_VERSION"
        echo "  kind: AppService"
        echo "  metadata:"
        echo "    name: my-nginx"
        echo "  spec:"
        echo "    image: nginx:latest"
        echo "    replicas: 2"
        echo "    port: 80"
        echo "  EOF"
        exit 0
    fi

    local count=0
    while IFS= read -r name; do
        [[ -z "$name" ]] && continue
        count=$((count + 1))
        echo ""
        log_info ">>> 处理 AppService: $name ($count)"

        # 读取 spec 字段
        local image replicas port cpu memory
        image=$(kubectl get appservice "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.image}')
        replicas=$(kubectl get appservice "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.replicas}')
        port=$(kubectl get appservice "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.port}')
        cpu=$(kubectl get appservice "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.resources.cpu}' 2>/dev/null || echo "")
        memory=$(kubectl get appservice "$name" -n "$NAMESPACE" \
            -o jsonpath='{.spec.resources.memory}' 2>/dev/null || echo "")

        port=${port:-80}

        log_info "  spec.image=$image"
        log_info "  spec.replicas=$replicas"
        log_info "  spec.port=$port"
        [[ -n "$cpu"    ]] && log_info "  spec.resources.cpu=$cpu"
        [[ -n "$memory" ]] && log_info "  spec.resources.memory=$memory"

        # Reconcile Deployment
        reconcile_deployment "$name" "$image" "$replicas" "$cpu" "$memory"

        # Reconcile Service
        reconcile_service "$name" "$port"

        # Update Status
        update_status "$name"

    done <<< "$items"

    echo ""
    log_info "============================================"
    log_info "Reconcile 完成！处理了 $count 个 AppService"
    log_info "============================================"
    echo ""
    log_info "查看结果:"
    echo "  kubectl get appservices"
    echo "  kubectl get deployment,svc,pods"
}

main "$@"
