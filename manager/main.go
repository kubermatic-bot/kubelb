/*
Copyright 2020 Kubermatic GmbH.

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
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	envoycp "k8c.io/kubelb/manager/pkg/envoy"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kubelbk8ciov1alpha1 "k8c.io/kubelb/manager/pkg/api/kubelb.k8c.io/v1alpha1"
	"k8c.io/kubelb/manager/pkg/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = kubelbk8ciov1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	srv := envoycp.Server{}

	flag.StringVar(&srv.ListenAddress, "listen-address", ":8001", "Address to serve envoy control-plane on")
	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// setup signal handler
	ctx, cancel := context.WithCancel(context.Background())
	signalHandler := ctrl.SetupSignalHandler()
	go func() {
		<-signalHandler
		cancel()
	}()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "manager.kubelb.k8c.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create a snapshotCache
	snapshotCache := cachev3.NewSnapshotCache(false, cachev3.IDHash{}, envoycp.Logger{})
	srv.Cache = snapshotCache

	if err := mgr.Add(&srv); err != nil {
		setupLog.Error(err, "failed to register envoy config server with controller-runtime manager")
		os.Exit(1)
	}

	if err = (&controllers.TCPLoadBalancerReconciler{
		Client:     mgr.GetClient(),
		Log:        ctrl.Log.WithName("controllers").WithName("TCPLoadBalancer"),
		Cache:      mgr.GetCache(),
		Scheme:     mgr.GetScheme(),
		EnvoyCache: snapshotCache,
		Ctx:        ctx,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TCPLoadBalancer")
		os.Exit(1)
	}

	if err = (&controllers.HTTPLoadBalancerReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("HTTPLoadBalancer"),
		Scheme: mgr.GetScheme(),
		Ctx:    ctx,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HTTPLoadBalancer")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(signalHandler); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
