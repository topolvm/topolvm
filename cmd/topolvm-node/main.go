package main

import (
	"github.com/topolvm/topolvm/cmd/topolvm-node/app"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	app.Execute()
}
