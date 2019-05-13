package well

import "time"

// AccessLog is to decode access log records from HTTPServer.
// The struct is tagged for JSON format.
type AccessLog struct {
	Topic    string    `json:"topic"`
	LoggedAt time.Time `json:"logged_at"`
	Severity string    `json:"severity"`
	Utsname  string    `json:"utsname"`
	Message  string    `json:"message"`

	Type           string  `json:"type"`             // "access"
	Elapsed        float64 `json:"response_time"`    // floating point number of seconds.
	Protocol       string  `json:"protocol"`         // "HTTP/1.1" or alike
	StatusCode     int     `json:"http_status_code"` // 200, 404, ...
	Method         string  `json:"http_method"`
	RequestURI     string  `json:"url"`
	Host           string  `json:"http_host"`
	RequestLength  int64   `json:"request_size"`
	ResponseLength int64   `json:"response_size"`
	RemoteAddr     string  `json:"remote_ipaddr"`
	UserAgent      string  `json:"http_user_agent"`
	RequestID      string  `json:"request_id"`
}

// RequestLog is to decode request log from HTTPClient.
// The struct is tagged for JSON format.
type RequestLog struct {
	Topic    string    `json:"topic"`
	LoggedAt time.Time `json:"logged_at"`
	Severity string    `json:"severity"` // "error" if request failed.
	Utsname  string    `json:"utsname"`
	Message  string    `json:"message"`

	Type         string    `json:"type"`             // "http"
	ResponseTime float64   `json:"response_time"`    // floating point number of seconds.
	StatusCode   int       `json:"http_status_code"` // 200, 404, 500, ...
	Method       string    `json:"http_method"`
	URLString    string    `json:"url"`
	StartAt      time.Time `json:"start_at"`
	RequestID    string    `json:"request_id"`
	Error        string    `json:"error"`
}

// ExecLog is a struct to decode command execution log from LogCmd.
// The struct is tagged for JSON format.
type ExecLog struct {
	Topic    string    `json:"topic"`
	LoggedAt time.Time `json:"logged_at"`
	Severity string    `json:"severity"` // "error" if exec failed.
	Utsname  string    `json:"utsname"`
	Message  string    `json:"message"`

	Type      string   `json:"type"`          // "exec"
	Elapsed   float64  `json:"response_time"` // floating point number of seconds.
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	RequestID string   `json:"request_id"`
	Error     string   `json:"error"`
	Stderr    string   `json:"stderr"`
}
