package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/go-logr/logr/testr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Test_lvm_command(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("run as root")
	}
	t.Run("simple lvm version should succeed with stream", func(t *testing.T) {
		ctx := log.IntoContext(context.Background(), testr.New(t))
		dataStream, err := callLVMStreamed(ctx, "version")
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

	t.Run("simple lvm vgcreate with non existing device should fail and show logs", func(t *testing.T) {
		// fakeDeviceName is a string that does not exist on the system (or rather is highly unlikely to exist)
		// it is used to test the error handling of the callLVMStreamed function
		fakeDeviceName := "/dev/does-not-exist"

		ctx := log.IntoContext(context.Background(), testr.New(t))
		dataStream, err := callLVMStreamed(ctx, "vgcreate", "test-vg", fakeDeviceName)
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
		if err != nil {
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
		if dataStream != nil {
			t.Fatal("data stream should be nil")
		}
	})

	t.Run("callLVM should succeed for non-json based calls", func(t *testing.T) {
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

		if !strings.Contains(messages[0], `"args"=["/sbin/lvm","version"]`) {
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
