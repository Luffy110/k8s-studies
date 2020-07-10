package webhookserver

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	deploymentKind = "Deployment"
	scaleKind      = "Scale"
)

// ValidatingHandler validates deployment,scale
type ValidatingHandler struct {
	Client      client.Client
	decoder     *admission.Decoder
	serviceName string
}

func NewValidatingHandler(c client.Client, servicename string) *ValidatingHandler {
	return &ValidatingHandler{
		Client:      c,
		serviceName: servicename,
	}
}

func (vh *ValidatingHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Kind.Kind {
	case deploymentKind:
		isAllow := false
		if isAllow {
			return admission.Allowed("")
		}
		return admission.Denied(fmt.Sprintf("The %s service doesn't allow update", req.Name))
	case scaleKind:
		isAllow := false
		if isAllow {
			return admission.Allowed("")
		}
		return admission.Denied(fmt.Sprintf("The %s service doesn't allow update", req.Name))
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder.
func (vh *ValidatingHandler) InjectDecoder(d *admission.Decoder) error {
	vh.decoder = d
	return nil
}
