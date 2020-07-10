package webhookserver

import (
	"bytes"
	rand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionreg "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

const (
	defaultClusterDomain    = "cluster.local"
	certificateOrganization = "xxx.com"
)

func getClusterDomain(service string) string {
	clusterName, err := net.LookupCNAME(service)
	if err != nil {
		return defaultClusterDomain
	}

	domain := strings.TrimPrefix(clusterName, service)
	domain = strings.Trim(domain, ".")
	if len(domain) == 0 {
		return defaultClusterDomain
	}

	return domain
}

func getSubjectAlternativeNames(service string, namespace string) []string {
	serviceNamespace := strings.Join([]string{service, namespace}, ".")
	serviceNamespaceSvc := strings.Join([]string{serviceNamespace, "svc"}, ".")
	return []string{serviceNamespace, serviceNamespaceSvc,
		strings.Join([]string{serviceNamespaceSvc, getClusterDomain(serviceNamespaceSvc)}, ".")}
}

type CertificateHandler struct {
	sync.Mutex
	tslCert               *tls.Certificate
	cert                  []byte
	key                   []byte
	ca                    []byte
	serviceName           string
	namespace             string
	notAfter              time.Time
	certPath              string
	validatingWebHookName string
}

func newCertificateHandler(serviceName, namespace, certPath, validatingWebHookName string) *CertificateHandler {
	certHandler := &CertificateHandler{
		serviceName:           serviceName,
		namespace:             namespace,
		certPath:              certPath,
		validatingWebHookName: validatingWebHookName,
	}
	return certHandler
}

func UpdateCertificate(deployment ,namespace, certPath, webHookName string) error {
	certHandler := newCertificateHandler(deployment, namespace, certPath,webHookName)
	err := certHandler.loadCertificate()
	if err != nil {
		return err
	}

	// Update ValidatingWebHookConfiguration caBundle
	err = certHandler.updateCaBundle()
	if err != nil {
		return err
	}

	// Watch certificate in background
	go certHandler.renewCertificate()
	return nil
}

func (handler *CertificateHandler) storeCertificate() error {
	certPath := filepath.Join(handler.certPath, "tls.crt")
	err := ioutil.WriteFile(certPath, handler.cert, 0644)
	if err != nil {
		return fmt.Errorf("Cannot write cert to %s: %v", certPath, err)
	}
	keyPath := filepath.Join(handler.certPath, "tls.key")
	err = ioutil.WriteFile(keyPath, handler.key, 0644)
	if err != nil {
		return fmt.Errorf("Cannot write key to %s: %v", keyPath, err)
	}
	return nil
}

func (handler *CertificateHandler) loadCertificate() error {
	handler.Lock()
	defer handler.Unlock()

	err := handler.generateCertificate()
	if err != nil {
		return err
	}
	err = handler.storeCertificate()
	if err != nil {
		return err
	}

	tmpCert, err := tls.X509KeyPair(handler.cert, handler.key)
	if err != nil {
		return err
	}
	handler.tslCert = &tmpCert
	return nil
}

func (handler *CertificateHandler) generateCertificate() error {
	now := time.Now()
	notBefore := now.Add(-time.Hour)
	notAfter := now.Add(time.Hour * 24 * 365)

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: handler.serviceName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return err
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: handler.serviceName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              getSubjectAlternativeNames(handler.serviceName, handler.namespace),
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return err
	}

	// Server cert PEM
	// Add server cert
	serverCertPEM := bytes.Buffer{}
	err = pem.Encode(&serverCertPEM, &pem.Block{Type: certutil.CertificateBlockType, Bytes: serverCertDER})
	if err != nil {
		return err
	}

	// Add ca cert
	err = pem.Encode(&serverCertPEM, &pem.Block{Type: certutil.CertificateBlockType, Bytes: caCertDER})
	if err != nil {
		return err
	}

	// Server key PEM
	serverKeyPEM := bytes.Buffer{}
	err = pem.Encode(&serverKeyPEM, &pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})
	if err != nil {
		return err
	}

	// CA cert PEM
	caCertPEM := bytes.Buffer{}
	err = pem.Encode(&caCertPEM, &pem.Block{Type: certutil.CertificateBlockType, Bytes: caCertDER})
	if err != nil {
		return err
	}

	handler.cert = serverCertPEM.Bytes()
	handler.key = serverKeyPEM.Bytes()
	handler.ca = caCertPEM.Bytes()
	handler.notAfter = notAfter
	return nil
}

func (handler *CertificateHandler) renewCertificate() {
	waitingTime := time.Until(handler.notAfter) - 24*time.Hour
	for {
		time.Sleep(waitingTime)

		err := handler.loadCertificate()
		if err != nil {
			//log.Message(log.Error, err.Error())
			waitingTime = time.Hour
			continue
		}

		err = handler.updateCaBundle()
		if err != nil {
			//log.Message(log.Error, err.Error())
			waitingTime = time.Hour
			continue
		}
		waitingTime = time.Until(handler.notAfter) - 24*time.Hour
	}
}

func (handler *CertificateHandler) updateCaBundle() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		//log.Message(log.Error, err.Error())
		return err
	}
	admissionClient := admissionreg.NewForConfigOrDie(config)
	validating := admissionClient.ValidatingWebhookConfigurations()
	obj, err := validating.Get(handler.validatingWebHookName, metav1.GetOptions{})
	if err != nil {
		//log.Message(log.Error, err.Error())
		return err
	}

	for ind := range obj.Webhooks {
		obj.Webhooks[ind].ClientConfig.CABundle = handler.ca
	}
	_, err = validating.Update(obj)
	if err != nil {
		//log.Message(log.Error, err.Error())
		return err
	}
	return nil
}