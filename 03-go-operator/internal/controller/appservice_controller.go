// Package controller 实现 AppService 的 Reconcile 逻辑。
//
// 这是整个 Operator 的核心。每当你创建、修改或删除一个 AppService 资源时，
// Reconcile 方法都会被调用，确保实际状态与期望状态一致。
package controller

import (
	"context"
	"fmt"

	appsv1 "github.com/example/appservice-operator/api/v1"

	appsv1k8s "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ============================================================================
// AppServiceReconciler — 调谐器
// ============================================================================

// AppServiceReconciler 负责将 AppService 的期望状态同步到集群中。
// 它通过创建和管理底层的 Deployment 和 Service 来实现这一点。
type AppServiceReconciler struct {
	client.Client                 // 嵌入的 k8s 客户端（读/写资源）
	Scheme       *runtime.Scheme  // 类型注册表（用于 OwnerReference 等）
}

// ============================================================================
// RBAC 权限标记（由 controller-gen 生成 RBAC 清单）
// ============================================================================

// +kubebuilder:rbac:groups=example.com,resources=appservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=example.com,resources=appservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=example.com,resources=appservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// ============================================================================
// Reconcile — 核心调谐方法
// ============================================================================

// Reconcile 是 Operator 的核心方法。
//
// 流程：
//   1. 获取 AppService CR
//   2. Reconcile 关联的 Deployment
//   3. Reconcile 关联的 Service
//   4. 更新 AppService 的 Status
//
// 返回值说明：
//   - (Result{}, nil)          → 成功，等待下次 Watch 事件
//   - (Result{Requeue: true}, nil) → 成功后立即重新 Reconcile
//   - (Result{}, err)          → 失败，带指数退避的重试
func (r *AppServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// -----------------------------------------------------------------------
	// 第 1 步：获取 AppService
	// -----------------------------------------------------------------------
	var appService appsv1.AppService
	if err := r.Get(ctx, req.NamespacedName, &appService); err != nil {
		// 如果资源被删除了，直接返回成功（不需要再处理）
		// IgnoreNotFound 会将 NotFound 错误转为 nil
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Reconciling AppService",
		"image", appService.Spec.Image,
		"replicas", appService.Spec.Replicas,
		"port", appService.Spec.Port,
	)

	// -----------------------------------------------------------------------
	// 第 2 步：Reconcile Deployment
	// -----------------------------------------------------------------------
	deploy, err := r.reconcileDeployment(ctx, &appService)
	if err != nil {
		logger.Error(err, "Failed to reconcile Deployment")
		// 更新状态为失败 — 注意：即使 reconcile 失败，
		// 也需要将失败状态写回 API Server，用户才能看到
		updated := appService.DeepCopy()
		meta.SetStatusCondition(&updated.Status.Conditions, metav1.Condition{
			Type:               appsv1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             appsv1.ConditionReasonDeploymentFailed,
			Message:            err.Error(),
			ObservedGeneration: appService.Generation,
		})
		if statusErr := r.Status().Update(ctx, updated); statusErr != nil {
			logger.Error(statusErr, "Failed to update failure status")
		}
		return ctrl.Result{}, err
	}

	// -----------------------------------------------------------------------
	// 第 3 步：Reconcile Service
	// -----------------------------------------------------------------------
	svc, err := r.reconcileService(ctx, &appService)
	if err != nil {
		logger.Error(err, "Failed to reconcile Service")
		// 注意：状态条件可能有多个，这里只设置 Ready 为 False
		// 在真实项目中，可能会分别设置 DeploymentReady 和 ServiceReady 条件
		return ctrl.Result{}, err
	}

	// -----------------------------------------------------------------------
	// 第 4 步：更新 Status
	// -----------------------------------------------------------------------
	if err := r.updateStatus(ctx, &appService, deploy); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconcile completed successfully",
		"deployment", deploy.Name,
		"service", svc.Name,
	)

	return ctrl.Result{}, nil
}

// ============================================================================
// reconcileDeployment — 创建或更新 Deployment
// ============================================================================

