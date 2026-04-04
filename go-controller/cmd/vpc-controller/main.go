package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	utilpointer "k8s.io/utils/ptr"

	vpcv1beta1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/vpc/v1beta1"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/vpc"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(vpcv1beta1.AddToScheme(scheme))
}

type config struct {
	kubeconfig     string
	apiServer      string
	logLevel       int
	metricsAddress string
	leaseNamespace string
}

var cfg config

func main() {
	app := cli.NewApp()
	app.Name = "vpc-controller"
	app.Usage = "Reconcile VPC custom resources into OVN-Kubernetes primitives (Namespaces, UDNs, RouteAdvertisements)"

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "kubeconfig",
			Usage:       "kubeconfig path",
			Destination: &cfg.kubeconfig,
		},
		&cli.StringFlag{
			Name:        "k8s-apiserver",
			Usage:       "URL of the Kubernetes API server",
			Destination: &cfg.apiServer,
		},
		&cli.IntFlag{
			Name:        "loglevel",
			Usage:       "klog verbosity (default: 4)",
			Value:       4,
			Destination: &cfg.logLevel,
		},
		&cli.StringFlag{
			Name:        "metrics-address",
			Usage:       "bind address for the metrics endpoint (0 = disabled)",
			Value:       ":8080",
			Destination: &cfg.metricsAddress,
		},
		&cli.StringFlag{
			Name:        "lease-namespace",
			Usage:       "namespace for leader election lease",
			Value:       "ovn-kubernetes",
			Destination: &cfg.leaseNamespace,
		},
	}

	app.Action = func(c *cli.Context) error {
		ctrl.SetLogger(klog.NewKlogr())

		var level klog.Level
		if err := level.Set(strconv.Itoa(cfg.logLevel)); err != nil {
			klog.Errorf("Failed to set klog log level: %v", err)
			os.Exit(1)
		}

		restCfg, err := clientcmd.BuildConfigFromFlags(cfg.apiServer, cfg.kubeconfig)
		if err != nil {
			return err
		}

		mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
			Scheme: scheme,
			Metrics: server.Options{
				BindAddress: cfg.metricsAddress,
			},
			LeaderElection:                true,
			LeaderElectionID:              "vpc-controller",
			LeaderElectionNamespace:       cfg.leaseNamespace,
			LeaseDuration:                 utilpointer.To(60 * time.Second),
			RenewDeadline:                 utilpointer.To(30 * time.Second),
			RetryPeriod:                   utilpointer.To(20 * time.Second),
			LeaderElectionReleaseOnCancel: true,
		})
		if err != nil {
			klog.Errorf("Failed to create manager: %v", err)
			return err
		}

		reconciler := vpc.NewReconciler(mgr.GetClient())
		if err := reconciler.SetupWithManager(mgr); err != nil {
			klog.Errorf("Failed to setup VPC reconciler: %v", err)
			return err
		}

		klog.Info("Starting VPC controller")
		return mgr.Start(c.Context)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := app.RunContext(ctx, os.Args); err != nil {
		klog.Exit(err)
	}
}
