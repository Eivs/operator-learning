// Package v1 contains API Schema definitions for the example.com v1 API group.
//
// +kubebuilder:object:generate=true
// +groupName=example.com
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ============================================================================
// AppService — 自定义资源的主类型
// ============================================================================

// AppServiceSpec 定义 AppService 的期望状态 (Desired State)。
// 用户修改 spec 中的字段来告诉 Operator "我想要什么"。
type AppServiceSpec struct {
	// Image 是容器镜像地址，必填字段。
	// +kubebuilder:validation:Required
	// +kubebuilder:example=nginx:latest
	Image string `json:"image"`

	// Replicas 是期望的 Pod 副本数量。
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Port 是服务暴露的端口号。
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=80
	Port int32 `json:"port,omitempty"`

	// Resources 是可选的容器资源限制。
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Env 是可选的环境变量列表。
	// +optional
	Env []EnvVar `json:"env,omitempty"`
}

// ResourceRequirements 定义容器资源需求。
type ResourceRequirements struct {
	// CPU 请求量，如 "100m" 或 "1"。
	// +optional
	CPU string `json:"cpu,omitempty"`

	// Memory 请求量，如 "128Mi" 或 "1Gi"。
	// +optional
	Memory string `json:"memory,omitempty"`
}

// EnvVar 定义环境变量键值对。
type EnvVar struct {
	// Name 是环境变量名。
	Name string `json:"name"`

	// Value 是环境变量值。
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom 从其他来源获取值。
	// +optional
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource 定义环境变量的来源。
type EnvVarSource struct {
	// ConfigMapKeyRef 从 ConfigMap 中获取值。
	// +optional
	ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`

	// SecretKeyRef 从 Secret 中获取值。
	// +optional
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// ConfigMapKeySelector 选择 ConfigMap 中的键。
type ConfigMapKeySelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// SecretKeySelector 选择 Secret 中的键。
type SecretKeySelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// ============================================================================
// AppServiceStatus — 当前状态 (Current State)
// ============================================================================

// AppServiceStatus 定义 AppService 的当前状态。
// 由 Operator 写入，反映底层资源的实际状态。
type AppServiceStatus struct {
	// AvailableReplicas 是当前可用的 Pod 副本数。
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Conditions 表示资源的当前状态条件。
	// 遵循 Kubernetes API 约定。
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// DeploymentName 是关联的 Deployment 名称。
	// +optional
	DeploymentName string `json:"deploymentName,omitempty"`

	// ServiceName 是关联的 Service 名称。
	// +optional
	ServiceName string `json:"serviceName,omitempty"`
}

// ============================================================================
// AppService — 顶层类型
// ============================================================================

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=as
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image",description="容器镜像"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas",description="期望副本数"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas",description="可用副本数"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AppService 是 appservice 自定义资源的 Schema。
// 它代表一个由 Operator 管理的 Web 应用程序。
type AppService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec 定义期望状态。
	Spec AppServiceSpec `json:"spec,omitempty"`

	// Status 定义当前状态，由 Operator 维护。
	Status AppServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppServiceList 包含 AppService 的列表。
type AppServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppService `json:"items"`
}

// 资源类型常量
const (
	// ConditionTypeReady 表示 AppService 是否就绪。
	ConditionTypeReady = "Ready"

	// ConditionReasonDeploymentReady 表示 Deployment 已就绪。
	ConditionReasonDeploymentReady = "DeploymentReady"

	// ConditionReasonDeploymentFailed 表示 Deployment 失败。
	ConditionReasonDeploymentFailed = "DeploymentFailed"

	// ConditionReasonServiceReady 表示 Service 已就绪。
	ConditionReasonServiceReady = "ServiceReady"
)

func init() {
	SchemeBuilder.Register(&AppService{}, &AppServiceList{})
}
