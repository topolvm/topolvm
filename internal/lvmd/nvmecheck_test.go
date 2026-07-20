package lvmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// writeStubTool writes an executable shell script with the given body.
func writeStubTool(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "nvme-check")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0755); err != nil {
		t.Fatalf("failed to write stub tool: %v", err)
	}
	return path
}

// echoResult prints "<device>: <n>" like the real tool ($1 is the device path
// since the stub takes no fixed args).
func echoResult(n string) string { return `echo "$1: ` + n + `"` }

func TestRunNVMeCheckDevice(t *testing.T) {
	ctx := context.Background()
	const dev = "/dev/nvme0n1"

	tests := []struct {
		name     string
		cmd      func(t *testing.T) []string
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name:    "result 1 is supported",
			cmd:     func(t *testing.T) []string { return []string{writeStubTool(t, echoResult("1"))} },
			wantErr: false,
		},
		{
			name:     "result 0 is unsupported",
			cmd:      func(t *testing.T) []string { return []string{writeStubTool(t, echoResult("0"))} },
			wantErr:  true,
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "unexpected result is an error",
			cmd:      func(t *testing.T) []string { return []string{writeStubTool(t, echoResult("9"))} },
			wantErr:  true,
			wantCode: codes.Internal,
		},
		{
			name:     "unparseable output is an error",
			cmd:      func(t *testing.T) []string { return []string{writeStubTool(t, `echo "no number here"`)} },
			wantErr:  true,
			wantCode: codes.Internal,
		},
		{
			name:     "empty output is an error",
			cmd:      func(t *testing.T) []string { return []string{writeStubTool(t, `exit 0`)} },
			wantErr:  true,
			wantCode: codes.Internal,
		},
		{
			name:     "missing binary is an error",
			cmd:      func(t *testing.T) []string { return []string{filepath.Join(t.TempDir(), "does-not-exist")} },
			wantErr:  true,
			wantCode: codes.Internal,
		},
		{
			name:    "non-zero exit with valid output is honored",
			cmd:     func(t *testing.T) []string { return []string{writeStubTool(t, echoResult("1")+"; exit 3")} },
			wantErr: false,
		},
		{
			name: "command prefix args are forwarded",
			cmd: func(t *testing.T) []string {
				return []string{
					writeStubTool(t, `if [ "$1" != "support_inference" ]; then echo "bad: $1"; exit 0; fi; echo "$2: 1"`),
					"support_inference",
				}
			},
			wantErr: false,
		},
		{
			// The device must reach the tool WITHOUT a "/dev/" prefix; the stub
			// reports 9 (error) if it still sees one.
			name: "device is passed without /dev/ prefix",
			cmd: func(t *testing.T) []string {
				return []string{writeStubTool(t, `case "$1" in /dev/*) echo "$1: 9";; *) echo "$1: 1";; esac`)}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runNVMeCheckDevice(ctx, tt.cmd(t), dev)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				if got := status.Code(err); got != tt.wantCode {
					t.Fatalf("expected code %v, got %v (err=%v)", tt.wantCode, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestParseNVMeCheckOutput(t *testing.T) {
	cases := map[string]struct {
		in      string
		want    int
		wantErr bool
	}{
		"simple":      {"/dev/nvme0n1: 1", 1, false},
		"zero":        {"/dev/sda: 0", 0, false},
		"trailing nl": {"/dev/sda: 4\n", 4, false},
		"extra lines": {"loading...\n/dev/sda: 1\n", 1, false},
		"no number":   {"hello", 0, true},
		"empty":       {"", 0, true},
		"blank lines": {"\n\n", 0, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseNVMeCheckOutput([]byte(c.in))
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

// fakeChecker builds an nvmeChecker whose run() returns errs[i] on the i-th
// call (last value repeats) and counts invocations.
func fakeChecker(maxAttempts int, errs ...error) (*nvmeChecker, *int) {
	calls := 0
	c := &nvmeChecker{
		cmd:         []string{"stub"},
		maxAttempts: maxAttempts,
		cache:       map[string]*nvmeCheckEntry{},
	}
	c.run = func(_ context.Context, _ []string, _ string) error {
		i := calls
		calls++
		if i >= len(errs) {
			i = len(errs) - 1
		}
		return errs[i]
	}
	return c, &calls
}

func TestNVMeChecker_PositiveIsCached(t *testing.T) {
	ctx := context.Background()
	c, calls := fakeChecker(3, nil) // always supported

	for i := 0; i < 5; i++ {
		if err := c.check(ctx, "/dev/nvme0n1"); err != nil {
			t.Fatalf("call %d: unexpected error %v", i, err)
		}
	}
	if *calls != 1 {
		t.Fatalf("expected tool to run once (cached afterwards), ran %d times", *calls)
	}
}

func TestNVMeChecker_GivesUpAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	blocked := status.Error(codes.FailedPrecondition, "NVMe device /dev/nvme0n1 does not support this feature")
	c, calls := fakeChecker(3, blocked)

	// First 3 calls each run the tool and return an error.
	for i := 1; i <= 3; i++ {
		err := c.check(ctx, "/dev/nvme0n1")
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("attempt %d: expected FailedPrecondition, got %v", i, err)
		}
	}
	// Further calls must NOT run the tool again and must return a final error.
	for i := 0; i < 3; i++ {
		err := c.check(ctx, "/dev/nvme0n1")
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("post-final call: expected FailedPrecondition, got %v", err)
		}
		if msg := status.Convert(err).Message(); !strings.Contains(msg, "giving up") {
			t.Fatalf("expected final message to mention giving up, got %q", msg)
		}
	}
	if *calls != 3 {
		t.Fatalf("expected tool to run exactly maxAttempts(3) times, ran %d", *calls)
	}
}

func TestNVMeChecker_RecoversBeforeGivingUp(t *testing.T) {
	ctx := context.Background()
	blocked := status.Error(codes.Internal, "transient")
	// fail, fail, then supported
	c, calls := fakeChecker(3, blocked, blocked, nil)

	if err := c.check(ctx, "/dev/nvme0n1"); status.Code(err) != codes.Internal {
		t.Fatalf("attempt 1: expected Internal, got %v", err)
	}
	if err := c.check(ctx, "/dev/nvme0n1"); status.Code(err) != codes.Internal {
		t.Fatalf("attempt 2: expected Internal, got %v", err)
	}
	if err := c.check(ctx, "/dev/nvme0n1"); err != nil {
		t.Fatalf("attempt 3: expected success, got %v", err)
	}
	// Now cached supported: no more runs.
	if err := c.check(ctx, "/dev/nvme0n1"); err != nil {
		t.Fatalf("post-success: expected success, got %v", err)
	}
	if *calls != 3 {
		t.Fatalf("expected 3 runs (2 fail + 1 success), ran %d", *calls)
	}
}
