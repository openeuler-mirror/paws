/*
Copyright 2020 The Kubernetes Authors.

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

package app

import (
	"context"
	"time"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func Run(s *ServerRunOptions) error {
	config := ctrl.GetConfigOrDie()
	config.QPS = float32(s.ApiServerQPS)
	config.Burst = s.ApiServerBurst

	// Controller Runtime Controllers
	ctrl.SetLogger(klogr.New())

	controllerName := "paws-controllers"

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      s.MetricsAddr,
		Port:                    9443,
		HealthProbeBindAddress:  s.HealthProbeAddr,
		LeaderElection:          s.EnableLeaderElection,
		LeaderElectionID:        controllerName,
		LeaderElectionNamespace: "kube-system",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	ctrlCtx := ctrl.SetupSignalHandler()

	runCtx, cancel := context.WithCancel(ctrlCtx)
	defer cancel()

	if err = (&controllers.UsageTemplateReconciler{
		Log:      ctrl.Log.WithName("reconciler"),
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManager(mgr, controller.Options{
		MaxConcurrentReconciles: s.Workers}, time.Duration(s.TimeoutMinutes)*time.Minute,
		time.Second*time.Duration(s.EvaluationResolutionSeconds), s.PrometheusAddress, runCtx); err != nil {
		setupLog.Error(err, "unable to create reconciler", "controller", "UsageTemplate")
		return err
	}

	setupLog.Info("Controller", "Options", s)

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	setupLog.Info("starting manager...")

	if err := mgr.Start(ctrlCtx); err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	return nil
}
