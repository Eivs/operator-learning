/*
Copyright 2024.

AppService Operator — 一个教学用的 Kubernetes Operator。

这个 Operator 管理 AppService 自定义资源，通过创建和管理底层的
Deployment 和 Service 来实现应用的自动化部署和运维。

使用方法：
  # 本地运行
  go run main.go

  # 构建镜像（需要 Dockerfile）
  docker build -t appservice-operator:v0.1.0 .
*/

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins so that OIDC, AWS, GCP, etc. work.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	appsv1 "github.com/example/appservice-operator/api/v1"
	"github.com/example/appservice-operator/internal/controller"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	// 注册 Kubernetes 内置类型（Deployment, Service, Pod 等）
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// 注册自定义类型（AppService）
	utilruntime.Must(appsv1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	// -----------------------------------------------------------------------
	// 命令行参数
	// -----------------------------------------------------------------------
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// 日志选项
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// -----------------------------------------------------------------------
	// 创建 Manager
	// -----------------------------------------------------------------------
	// Manager 是 controller-runtime 的核心组件，负责：
	//   - 管理所有 Controller 的生命周期
	//   - 提供共享的 Kubernetes 客户端（带缓存）
	//   - 处理 Leader Election（高可用部署）
	//   - 暴露 Metrics 和 Health Probe
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "appservice-operator.example.com",
		// LeaderElectionReleaseOnCancel: true 在开发环境中非常有用：
		//   - 当 Operator 进程被 Ctrl+C 停止时，立即释放 Leader 锁
		//   - 其他副本可以快速接管，避免等待约 15 秒的 Lease 过期
		//   - 生产环境中通常设为 false（默认），避免频繁切换 Leader
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// -----------------------------------------------------------------------
	// 注册 Controller
	// -----------------------------------------------------------------------
	if err = (&controller.AppServiceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AppService")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// -----------------------------------------------------------------------
	// 健康检查
	// -----------------------------------------------------------------------
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// -----------------------------------------------------------------------
	// 启动 Manager（阻塞直到收到退出信号）
	// -----------------------------------------------------------------------
	setupLog.Info("Starting AppService Operator",
		"version", "v0.1.0",
	)
	setupLog.Info("Watching AppService resources",
		"group", "example.com",
		"version", "v1",
	)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