func (r *AppServiceReconciler) reconcileDeployment(
	ctx context.Context,
	appService *appsv1.AppService,
) (*appsv1k8s.Deployment, error) {
	logger := log.FromContext(ctx)

	// 构建期望的 Deployment
	desired := r.buildDeployment(appService)

	// 设置 OwnerReference — 关键步骤！
	// 这确保：当 AppService 被删除时，Deployment 也会被自动清理（GC）
	if err := controllerutil.SetControllerReference(appService, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("set owner reference: %w", err)
	}

	// 检查 Deployment 是否已存在
	var existing appsv1k8s.Deployment
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, &existing)

	if err != nil && errors.IsNotFound(err) {
		// --- 不存在 → 创建 ---
		logger.Info("Creating Deployment", "name", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("create deployment: %w", err)
		}
		return desired, nil
	} else if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}

	// --- 已存在 → 检查是否需要更新 ---
	needsUpdate := false

	// 比较副本数
	if *existing.Spec.Replicas != *desired.Spec.Replicas {
		logger.Info("Deployment replicas changed",
			"current", *existing.Spec.Replicas,
			"desired", *desired.Spec.Replicas,
		)
		existing.Spec.Replicas = desired.Spec.Replicas
		needsUpdate = true
	}

	// 比较镜像
	if len(existing.Spec.Template.Spec.Containers) > 0 &&
		len(desired.Spec.Template.Spec.Containers) > 0 {
		currentImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := desired.Spec.Template.Spec.Containers[0].Image
		if currentImage != desiredImage {
			logger.Info("Deployment image changed",
				"current", currentImage,
				"desired", desiredImage,
			)
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			needsUpdate = true
		}
	}

	// 比较标签
	if !labelsMatch(existing.Spec.Template.Labels, desired.Spec.Template.Labels) {
		logger.Info("Deployment labels changed")
		existing.Spec.Template.Labels = desired.Spec.Template.Labels
		existing.Spec.Selector = desired.Spec.Selector
		needsUpdate = true
	}

	if needsUpdate {
		logger.Info("Updating Deployment", "name", desired.Name)
		if err := r.Update(ctx, &existing); err != nil {
			return nil, fmt.Errorf("update deployment: %w", err)
		}
	} else {
		logger.V(1).Info("Deployment is up to date", "name", desired.Name)
	}

	return &existing, nil
}

// ============================================================================
// reconcileService — 创建或更新 Service
// ============================================================================

func (r *AppServiceReconciler) reconcileService(
	ctx context.Context,
	appService *appsv1.AppService,
) (*corev1.Service, error) {
	logger := log.FromContext(ctx)

	desired := r.buildService(appService)

	// 设置 OwnerReference
	if err := controllerutil.SetControllerReference(appService, desired, r.Scheme); err != nil {
		return nil, fmt.Errorf("set owner reference: %w", err)
	}

	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, &existing)

	if err != nil && errors.IsNotFound(err) {
		// --- 不存在 → 创建 ---
		logger.Info("Creating Service", "name", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("create service: %w", err)
		}
		return desired, nil
	} else if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}

	// --- 已存在 → 检查是否需要更新 ---
	needsUpdate := false

	// 比较端口
	if len(existing.Spec.Ports) > 0 && len(desired.Spec.Ports) > 0 {
		if existing.Spec.Ports[0].Port != desired.Spec.Ports[0].Port {
			logger.Info("Service port changed",
				"current", existing.Spec.Ports[0].Port,
				"desired", desired.Spec.Ports[0].Port,
			)
			existing.Spec.Ports = desired.Spec.Ports
			needsUpdate = true
		}
	}

	if needsUpdate {
		logger.Info("Updating Service", "name", desired.Name)
		if err := r.Update(ctx, &existing); err != nil {
			return nil, fmt.Errorf("update service: %w", err)
		}
	} else {
		logger.V(1).Info("Service is up to date", "name", desired.Name)
	}

	return &existing, nil
}

// ============================================================================
// buildDeployment — 从 AppService Spec 构建 Deployment 对象
// ============================================================================

