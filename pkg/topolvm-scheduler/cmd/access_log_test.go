package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	ctrl "sigs.k8s.io/controller-runtime"
)

func getInt64(t *testing.T, m map[string]interface{}, key string) int64 {
	t.Helper()
	i, ok := m[key].(int64)
	if !ok {
		t.Errorf(`i, ok := m[%q].(int64); !ok`, key)
	}
	return i
}
func getString(t *testing.T, m map[string]interface{}, key string) string {
	t.Helper()
	s, ok := m[key].(string)
	if !ok {
		t.Errorf(`s, ok := m[%q].(string); !ok`, key)
	}
	return s
}

func TestAccessLogHandler(t *testing.T) {
	ctx := context.Background()

	observer, logs := observer.New(zap.InfoLevel)
	ctrl.SetLogger(zapr.NewLogger(zap.New(observer)))

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})
	serv := httptest.NewServer(accessLogHandler(ctx, mux))
	defer serv.Close()

	cli := serv.Client()
	_, err := cli.Get(serv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.Get(serv.URL + "/notfound")
	if err != nil {
		t.Fatal(err)
	}

	if logs.Len() != 2 {
		t.Fatal(`len(accessLogs) != 2`)
	}

	helloLog := logs.All()[0].ContextMap()
	notfoundLog := logs.All()[1].ContextMap()

	if getString(t, helloLog, "type") != "access" {
		t.Error(`getString(t, helloLog, "type") != "access"`)
	}
	if getInt64(t, helloLog, "http_status_code") != http.StatusOK {
		t.Error(`getInt(t, helloLog, "http_status_code") != http.StatusOK`)
	}
	if getString(t, helloLog, "http_method") != "GET" {
		t.Error(`getString(t, helloLog, "http_method") != "GET"`)
	}
	if getString(t, helloLog, "url") != "/hello" {
		t.Error(`getString(t, helloLog, "url") != "/hello"`)
	}
	if getString(t, notfoundLog, "url") != "/notfound" {
		t.Error(`getString(t, notfoundLog, "url") != "/notfound"`)
	}
	if getInt64(t, helloLog, "response_size") != 5 {
		t.Error(`getInt(t, helloLog, "response_size") != helloLength`)
	}
}
