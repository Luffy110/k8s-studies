package webhook

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionreg "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	"k8s.io/client-go/rest"
)

type ValidatingWebHookHandler struct {
	validatingWebHookName string
	caBundle              []byte
}

func NewValidatingWebHookHandler(caBundle []byte, validatingWebHookName string) *ValidatingWebHookHandler {
	mwh := &ValidatingWebHookHandler{
		validatingWebHookName: validatingWebHookName,
		caBundle:              caBundle,
	}
	return mwh
}

func (handler *ValidatingWebHookHandler) UpdateCaBundle() error {
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
		obj.Webhooks[ind].ClientConfig.CABundle = handler.caBundle
	}
	_, err = validating.Update(obj)
	if err != nil {
		//log.Message(log.Error, err.Error())
		return err
	}
	return nil
}
