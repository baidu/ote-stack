package main

import (
	"fmt"
	"os"

	"k8s.io/component-base/logs"

	"github.com/baidu/ote-stack/cmd/edgecontroller/app"
)

func main() {
	command := app.NewEdgeControllerCommand()

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
