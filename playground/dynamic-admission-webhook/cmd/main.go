package main

import (
	"crypto/tls"
	"dynamic-admission-webhook/httpsserver"
	"dynamic-admission-webhook/webhook"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var (
	validatingWebHookName = flag.String("validatingwebhookname", "example", "ValidatingWebHookConfiguration name")
	webhookServerPort     = flag.Uint("webhookserverport", 10000, "ValidatingWebHookConfiguration server port")
	namespace             = flag.String("namespace", "default", "namespace")
	serviceName           = flag.String("servicename", "example", "deployment name")
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

	// Create certificates
	certHandler := webhook.NewCertificateHandler(*serviceName, *namespace)
	err := certHandler.LoadCertificate()
	if err != nil {
		panic(err.Error())
	}

	// Update MutatingWebHookConfiguration caBundle
	mwhHandler := webhook.NewValidatingWebHookHandler(certHandler.GetCACert(), *validatingWebHookName)
	err = mwhHandler.UpdateCaBundle()
	if err != nil {
		panic(err.Error())
	}

	// Watch certificate in background
	go certHandler.RenewCertificate(mwhHandler)

	// Set up server
	config := &tls.Config{
		GetCertificate: certHandler.GetCertificate,
		ClientAuth:     tls.NoClientCert,
	}

	whs := webhook.NewWebhookService()
	s := httpsserver.NewServer(fmt.Sprintf(":%d", *webhookServerPort), whs, config)

	if err := s.ListenAndServeTLS("", ""); err != nil {
		log.Printf("Error while listening webhook %s", err.Error())
	}
}
