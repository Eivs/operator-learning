// Package v1 contains API Schema definitions for the example.com v1 API group.
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion 是用于注册这些对象的 API GroupVersion。
	GroupVersion = schema.GroupVersion{
		Group:   "example.com",
		Version: "v1",
	}

	// SchemeBuilder 用于向 Scheme 添加 Go 类型到 GroupVersionKind。
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme 将本包中的类型添加到 Scheme 中。
	AddToScheme = SchemeBuilder.AddToScheme
)
