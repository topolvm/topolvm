package well

import (
	"bytes"
	"context"
	"os/exec"
	"time"
	"unicode/utf8"

	"github.com/cybozu-go/log"
)

// UTF8StringFromBytes returns a valid UTF-8 string from
// maybe invalid slice of bytes.
func UTF8StringFromBytes(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}

	// This effectively replaces invalid bytes to \uFFFD (replacement char).
	return string(bytes.Runes(b))
}

// LogCmd is a wrapper for *exec.Cmd to record command execution results.
// If command fails, log level will be log.LvError.
// If command succeeds, log level will be log.LvInfo.
//
// In most cases, use CommandContext function to prepare LogCmd.
type LogCmd struct {
	*exec.Cmd

	// Severity is used to log successful requests.
	//
	// Zero suppresses logging.  Valid values are one of
	// log.LvDebug, log.LvInfo, and so on.
	//
	// Errors are always logged with log.LvError.
	Severity int

	// Fields is passed to Logger as log fields.
	Fields map[string]interface{}

	// Logger for execution results.  If nil, the default logger is used.
	Logger *log.Logger
}

func (c *LogCmd) log(st time.Time, err error, output []byte) {
	logger := c.Logger
	if logger == nil {
		logger = log.DefaultLogger()
	}

	if err == nil && (c.Severity == 0 || !logger.Enabled(c.Severity)) {
		// successful logs are suppressed if c.Severity is 0 or
		// logger threshold is under c.Severity.
		return
	}

	fields := c.Fields
	fields[log.FnType] = "exec"
	fields[log.FnResponseTime] = time.Since(st).Seconds()
	fields["command"] = c.Cmd.Path
	fields["args"] = c.Cmd.Args

	if err == nil {
		logger.Log(c.Severity, "well: exec", fields)
		return
	}

	fields["error"] = err.Error()
	if len(output) > 0 {
		fields["stderr"] = UTF8StringFromBytes(output)
	}
	logger.Error("well: exec", fields)
}

// CombinedOutput overrides exec.Cmd.CombinedOutput to record the result.
func (c *LogCmd) CombinedOutput() ([]byte, error) {
	st := time.Now()
	data, err := c.Cmd.CombinedOutput()
	c.log(st, err, nil)
	return data, err
}

// Output overrides exec.Cmd.Output to record the result.
// If Cmd.Stderr is nil, Output logs outputs to stderr as well.
func (c *LogCmd) Output() ([]byte, error) {
	st := time.Now()
	data, err := c.Cmd.Output()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if ok {
			c.log(st, err, ee.Stderr)
			return data, err
		}
	}
	c.log(st, err, nil)
	return data, err
}

// Run overrides exec.Cmd.Run to record the result.
// If both Cmd.Stdout and Cmd.Stderr are nil, this calls Output
// instead to log stderr.
func (c *LogCmd) Run() error {
	if c.Cmd.Stdout == nil && c.Cmd.Stderr == nil {
		_, err := c.Output()
		return err
	}

	st := time.Now()
	err := c.Cmd.Run()
	c.log(st, err, nil)
	return err
}

// Wait overrides exec.Cmd.Wait to record the result.
func (c *LogCmd) Wait() error {
	st := time.Now()
	err := c.Cmd.Wait()
	c.log(st, err, nil)
	return err
}

// CommandContext is similar to exec.CommandContext,
// but returns *LogCmd with its Context set to ctx.
//
// LogCmd.Severity is set to log.LvInfo.
//
// LogCmd.Logger is left nil.  If you want to use another logger,
// set it manually.
func CommandContext(ctx context.Context, name string, args ...string) *LogCmd {
	return &LogCmd{
		Cmd:      exec.CommandContext(ctx, name, args...),
		Severity: log.LvInfo,
		Fields:   FieldsFromContext(ctx),
	}
}
