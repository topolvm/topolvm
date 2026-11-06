package command

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// fakeIpcs reports one device-mapper udev cookie (semid 7) and one unrelated
// semaphore (semid 8). The cookie's waiter count and last PID come from the
// files zcount and lastpid next to the script.
const fakeIpcs = `#!/bin/sh
dir=$(dirname "$0")
if [ "$2" = "-i" ]; then
	cat <<EOT

Semaphore Array semid=$3
uid=0	 gid=0	 cuid=0	 cgid=0
mode=0600, access_perms=0600
nsems = 1
otime = Mon Oct 26 21:45:31 2026
ctime = Mon Oct 26 21:45:31 2026
semnum     value      ncount     zcount     pid
0          1          0          $(cat "$dir/zcount")          $(cat "$dir/lastpid")

EOT
else
	cat <<EOT

------ Semaphore Arrays --------
key        semid      owner      perms      nsems
0x0d4df510 7          root       600        1
0x00001234 8          root       600        1

EOT
fi
`

// fakeIpcrm records its arguments and lets a fake lvm waiting for the cookie go.
const fakeIpcrm = `#!/bin/sh
dir=$(dirname "$0")
echo "$@" >> "$dir/ipcrm.log"
touch "$dir/released"
`

// setupFakeHost installs fake ipcs and ipcrm, and runs lvm as the shell script
// lvm with the fake's directory as $1. It returns that directory.
func setupFakeHost(t *testing.T, lvm, zcount string) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("finding the processes of a group needs /proc")
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"ipcs": fakeIpcs, "ipcrm": fakeIpcrm} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "zcount"), []byte(zcount), 0o644); err != nil {
		t.Fatal(err)
	}

	origPrefix, origHost, origGrace := lvmCommandPrefix, hostCommand, cookieReleaseGrace
	SetLVMCommandPrefix([]string{"sh", "-c", lvm, "sh", dir})
	hostCommand = func(name string, args ...string) []string {
		return slices.Concat([]string{filepath.Join(dir, name)}, args)
	}
	cookieReleaseGrace = 2 * time.Second
	t.Cleanup(func() {
		SetLVMCommandPrefix(origPrefix)
		hostCommand, cookieReleaseGrace = origHost, origGrace
	})
	return dir
}

// callLVMUntilCancelled runs a fake lvm with a short deadline and fails the
// test if it does not return soon after.
func callLVMUntilCancelled(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	ctx = log.IntoContext(ctx, testr.New(t))

	done := make(chan error, 1)
	go func() { done <- callLVM(ctx, "lvcreate") }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error after the context deadline")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("callLVM still blocked 10s after its context expired")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	} else if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCancelReleasesTheUdevCookieLVMWaitsFor(t *testing.T) {
	// lvm waits for its cookie, and finishes once the cookie is deleted.
	dir := setupFakeHost(t, `echo $$ > "$1/lastpid"
		while [ ! -e "$1/released" ]; do sleep 0.05; done
		touch "$1/exited"`, "1")

	callLVMUntilCancelled(t)

	if got := readFile(t, filepath.Join(dir, "ipcrm.log")); got != "-s 7\n" {
		t.Errorf("expected only the cookie lvm waits for (semid 7) to be deleted, ipcrm got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "exited")); err != nil {
		t.Error("expected lvm to finish by itself once its cookie was released, but it was killed")
	}
}

func TestCancelRemovesTheUdevCookieOfAKilledLVM(t *testing.T) {
	// lvm holds a cookie but is stuck elsewhere, so it has to be killed.
	dir := setupFakeHost(t, `echo $$ > "$1/lastpid"; sleep 60; exit $?`, "0")

	callLVMUntilCancelled(t)

	if got := readFile(t, filepath.Join(dir, "ipcrm.log")); got != "-s 7\n" {
		t.Errorf("expected the cookie left by the killed lvm (semid 7) to be deleted, ipcrm got %q", got)
	}
}

func TestCancelLeavesOtherUdevCookiesAlone(t *testing.T) {
	// Someone else (PID 1) waits for the only cookie.
	dir := setupFakeHost(t, `sleep 60; exit $?`, "1")
	if err := os.WriteFile(filepath.Join(dir, "lastpid"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}

	callLVMUntilCancelled(t)

	if got := readFile(t, filepath.Join(dir, "ipcrm.log")); got != "" {
		t.Errorf("expected no cookie to be deleted, ipcrm got %q", got)
	}
}

func TestDefaultHostCommand(t *testing.T) {
	orig := lvmCommandPrefix
	t.Cleanup(func() { SetLVMCommandPrefix(orig) })

	for _, tc := range []struct {
		prefix []string
		want   []string
	}{
		{
			prefix: []string{"/usr/bin/nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", "/sbin/lvm"},
			want:   []string{"/usr/bin/nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", "ipcs", "-s"},
		},
		{prefix: []string{"/usr/sbin/lvm"}, want: []string{"ipcs", "-s"}},
		{prefix: []string{"chroot", "/host", "--"}, want: []string{"chroot", "/host", "--", "ipcs", "-s"}},
		{prefix: nil, want: []string{"ipcs", "-s"}},
	} {
		SetLVMCommandPrefix(tc.prefix)
		if got := defaultHostCommand("ipcs", "-s"); !slices.Equal(got, tc.want) {
			t.Errorf("prefix %q: got %q, want %q", tc.prefix, got, tc.want)
		}
	}
}

func TestParseSemaphore(t *testing.T) {
	c := udevCookie{semid: "50"}
	err := c.parseSemaphore([]byte(`
Semaphore Array semid=50
uid=0	 gid=0	 cuid=0	 cgid=0
mode=0600, access_perms=0600
nsems = 1
otime = Mon Oct 26 21:45:31 2026
ctime = Mon Oct 26 21:45:31 2026
semnum     value      ncount     zcount     pid
0          1          0          1          4155351
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.waiters != 1 || c.lastPID != 4155351 {
		t.Errorf("got waiters=%d lastPID=%d, want 1 and 4155351", c.waiters, c.lastPID)
	}

	if err := (&udevCookie{semid: "50"}).parseSemaphore([]byte("garbage")); err == nil {
		t.Error("expected an error for output without a semaphore table")
	}
}
