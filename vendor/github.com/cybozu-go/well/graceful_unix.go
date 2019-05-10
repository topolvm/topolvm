// +build !windows

package well

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cybozu-go/log"
)

const (
	listenEnv = "CYBOZU_LISTEN_FDS"

	restartWait = 10 * time.Millisecond
)

func isMaster() bool {
	return len(os.Getenv(listenEnv)) == 0
}

type fileFunc interface {
	File() (f *os.File, err error)
}

func listenerFiles(listeners []net.Listener) ([]*os.File, error) {
	files := make([]*os.File, 0, len(listeners))
	for _, l := range listeners {
		fd, ok := l.(fileFunc)
		if !ok {
			return nil, errors.New("no File() method for " + l.Addr().String())
		}
		f, err := fd.File()
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func restoreListeners(envvar string) ([]net.Listener, error) {
	nfds, err := strconv.Atoi(os.Getenv(envvar))
	defer os.Unsetenv(envvar)

	if err != nil {
		return nil, err
	}
	if nfds == 0 {
		return nil, nil
	}

	log.Debug("well: restored listeners", map[string]interface{}{
		"nfds": nfds,
	})

	ls := make([]net.Listener, 0, nfds)
	for i := 0; i < nfds; i++ {
		fd := 3 + i
		f := os.NewFile(uintptr(fd), "FD"+strconv.Itoa(fd))
		l, err := net.FileListener(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	}
	return ls, nil
}

// SystemdListeners returns listeners from systemd socket activation.
func SystemdListeners() ([]net.Listener, error) {
	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil {
		return nil, err
	}
	if pid != os.Getpid() {
		return nil, nil
	}
	return restoreListeners("LISTEN_FDS")
}

// Run runs the graceful restarting server.
//
// If this is the master process, Run starts a child process,
// and installs SIGHUP handler to restarts the child process.
//
// If this is a child process, Run simply calls g.Serve.
//
// Run returns immediately in the master process, and never
// returns in the child process.
func (g *Graceful) Run() {
	if isMaster() {
		env := g.Env
		if env == nil {
			env = defaultEnv
		}
		env.Go(g.runMaster)
		return
	}

	lns, err := restoreListeners(listenEnv)
	if err != nil {
		log.ErrorExit(err)
	}
	log.DefaultLogger().SetDefaults(map[string]interface{}{
		"pid": os.Getpid(),
	})
	log.Info("well: new child", nil)
	g.Serve(lns)

	// child process should not return.
	os.Exit(0)
	return
}

// runMaster is the main function of the master process.
func (g *Graceful) runMaster(ctx context.Context) error {
	logger := log.DefaultLogger()

	// prepare listener files
	listeners, err := g.Listen()
	if err != nil {
		return err
	}
	files, err := listenerFiles(listeners)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no listener")
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
		// we cannot close listeners no sooner than this point
		// because net.UnixListener removes the socket file on Close.
		for _, l := range listeners {
			l.Close()
		}
	}()

	sighup := make(chan os.Signal, 2)
	signal.Notify(sighup, syscall.SIGHUP)

RESTART:
	child := g.makeChild(files)
	clog, err := child.StderrPipe()
	if err != nil {
		return err
	}
	copyDone := make(chan struct{})
	// clog will be closed on child.Wait().
	go copyLog(logger, clog, copyDone)

	done := make(chan error, 1)
	err = child.Start()
	if err != nil {
		return err
	}
	go func() {
		<-copyDone
		done <- child.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-sighup:
		child.Process.Signal(syscall.SIGTERM)
		log.Warn("well: got sighup", nil)
		time.Sleep(restartWait)
		goto RESTART
	case <-ctx.Done():
		child.Process.Signal(syscall.SIGTERM)
		if g.ExitTimeout == 0 {
			<-done
			return nil
		}
		select {
		case <-done:
			return nil
		case <-time.After(g.ExitTimeout):
			logger.Warn("well: timeout child exit", nil)
			return nil
		}
	}
}

func (g *Graceful) makeChild(files []*os.File) *exec.Cmd {
	child := exec.Command(os.Args[0], os.Args[1:]...)
	child.Env = []string{listenEnv + "=" + strconv.Itoa(len(files))}
	child.ExtraFiles = files
	return child
}

func copyLog(logger *log.Logger, r io.Reader, done chan<- struct{}) {
	defer func() {
		close(done)
	}()

	var unwritten []byte
	buf := make([]byte, 1<<20)

	for {
		n, err := r.Read(buf)
		if err != nil {
			if len(unwritten) == 0 {
				if n > 0 {
					logger.WriteThrough(buf[0:n])
				}
				return
			}
			unwritten = append(unwritten, buf[0:n]...)
			logger.WriteThrough(unwritten)
			return
		}
		if n == 0 {
			continue
		}
		if buf[n-1] != '\n' {
			unwritten = append(unwritten, buf[0:n]...)
			continue
		}
		if len(unwritten) == 0 {
			err = logger.WriteThrough(buf[0:n])
			if err != nil {
				return
			}
			continue
		}
		unwritten = append(unwritten, buf[0:n]...)
		err = logger.WriteThrough(unwritten)
		if err != nil {
			return
		}
		unwritten = unwritten[:0]
	}
}
