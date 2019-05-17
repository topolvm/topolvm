package microtest

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMtest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test on microk8s")
}
