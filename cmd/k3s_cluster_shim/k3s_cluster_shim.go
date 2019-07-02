/*
Copyright 2019 Baidu, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Binary k3s_cluster_shim is an adapter handling message from clustercontroller for k3s cluster.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/logs"

	"github.com/baidu/ote-stack/cmd/k3s_cluster_shim/app"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	command := app.NewK3sClusterShimCommand()

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
