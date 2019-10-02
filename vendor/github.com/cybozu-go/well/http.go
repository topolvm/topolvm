// +build go1.8

package well

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/netutil"
)

const (
	defaultHTTPReadTimeout = 30 * time.Second

	// request tracking header.
	defaultRequestIDHeader = "X-Cybozu-Request-ID"

	requestIDHeaderEnv = "REQUEST_ID_HEADER"
)

var (
	requestIDHeader = defaultRequestIDHeader
)

func init() {
	hn := os.Getenv(requestIDHeaderEnv)
	if len(hn) > 0 {
		requestIDHeader = hn
	}
}

// HTTPServer is a wrapper for http.Server.
//
// This struct overrides Serve and ListenAndServe* methods.
//
// http.Server members are replaced as following:
//    - Handler is replaced with a wrapper handler that logs requests.
//    - ReadTimeout is set to 30 seconds if it is zero.
//    - ConnState is replaced with the one provided by the framework.
type HTTPServer struct {
	*http.Server

	// AccessLog is a logger for access logs.
	// If this is nil, the default logger is used.
	AccessLog *log.Logger

	// ShutdownTimeout is the maximum duration the server waits for
	// all connections to be closed before shutdown.
	//
	// Zero duration disables timeout.
	ShutdownTimeout time.Duration

	// Env is the environment where this server runs.
	//
	// The global environment is used if Env is nil.
	Env *Environment

	handler     http.Handler
	shutdownErr error
	generator   *IDGenerator

	initOnce sync.Once
}

// StdResponseWriter is the interface implemented by
// the ResponseWriter from http.Server for non-HTTP/2 requests.
//
// HTTPServer's ResponseWriter implements this as well.
type StdResponseWriter interface {
	http.ResponseWriter
	io.ReaderFrom
	http.Flusher
	http.CloseNotifier
	http.Hijacker
	WriteString(data string) (int, error)
}

// StdResponseWriter2 is the interface implemented by
// the ResponseWriter from http.Server for HTTP/2 requests.
//
// HTTPServer's ResponseWriter implements this as well.
type StdResponseWriter2 interface {
	http.ResponseWriter
	http.Flusher
	http.CloseNotifier
	http.Pusher
	WriteString(data string) (int, error)
}

type logWriter interface {
	Status() int
	Size() int64
}

type logResponseWriter struct {
	StdResponseWriter
	status int
	size   int64
}

func (w *logResponseWriter) WriteHeader(status int) {
	w.status = status
	w.StdResponseWriter.WriteHeader(status)
}

func (w *logResponseWriter) Write(data []byte) (int, error) {
	n, err := w.StdResponseWriter.Write(data)
	w.size += int64(n)
	return n, err
}

func (w *logResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	n, err := w.StdResponseWriter.ReadFrom(r)
	w.size += n
	return n, err
}

func (w *logResponseWriter) WriteString(data string) (int, error) {
	n, err := w.StdResponseWriter.WriteString(data)
	w.size += int64(n)
	return n, err
}

func (w *logResponseWriter) Status() int {
	return w.status
}

func (w *logResponseWriter) Size() int64 {
	return w.size
}

type logResponseWriter2 struct {
	StdResponseWriter2
	status int
	size   int64
}

func (w *logResponseWriter2) WriteHeader(status int) {
	w.status = status
	w.StdResponseWriter2.WriteHeader(status)
}

func (w *logResponseWriter2) Write(data []byte) (int, error) {
	n, err := w.StdResponseWriter2.Write(data)
	w.size += int64(n)
	return n, err
}

func (w *logResponseWriter2) WriteString(data string) (int, error) {
	n, err := w.StdResponseWriter2.WriteString(data)
	w.size += int64(n)
	return n, err
}

func (w *logResponseWriter2) Status() int {
	return w.status
}

func (w *logResponseWriter2) Size() int64 {
	return w.size
}

func createLogWriter(w http.ResponseWriter) (http.ResponseWriter, logWriter) {
	if srw1, ok := w.(StdResponseWriter); ok {
		t := &logResponseWriter{srw1, http.StatusOK, 0}
		return t, t
	}

	if srw2, ok := w.(StdResponseWriter2); ok {
		t := &logResponseWriter2{srw2, http.StatusOK, 0}
		return t, t
	}

	panic("unexpected ResponseWriter implementation")
}

// ServeHTTP implements http.Handler interface.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	w, lw := createLogWriter(w)

	ctx, cancel := context.WithCancel(s.Env.ctx)
	defer cancel()

	reqid := r.Header.Get(requestIDHeader)
	if len(reqid) == 0 {
		reqid = s.generator.Generate()
	}
	ctx = WithRequestID(ctx, reqid)

	s.handler.ServeHTTP(w, r.WithContext(ctx))
	status := lw.Status()

	fields := map[string]interface{}{
		log.FnType:           "access",
		log.FnResponseTime:   time.Since(startTime).Seconds(),
		log.FnProtocol:       r.Proto,
		log.FnHTTPStatusCode: status,
		log.FnHTTPMethod:     r.Method,
		log.FnURL:            r.RequestURI,
		log.FnHTTPHost:       r.Host,
		log.FnRequestSize:    r.ContentLength,
		log.FnResponseSize:   lw.Size(),
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		fields[log.FnRemoteAddress] = ip
	}
	ua := r.Header.Get("User-Agent")
	if len(ua) > 0 {
		fields[log.FnHTTPUserAgent] = ua
	}
	if len(reqid) > 0 {
		fields[log.FnRequestID] = reqid
	}

	lv := log.LvInfo
	switch {
	case 500 <= status:
		lv = log.LvError
	case 400 <= status:
		lv = log.LvWarn
	}
	s.AccessLog.Log(lv, "well: access", fields)
}

