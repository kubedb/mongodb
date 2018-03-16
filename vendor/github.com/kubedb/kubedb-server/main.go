package main

import (
	"log"
	"os"
	"runtime"

	logs "github.com/appscode/go/log/golog"
	_ "github.com/kubedb/apimachinery/client/clientset/versioned/fake"
	"github.com/kubedb/kubedb-server/pkg/cmds"
	_ "k8s.io/client-go/kubernetes/fake"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	if err := cmds.NewRootCmd(Version).Execute(); err != nil {
		log.Fatal(err)
	}
}
