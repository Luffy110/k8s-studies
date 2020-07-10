package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func errorResponse(w http.ResponseWriter, code int, errStr string) {
	w.WriteHeader(code)
}

type WebhookService struct {
	scheme *runtime.Scheme
	codecs serializer.CodecFactory
}

func NewWebhookService() *WebhookService {
	s := &WebhookService{
		scheme: runtime.NewScheme(),
	}
	corev1.AddToScheme(s.scheme)
	v1beta1.AddToScheme(s.scheme)
	s.codecs = serializer.NewCodecFactory(s.scheme)
	return s
}

func (whsvr *WebhookService) validate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request

	switch req.Kind.Kind {
	case "Deployment":
		isAllow := false
		//TODO: logic handling
		return &v1beta1.AdmissionResponse{
			Allowed: isAllow,
		}
	case "Scale":
		isAllow := false
		//TODO: logic handling
		return &v1beta1.AdmissionResponse{
			Allowed: isAllow,
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Reason: "UnSupported resource",
		},
	}
}

func (service *WebhookService) Server(
	w http.ResponseWriter, r *http.Request) {

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		errorResponse(w, http.StatusUnsupportedMediaType, fmt.Sprintf("Unsupported Content-Type: %s", contentType))
		return
	}
	if r.Body == nil {
		errorResponse(w, http.StatusBadRequest, "Empty request body")
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error reading request body: %s", err.Error()))
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	admissionReview := v1beta1.AdmissionReview{}
	deserializer := service.codecs.UniversalDeserializer()
	if _, _, err = deserializer.Decode(body, nil, &admissionReview); err != nil {
		errorResponse(w, http.StatusBadRequest,
			fmt.Sprintf("Error parsing request body: %s", err.Error()))
		return
	} else {
		admissionResponse = service.validate(&admissionReview)
	}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if admissionReview.Request != nil {
			admissionReview.Response.UID = admissionReview.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError,
			fmt.Sprintf("could not encode response: %s", err.Error()))
	}
	if _, err := w.Write(resp); err != nil {
		errorResponse(w, http.StatusInternalServerError,
			fmt.Sprintf("could not write response: %s", err.Error()))
	}
}
