package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	"github.com/go-logr/logr/testr"
	"github.com/topolvm/topolvm/internal/lvmd/testutils"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestCallLVMStreamed(t *testing.T) {
	testutils.RequireRoot(t)

	t.Run("command output should be read from the returned stream", func(t *testing.T) {
		ctx := log.IntoContext(context.Background(), testr.New(t))
		dataStream, err := callLVMStreamed(ctx, verbosityLVMStateNoUpdate, "version")
		if err != nil {
			t.Fatal(err, "version should succeed")
		}

		data, err := io.ReadAll(dataStream)
		if err != nil {
			t.Fatal(err, "data should be readable from io stream")
		}
		if err := dataStream.Close(); err != nil {
			t.Fatal(err, "data stream should close without problems")
		}
		if !strings.Contains(string(data), "LVM version") {
			t.Fatal("LVM version not found in output")
		}
	})

	t.Run("The Close error should be LVMError and the error message is accessible from it when lvm command fails", func(t *testing.T) {
		// fakeDeviceName is a string that does not exist on the system (or rather is highly unlikely to exist)
		// it is used to test the error handling of the callLVMStreamed function
		fakeDeviceName := "/dev/does-not-exist"

		ctx := log.IntoContext(context.Background(), testr.New(t))
		dataStream, err := callLVMStreamed(ctx, verbosityLVMStateUpdate, "vgcreate", "test-vg", fakeDeviceName)
		if err != nil {
			t.Fatal(err, "vgcreate should not fail instantly as read didn't finish")
		}
		data, err := io.ReadAll(dataStream)
		if err != nil {
			t.Fatal(err, "data should be readable from io stream")
		}
		if len(data) != 0 {
			t.Fatal("data should be empty as the command should fail")
		}
		err = dataStream.Close()
		if err == nil {
			t.Fatal(err, "data stream should fail during close")
		}

		lvmErr, ok := AsLVMError(err)
		if !ok {
			t.Fatal("error should be a LVM error")
		}
		if lvmErr == nil {
			t.Fatal("error should not be nil")
		}
		if lvmErr.ExitCode() != 5 {
			t.Fatalf("exit code should be 5, got %d", lvmErr.ExitCode())
		}
		if !strings.Contains(lvmErr.Error(), "exit status 5") {
			t.Fatal("exit status 5 not contained in error")
		}
		if !strings.Contains(lvmErr.Error(), fmt.Sprintf("No device found for %s", fakeDeviceName)) {
			t.Fatal("No device found message not contained in error")
		}
	})
}

func TestCallLVM(t *testing.T) {
	testutils.RequireRoot(t)

	t.Run("The LVMNotFound error should be happened if a given VG does not exists", func(t *testing.T) {
		var messages []string
		funcLog := funcr.New(func(_, args string) {
			messages = append(messages, args)
		}, funcr.Options{
			Verbosity: 9,
		})

		ctx := log.IntoContext(context.Background(), funcLog)

		err := callLVM(ctx, "vgs", "non-existing-vg")
		if err == nil {
			t.Fatal(err, "vg should not exist")
		}

		if !IsLVMNotFound(err) {
			t.Fatal("error should be not found")
		}

		if len(messages) != 1 || !strings.Contains(messages[0], "invoking command") {
			t.Fatal("there should be nothing in stdout except the invoking command log")
		}
	})

	t.Run("The returned error should not be LVMNotFound if a generic error happens", func(t *testing.T) {
		ctx := log.IntoContext(context.Background(), testr.New(t))
		err := callLVM(ctx, "foobar")
		if err == nil {
			t.Fatal(err, "command should not be recognized")
		}

		if IsLVMNotFound(err) {
			t.Fatal("error should not be not found")
		}
	})

	t.Run("command output should be logged", func(t *testing.T) {
		var messages []string
		funcLog := funcr.New(func(_, args string) {
			messages = append(messages, args)
		}, funcr.Options{
			Verbosity: 9,
		})

		ctx := log.IntoContext(context.Background(), funcLog)
		err := callLVM(ctx, "version")
		if err != nil {
			t.Fatal(err, "version should succeed")
		}

		if len(messages) == 0 {
			t.Fatal("no messages logged")
		}

		match, _ := regexp.MatchString(`"args"=\[.* "/sbin/lvm" "version"\]`, messages[0])
		if !match {
			t.Fatal("command log was not found")
		}

		// check if the version message was logged
		stdoutExistsInLogs := false
		for _, m := range messages[1:] {
			if strings.Contains(m, "LVM version") {
				stdoutExistsInLogs = true
				break
			}
		}
		if !stdoutExistsInLogs {
			t.Fatalf("version from stdout was not logged, existing logs: %v", messages)
		}
	})
}