func (r *AppServiceReconciler) buildDeployment(appService *appsv1.AppService) *appsv1k8s.Deployment {
	name := appService.Name
	labels := defaultLabels(appService)
	replicas := appService.Spec.Replicas
	port := appService.Spec.Port
	if port == 0 {
		port = 80
	}

	deploy := &appsv1k8s.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: appService.Namespace,
			Labels:    labels,
		},
		Spec: appsv1k8s.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app", // 使用固定名称，避免 CR 名不符合 DNS-1123 规范
							Image: appService.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							// 健康检查（可选但推荐）
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(port),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
						},
					},
				},
			},
		},
	}

	// 如果指定了资源限制，解析并设置
	if appService.Spec.Resources != nil {
		res := appService.Spec.Resources
		if res.CPU != "" || res.Memory != "" {
			requests := corev1.ResourceList{}
			if res.CPU != "" {
				if q, err := resource.ParseQuantity(res.CPU); err == nil {
					requests[corev1.ResourceCPU] = q
				}
			}
			if res.Memory != "" {
				if q, err := resource.ParseQuantity(res.Memory); err == nil {
					requests[corev1.ResourceMemory] = q
				}
			}
			deploy.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
				Requests: requests,
			}
		}
	}

	// 如果指定了环境变量
	if len(appService.Spec.Env) > 0 {
		envVars := make([]corev1.EnvVar, 0, len(appService.Spec.Env))
		for _, e := range appService.Spec.Env {
			envVar := corev1.EnvVar{Name: e.Name}
			if e.Value != "" {
				envVar.Value = e.Value
			}
			if e.ValueFrom != nil {
				if e.ValueFrom.ConfigMapKeyRef != nil {
					envVar.ValueFrom = &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: e.ValueFrom.ConfigMapKeyRef.Name,
							},
							Key: e.ValueFrom.ConfigMapKeyRef.Key,
						},
					}
				}
				if e.ValueFrom.SecretKeyRef != nil {
					envVar.ValueFrom = &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: e.ValueFrom.SecretKeyRef.Name,
							},
							Key: e.ValueFrom.SecretKeyRef.Key,
						},
					}
				}
			}
			envVars = append(envVars, envVar)
		}
		deploy.Spec.Template.Spec.Containers[0].Env = envVars
	}

	return deploy
}

// ============================================================================
// buildService — 从 AppService Spec 构建 Service 对象
// ============================================================================

func (r *AppServiceReconciler) buildService(appService *appsv1.AppService) *corev1.Service {
	name := appService.Name
	labels := defaultLabels(appService)
	port := appService.Spec.Port
	if port == 0 {
		port = 80
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: appService.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "tcp", // 通用端口名，适用任意端口
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// ============================================================================
// updateStatus — 更新 AppService 的状态
// ============================================================================

func (r *AppServiceReconciler) updateStatus(
	ctx context.Context,
	appService *appsv1.AppService,
	deploy *appsv1k8s.Deployment,
) error {
	// 从 Deployment 状态中获取可用副本数
	availableReplicas := deploy.Status.AvailableReplicas

	// 只在状态真正发生变化时才更新（减少 API 调用）
	if appService.Status.AvailableReplicas == availableReplicas &&
		appService.Status.DeploymentName == deploy.Name {
		// 检查 Condition 是否需要更新
		// （在实际实现中会更复杂）
		return nil
	}

	// 创建深拷贝，避免修改缓存中的对象
	updated := appService.DeepCopy()
	updated.Status.AvailableReplicas = availableReplicas
	updated.Status.DeploymentName = deploy.Name
	updated.Status.ServiceName = appService.Name

	// 设置 Ready Condition
	r.setCondition(ctx, updated, appsv1.ConditionTypeReady,
		metav1.ConditionTrue, appsv1.ConditionReasonDeploymentReady,
		fmt.Sprintf("Deployment %s has %d available replicas", deploy.Name, availableReplicas),
	)

	return r.Status().Update(ctx, updated)
}

// ============================================================================
// setCondition — 更新状态条件（辅助方法）
// ============================================================================

func (r *AppServiceReconciler) setCondition(
	ctx context.Context,
	appService *appsv1.AppService,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	meta.SetStatusCondition(&appService.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: appService.Generation,
	})
}

// ============================================================================
// SetupWithManager — 注册 Controller 到 Manager
// ============================================================================

// SetupWithManager 设置 Watch 并注册 Controller。
//
// For(&appsv1.AppService{}) 表示监听 AppService CR 的变化。
// Owns(&appsv1k8s.Deployment{}) 表示同时监听由本 Controller 创建的 Deployment。
//   当关联的 Deployment 发生变化时（如被手动删除），也会触发 Reconcile。
// Owns(&corev1.Service{}) 同上，监听关联的 Service。
func (r *AppServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.AppService{}).
		Owns(&appsv1k8s.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// ============================================================================
// 辅助函数
// ============================================================================

// defaultLabels 返回一个 AppService 的标准标签集。
func defaultLabels(appService *appsv1.AppService) map[string]string {
	return map[string]string{
		"app":                          appService.Name,
		"app.kubernetes.io/name":       appService.Name,
		"app.kubernetes.io/managed-by": "appservice-operator",
	}
}

// labelsMatch 比较两个标签映射是否相同。
func labelsMatch(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
