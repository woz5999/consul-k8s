package synccatalog

import (
	"context"
	"flag"
	"fmt"
	"sync"

	"github.com/hashicorp/consul-k8s/catalog"
	"github.com/hashicorp/consul-k8s/helper/controller"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

// Command is the command for syncing the K8S and Consul service
// catalogs (one or both directions).
type Command struct {
	UI cli.Ui

	flags *flag.FlagSet
	http  *flags.HTTPFlags

	once sync.Once
	help string
}

func (c *Command) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *Command) Run(args []string) int {
	c.once.Do(c.init)
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	if len(c.flags.Args()) > 0 {
		c.UI.Error(fmt.Sprintf("Should have no non-flag arguments."))
		return 1
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/mitchellh/.kube/config")
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Setup Consul client
	consulClient, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	syncer := &catalog.ConsulSyncer{
		Client: consulClient,
		Log:    hclog.Default().Named("syncer/consul"),
	}

	go syncer.Run(context.Background())

	ctl := &controller.Controller{
		Log: hclog.Default().Named("controller/service"),
		Resource: &catalog.ServiceResource{
			Log:    hclog.Default().Named("controller/service"),
			Client: clientset,
			Syncer: syncer,
		},
	}

	ctl.Run(make(chan struct{}))
	return 0
}

func (c *Command) Synopsis() string { return synopsis }
func (c *Command) Help() string {
	c.once.Do(c.init)
	return c.help
}

const synopsis = "Sync Kubernetes services and Consul services."
const help = `
Usage: consul-k8s sync-catalog [options]

  Sync K8S pods, services, and more with the Consul service catalog.
  This enables K8S services to discover and communicate with external
  services, and allows external services to discover and communicate with
  K8S services.

`