func TestCommandReadCloserClose(t *testing.T) {
	t.Run("Close returns even when the caller stops reading early", func(t *testing.T) {
		// Regression for #1161.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx = log.IntoContext(ctx, testr.New(t))
		cmd := exec.CommandContext(ctx, "dd", "if=/dev/zero", "bs=1024", "count=256", "status=none")

		rc, err := runCommand(ctx, verbosityLVMStateNoUpdate, cmd)
		if err != nil {
			t.Fatalf("runCommand: %v", err)
		}

		buf := make([]byte, 4)
		if _, err := io.ReadFull(rc, buf); err != nil {
			t.Fatalf("read prefix: %v", err)
		}

		if err := rc.Close(); err != nil {
			t.Fatalf("close returned error: %v", err)
		}
	})
}

// withLVMCommandPrefix runs LVM commands as prefix followed by their
// arguments until the test ends.
func withLVMCommandPrefix(t *testing.T, prefix ...string) {
	t.Helper()
	orig := lvmCommandPrefix
	SetLVMCommandPrefix(prefix)
	t.Cleanup(func() { SetLVMCommandPrefix(orig) })
}

// callLVMUntilCancelled runs an LVM command with a short deadline and fails
// the test if it does not return soon after.
func callLVMUntilCancelled(t *testing.T, args ...string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	ctx = log.IntoContext(ctx, testr.New(t))

	done := make(chan error, 1)
	go func() { done <- callLVM(ctx, args...) }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("%v still blocked 10s after its context expired", args)
		return nil
	}
}

func TestCallLVMCancelStopsReadCommand(t *testing.T) {
	// The real prefix is "nsenter -p ... /sbin/lvm", and nsenter forks before
	// it execs lvm. Stand in for it with a shell that forks a blocked child.
	withLVMCommandPrefix(t, "sh", "-c", "sleep 60; exit $?", "sh")

	for _, command := range []string{"lvs", "vgs", "fullreport"} {
		t.Run(command, func(t *testing.T) {
			err := callLVMUntilCancelled(t, command)
			if err == nil {
				t.Fatal("expected an error after the context deadline")
			}
			if _, ok := AsLVMError(err); !ok {
				t.Errorf("expected an LVM error, got %v", err)
			}
		})
	}
}

func TestCallLVMCancelReturnsWhenOutputStaysOpen(t *testing.T) {
	// A process outside the command's process group keeps stdout open after
	// the group was killed, as a process stuck in the kernel would.
	pidFile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("TOPOLVM_TEST_HELPER", "hold-output")
	t.Setenv("TOPOLVM_TEST_PID_FILE", pidFile)
	withLVMCommandPrefix(t, os.Args[0], "-test.run=^TestHelperProcess$", "--")
	origDelay := readCommandWaitDelay
	readCommandWaitDelay = 500 * time.Millisecond
	t.Cleanup(func() {
		readCommandWaitDelay = origDelay
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})

	if err := callLVMUntilCancelled(t, "vgs"); err == nil {
		t.Fatal("expected an error after the context deadline")
	}
}

