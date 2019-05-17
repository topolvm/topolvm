package microtest

import (
	"os"
	"testing"

	"github.com/cybozu-go/cke/mtest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMtest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test on microk8s")
}