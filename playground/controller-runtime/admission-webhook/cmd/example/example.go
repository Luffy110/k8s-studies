package main

import (
	"controller-runtime/admission-webhook/webhookserver"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	validatingWebHookName = flag.String("validatingwebhookname", "example", "ValidatingWebHookConfiguration name")
	webhookServerPort     = flag.Int("webhookserverport", 10000, "ValidatingWebHookConfiguration server port")
	namespace             = flag.String("namespace", "default", "namespace")
	serviceName           = flag.String("servicename", "example", "deployment name")
	certPath              = flag.String("certpath", "/opt/certs", "Certificates dir")
)

func main() {

	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	defer close(signalChan)

	go func() {
		for {
			abort := <-signalChan
			if abort == syscall.SIGTERM {
				log.Println("Receive SIGTERM signal .Service Shutdown")
				//log.Message(log.Error, "Receive SIGTERM signal .Service Shutdown")
				break
			}
		}
	}()

	// Update certificates
	err := webhookserver.UpdateCertificate(*serviceName, *namespace, *certPath, *validatingWebHookName)
	if err != nil {
		panic(err.Error())
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Port:    *webhookServerPort,
		CertDir: *certPath,
	})
	if err != nil {
		panic(err.Error())
	}

	hookServer := mgr.GetWebhookServer()
	hookServer.Register("/validate", &webhook.Admission{
		Handler: webhookserver.NewValidatingHandler(mgr.GetClient(), *serviceName),
	})

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		panic(err.Error())
	}
}