// TestHelperProcess is not a test: TestCallLVMCancelReturnsWhenOutputStaysOpen
// runs the test binary as a fake lvm.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("TOPOLVM_TEST_HELPER") {
	case "hold-output":
		holder := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
		holder.Env = append(os.Environ(), "TOPOLVM_TEST_HELPER=sleep")
		holder.Stdout = os.Stdout
		holder.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := holder.Start(); err != nil {
			os.Exit(2)
		}
		_ = os.WriteFile(os.Getenv("TOPOLVM_TEST_PID_FILE"), []byte(strconv.Itoa(holder.Process.Pid)), 0o644)
		time.Sleep(time.Minute)
	case "sleep":
		time.Sleep(30 * time.Second)
	default:
		return
	}
	os.Exit(0)
}

func TestCallLVMCancelDoesNotStopWriteCommand(t *testing.T) {
	// A command that changes LVM state runs to the end even if its context
	// ends meanwhile, and its result is reported.
	finished := filepath.Join(t.TempDir(), "finished")
	withLVMCommandPrefix(t, "sh", "-c", `sleep 1; touch "$1"`, "sh", finished)

	if err := callLVMUntilCancelled(t, "lvcreate"); err != nil {
		t.Fatalf("expected the command to succeed, got %v", err)
	}
	if _, err := os.Stat(finished); err != nil {
		t.Error("expected the command to run to the end")
	}
}

func TestCallLVMDoesNotStartWriteCommandAfterCancel(t *testing.T) {
	started := filepath.Join(t.TempDir(), "started")
	withLVMCommandPrefix(t, "sh", "-c", `touch "$1"`, "sh", started)

	ctx, cancel := context.WithCancel(log.IntoContext(context.Background(), testr.New(t)))
	cancel()
	if err := callLVM(ctx, "lvremove"); err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
	if _, err := os.Stat(started); !os.IsNotExist(err) {
		t.Error("expected the command not to start")
	}
}

func TestWarnWhileRunning(t *testing.T) {
	orig := lvmCommandWarningThreshold
	SetLVMCommandWarningThreshold(100 * time.Millisecond)
	t.Cleanup(func() { SetLVMCommandWarningThreshold(orig) })
	withLVMCommandPrefix(t, "sh", "-c", "sleep 0.5", "sh")

	// callWithWarnings runs an LVM command and returns the warnings it logged.
	callWithWarnings := func(t *testing.T, args ...string) []string {
		var mu sync.Mutex
		var warnings []string
		logger := funcr.New(func(_, args string) {
			if strings.Contains(args, `"msg"="LVM command is still running"`) {
				mu.Lock()
				defer mu.Unlock()
				warnings = append(warnings, args)
			}
		}, funcr.Options{})
		if err := callLVM(log.IntoContext(context.Background(), logger), args...); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(warnings)
	}

	t.Run("a slow command that changes LVM state is reported", func(t *testing.T) {
		warnings := callWithWarnings(t, "lvcreate")
		if len(warnings) < 2 {
			t.Fatalf("expected the warning to be repeated, got %q", warnings)
		}
		for _, key := range []string{`"lvcreate"`, `"pid"=`, `"wchan"=`, `"elapsed"=`} {
			if !strings.Contains(warnings[0], key) {
				t.Errorf("expected %s in the warning %q", key, warnings[0])
			}
		}
	})

	t.Run("a slow read command is not reported", func(t *testing.T) {
		if warnings := callWithWarnings(t, "lvs"); len(warnings) != 0 {
			t.Errorf("expected no warning, got %q", warnings)
		}
	})
}

func TestLVMProcess(t *testing.T) {
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("needs /proc")
	}
	// Like "nsenter -p", the shell forks the process that does the work.
	cmd := exec.Command("sh", "-c", "sleep 30; exit $?")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	var pid int
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if pid = lvmProcess(cmd.Process.Pid); pid != cmd.Process.Pid {
			break
		}
	}
	comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(comm)); got != "sleep" {
		t.Errorf("expected the forked sleep, got %q (pid %d)", got, pid)
	}
	if got := wchan(pid); got == "unknown" {
		t.Error("expected the wchan of the forked sleep")
	}
}
