package command

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

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
