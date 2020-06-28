// Binary ote_edgehub is a proxy handling all the request from node to k8s master.
package main

import (
	"fmt"
	"os"

	"k8s.io/component-base/logs"

	"github.com/baidu/ote-stack/cmd/edgehub/app"
)

func main() {
	command := app.NewEdgehubCommand()

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
