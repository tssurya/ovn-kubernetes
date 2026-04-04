package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var vpcGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1beta1",
	Resource: "vpcs",
}

func buildRestConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	return loader.ClientConfig()
}

func newClients(kubeconfig string) (dynamic.Interface, kubernetes.Interface, error) {
	cfg, err := buildRestConfig(kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("building kubeconfig: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("creating kubernetes client: %w", err)
	}
	return dyn, k8s, nil
}

func main() {
	var kubeconfig string

	app := &cli.App{
		Name:  "oc-vpc",
		Usage: "Manage OVN-Kubernetes VPC resources",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "kubeconfig",
				Usage:       "path to kubeconfig file",
				EnvVars:     []string{"KUBECONFIG"},
				Destination: &kubeconfig,
			},
		},
		Commands: []*cli.Command{
			newCreateCommand(&kubeconfig),
			newAddSubnetCommand(&kubeconfig),
			newRemoveSubnetCommand(&kubeconfig),
			newDescribeCommand(&kubeconfig),
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := app.RunContext(ctx, os.Args); err != nil {
		klog.Exit(err)
	}
}