func (s *HTTPServer) init() {
	s.generator = NewIDGenerator()

	if s.Server.Handler == nil {
		panic("Handler must not be nil")
	}
	s.handler = s.Server.Handler
	s.Server.Handler = s
	if s.Server.ReadTimeout == 0 {
		s.Server.ReadTimeout = defaultHTTPReadTimeout
	}

	if s.AccessLog == nil {
		s.AccessLog = log.DefaultLogger()
	}

	if s.Env == nil {
		s.Env = defaultEnv
	}

	s.Env.Go(s.wait)
}

func (s *HTTPServer) wait(ctx context.Context) error {
	<-ctx.Done()

	s.Server.SetKeepAlivesEnabled(false)

	ctx = context.Background()
	if s.ShutdownTimeout != 0 {
		ctx2, cancel := context.WithTimeout(ctx, s.ShutdownTimeout)
		defer cancel()
		ctx = ctx2
	}

	err := s.Server.Shutdown(ctx)
	if err != nil {
		log.Warn("well: unclean shutdown", map[string]interface{}{
			log.FnError: err,
		})
		s.shutdownErr = err
	}
	return err
}

// TimedOut returns true if the server shut down before all connections
// got closed.
func (s *HTTPServer) TimedOut() bool {
	return s.shutdownErr == context.DeadlineExceeded
}

// Serve overrides http.Server's Serve method.
//
// Unlike the original, this method returns immediately just after
// starting a goroutine to accept connections.
//
// The framework automatically closes l when the environment's Cancel
// is called.
//
// Serve always returns nil.
func (s *HTTPServer) Serve(l net.Listener) error {
	s.initOnce.Do(s.init)

	l = netutil.KeepAliveListener(l)

	go func() {
		s.Server.Serve(l)
	}()

	return nil
}

// ListenAndServe overrides http.Server's method.
//
// Unlike the original, this method returns immediately just after
// starting a goroutine to accept connections.  To stop listening,
// call the environment's Cancel.
//
// ListenAndServe returns non-nil error if and only if net.Listen failed.
func (s *HTTPServer) ListenAndServe() error {
	addr := s.Server.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// ListenAndServeTLS overrides http.Server's method.
//
// Unlike the original, this method returns immediately just after
// starting a goroutine to accept connections.  To stop listening,
// call the environment's Cancel.
//
// Another difference from the original is that certFile and keyFile
// must be specified.  If not, configure http.Server.TLSConfig
// manually and use Serve().
//
// HTTP/2 is always enabled.
//
// ListenAndServeTLS returns non-nil error if net.Listen failed
// or failed to load certificate files.
func (s *HTTPServer) ListenAndServeTLS(certFile, keyFile string) error {
	addr := s.Server.Addr
	if addr == "" {
		addr = ":https"
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	config := &tls.Config{
		NextProtos:               []string{"h2", "http/1.1"},
		Certificates:             []tls.Certificate{cert},
		PreferServerCipherSuites: true,
		ClientSessionCache:       tls.NewLRUClientSessionCache(0),
	}
	s.Server.TLSConfig = config

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(ln, config)
	return s.Serve(tlsListener)
}

// HTTPClient is a thin wrapper for *http.Client.
//
// This overrides Do method to add the request tracking header if
// the passed request's context brings a request tracking ID.  Do
// also records the request log to Logger.
//
// Do not use Get/Head/Post/PostForm.  They panics.
type HTTPClient struct {
	*http.Client

	// Severity is used to log successful requests.
	//
	// Zero suppresses logging.  Valid values are one of
	// log.LvDebug, log.LvInfo, and so on.
	//
	// Errors are always logged with log.LvError.
	Severity int

	// Logger for HTTP request.  If nil, the default logger is used.
	Logger *log.Logger
}

// Do overrides http.Client.Do.
//
// req's context should have been set by http.Request.WithContext
// for request tracking and context-based cancelation.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	v := ctx.Value(RequestIDContextKey)
	if v != nil {
		req.Header.Set(requestIDHeader, v.(string))
	}
	st := time.Now()
	resp, err := c.Client.Do(req)

	logger := c.Logger
	if logger == nil {
		logger = log.DefaultLogger()
	}

	if err == nil && (c.Severity == 0 || !logger.Enabled(c.Severity)) {
		// successful logs are suppressed if c.Severity is 0 or
		// logger threshold is under c.Severity.
		return resp, err
	}

	fields := FieldsFromContext(ctx)
	fields[log.FnType] = "http"
	fields[log.FnResponseTime] = time.Since(st).Seconds()
	fields[log.FnHTTPMethod] = req.Method
	fields[log.FnURL] = req.URL.String()
	fields[log.FnStartAt] = st

	if err != nil {
		fields["error"] = err.Error()
		logger.Error("well: http", fields)
		return resp, err
	}

	fields[log.FnHTTPStatusCode] = resp.StatusCode
	logger.Log(c.Severity, "well: http", fields)
	return resp, err
}

// Get panics.
func (c *HTTPClient) Get(url string) (*http.Response, error) {
	panic("Use Do")
}

// Head panics.
func (c *HTTPClient) Head(url string) (*http.Response, error) {
	panic("Use Do")
}

// Post panics.
func (c *HTTPClient) Post(url, bodyType string, body io.Reader) (*http.Response, error) {
	panic("Use Do")
}

// PostForm panics.
func (c *HTTPClient) PostForm(url string, data url.Values) (*http.Response, error) {
	panic("Use Do")
}
