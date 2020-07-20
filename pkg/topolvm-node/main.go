package main

import (
	"github.com/topolvm/topolvm/pkg/topolvm-node/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	cmd.Execute()
}
