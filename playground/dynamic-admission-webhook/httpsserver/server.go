package httpsserver

import (
	"crypto/tls"
	"dynamic-admission-webhook/webhook"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func NewServer(port string, whs *webhook.WebhookService, config *tls.Config) *http.Server {

	handler := handler{
		&logic{
			webhookserver: whs,
		},
	}
	h2s := &http2.Server{}

	return &http.Server{
		Addr:      port,
		Handler:   h2c.NewHandler(handler, h2s),
		TLSConfig: config,
	}
}

type logic struct {
	webhookserver *webhook.WebhookService
}

type handler struct {
	logic *logic
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path == "/validate" {
		h.logic.webhookserver.Server(w, r)
		return
	}
}
