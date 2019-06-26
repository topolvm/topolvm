package hook

import (
	"net/http"

	"github.com/cybozu-go/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type hook struct {
	k8sClient kubernetes.Interface
}

func (h hook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fields := map[string]interface{}{
		log.FnType:        "access",
		log.FnProtocol:    r.Proto,
		log.FnHTTPMethod:  r.Method,
		log.FnURL:         r.RequestURI,
		log.FnHTTPHost:    r.Host,
		log.FnRequestSize: r.ContentLength,
	}

	switch r.URL.Path {
	case "/mutate":
		log.Info("access topolvm-hook", fields)
		h.mutate(w, r)
	case "/status":
		status(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// NewHandler return new http.Handler of the scheduler extender
func NewHandler() (http.Handler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return hook{clientset}, nil
}

func status(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
