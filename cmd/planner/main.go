package main

import (
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/ai"
	"agent-flow/internal/api"
	agentflowv1beta1 "agent-flow/internal/apis/agentsxk8sio/v1beta1"
	arch "agent-flow/internal/architecture"
	"agent-flow/internal/cache"
	"agent-flow/internal/config"
	"agent-flow/internal/flow"
	applog "agent-flow/internal/log"
	"agent-flow/internal/store"
)

var (
	scheme       = runtime.NewScheme()
	appVersion   = "1.0.0"
	apiPort      = 8082
	aiConfigPath string
)

func init() {
	// Add Kubernetes builtin types to scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Add custom resource types to scheme
	utilruntime.Must(agentflowiov1alpha1.AddToScheme(scheme))
	utilruntime.Must(agentflowv1beta1.AddToScheme(scheme))
}

func main() {
	// Parse command line flags
	var metricsBindAddr string
	var enableLeaderElection bool
	var healthProbeAddr string
	flag.StringVar(&metricsBindAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager.")
	flag.IntVar(&apiPort, "api-port", 8082, "Chat API port")
	flag.StringVar(&aiConfigPath, "ai-config", "config/ai_config.yaml", "AI config file path")
	flag.Parse()

	applog.InitFromEnv()
	ctrl.SetLogger(logr.FromSlogHandler(applog.Handler()))
	setupLog := ctrl.Log.WithName("setup")

	setupLog.Info("Starting Agent Flow controller manager", "version", appVersion)

	// Load AI config
	aiConfig, err := config.LoadAIConfig(aiConfigPath)
	if err != nil {
		setupLog.Error(err, "Failed to load AI config file", "config", aiConfigPath)
		os.Exit(1)
	}
	setupLog.Info("AI config file loaded successfully", "config", aiConfigPath)

	// Create controller manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsBindAddr,
		},
		HealthProbeBindAddress:        healthProbeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "agentflow.io",
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Unable to start controller manager")
		os.Exit(1)
	}

	// Initialize AI service (before controller registration)
	aiService := ai.NewService(aiConfig)
	if err := aiService.Init(); err != nil {
		setupLog.Error(err, "Failed to initialize AI service")
		os.Exit(1)
	}
	setupLog.Info("AI service initialized successfully")
	setupLog.Info("Planner mode", "mode", aiConfig.GetPlannerConfig().Mode, "model", aiConfig.GetPlannerModel())
	setupLog.Info("Worker mode", "mode", aiConfig.GetWorkerConfig().Mode, "model", aiConfig.GetWorkerModel())
	setupLog.Info("Monitor mode", "mode", aiConfig.GetMonitorConfig().Mode, "model", aiConfig.GetMonitorModel())

	stateStore := cache.NewFromEnv()
	if err := stateStore.Ping(context.Background()); err != nil {
		setupLog.Info("State store ping failed, continuing with fallback behavior", "error", err)
	}

	chatRouter := flow.NewChatRouter(mgr.GetClient(), mgr.GetScheme(), aiService, stateStore)

	if err = (&arch.TaskPlannerEino{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		AIService:  aiService,
		ChatRouter: chatRouter,
		Store:      stateStore,
		Retry:      aiConfig.GetRetryConfig(),
		Quality:    aiConfig.GetQualityConfig(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create task planner controller", "controller", "Task Planner (eino)")
		os.Exit(1)
	}
	setupLog.Info("Task Planner controller (eino) registered")

	novelStore, err := store.OpenFromEnv()
	if err != nil {
		setupLog.Error(err, "Failed to open novel store")
		os.Exit(1)
	}
	defer func() { _ = novelStore.Close() }()
	if err := store.PingStore(context.Background(), novelStore); err != nil {
		setupLog.Info("Novel store ping failed, continuing with file fallback", "error", err)
	} else if novelStore.Enabled() {
		setupLog.Info("Novel store enabled")
	}

	if err = (&arch.WorkflowController{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Store:    novelStore,
		Notifier: chatRouter,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create workflow controller", "controller", "Workflow")
		os.Exit(1)
	}
	setupLog.Info("Workflow controller registered")

	// Add health check
	if err := mgr.AddHealthzCheck("health check", func(r *http.Request) error {
		return nil
	}); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	// Add ready check
	if err := mgr.AddReadyzCheck("ready check", func(r *http.Request) error {
		return nil
	}); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Controller manager started successfully, starting to listen for tasks")

	// Create API server
	chatAPI := api.NewAPI(chatRouter, mgr.GetClient(), novelStore)

	// Start chat API server (in background goroutine)
	go func() {
		if err := chatAPI.StartServer(apiPort); err != nil {
			setupLog.Error(err, "Chat API server failed to start")
		}
	}()
	setupLog.Info("Chat API server started", "port", apiPort)

	// Start controller manager
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running controller manager")
		os.Exit(1)
	}

	setupLog.Info("Controller manager stopped")
}
