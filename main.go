package main

import (
	"flag"
	"os"

	"github.com/maisem/tailscale-operator/pkg/controller"
	controllers "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	tailscaleImage = flag.String("tailscale-image", "", "Image to use for the Tailscale StatefulSet")
)

func main() {
	controllers.SetLogger(zap.New())
	var log = controllers.Log.WithName("tailscale")

	manager, err := controllers.NewManager(controllers.GetConfigOrDie(), controllers.Options{
		Logger: log,
	})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	if err := controller.New(manager); err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	if err := manager.Start(controllers.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}
