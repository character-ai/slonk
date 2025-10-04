/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	uberzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/controller"
	"your-org.com/slonklet/internal/server"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(slonkv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var infoAddr string
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var identifier string
	var logPath string
	var autoRemediate bool
	flag.StringVar(&infoAddr, "info-bind-address", ":8080", "The address the info server binds to")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8082", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&identifier, "identifier", "gpu-uuid-hash", "The value to use to uniquely identify a physical machine.")
	flag.StringVar(&logPath, "log-path", "/var/log/slurm/slonklet-controller.log", "The path to the log file.")
	flag.BoolVar(&autoRemediate, "auto-remediate", true, "Enable auto-remediation of k8s nodes.")

	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(uberzap.NewDevelopmentEncoderConfig()),
		w,
		uberzap.InfoLevel,
	)
	opts := zap.Options{
		ZapOpts: []uberzap.Option{
			uberzap.WrapCore(func(zapcore.Core) zapcore.Core {
				return core
			})},
	}
	// opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if identifier != controller.IDENTIFIER_GPU_UUID_HASH && identifier != controller.IDENTIFIER_PHYSICAL_HOST {
		setupLog.Error(nil, "identifier must be either 'gpu-uuid-hash' or 'physical-host'")
		os.Exit(1)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "your-org-slonklet-controller", // TODO: Replace with your organization identifier
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	nodeReconciler := &controller.PhysicalNodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = nodeReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PhysicalNode")
		os.Exit(1)
	}

	jobReconciler := &controller.SlurmJobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = jobReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create job controller", "controller", "SlurmJob")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	go func() {
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()

	setupLog.Info("starting info server")
	infoServer := server.NewInfoServer(infoAddr)
	go func() {
		setupLog.Error(infoServer.Serve(), "Info server failed")
		os.Exit(1)
	}()

	setupLog.Info("starting sync loop")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	iteration := 0
	for {
		select {
		case <-ticker.C:
			iteration++
			physicalNodeMap, err := nodeReconciler.Sync(
				context.Background(), "", autoRemediate,
			)
			if err != nil {
				setupLog.Error(err, "unable to sync slurm and k8s nodes")
				continue
			}
			if err := infoServer.UpdateNodes(physicalNodeMap); err != nil {
				setupLog.Error(err, "unable to update physical nodes in info server")
				continue
			}

			var slurmJobs map[int]*slonkv1.SlurmJob
			if iteration%4 == 0 {
				iteration = 0

				slurmJobs, err = jobReconciler.Sync(
					context.Background(), "", physicalNodeMap,
				)
				if err != nil {
					setupLog.Error(err, "unable to sync slurm jobs")
					continue
				}
				if err := infoServer.UpdateJobs(slurmJobs); err != nil {
					setupLog.Error(err, "unable to update slurm jobs in info server")
					continue
				}
			}

		}
	}
}
