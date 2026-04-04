package main

import (
	"fmt"

	"github.com/urfave/cli/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newCreateCommand(kubeconfig *string) *cli.Command {
	var (
		subnetName string
		cidrs      cli.StringSlice
		subnetType string
	)

	return &cli.Command{
		Name:      "create",
		Usage:     "Create a new VPC with an initial subnet",
		ArgsUsage: "<name>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "subnet",
				Usage:       "name of the initial subnet (required)",
				Required:    true,
				Destination: &subnetName,
			},
			&cli.StringSliceFlag{
				Name:        "cidr",
				Usage:       "subnet CIDR (can be specified multiple times for dual-stack)",
				Required:    true,
				Destination: &cidrs,
			},
			&cli.StringFlag{
				Name:        "type",
				Usage:       "subnet type: Public, Private, Isolated, VPNOnly",
				Value:       "Private",
				Destination: &subnetType,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return fmt.Errorf("VPC name is required")
			}
			name := c.Args().First()

			dyn, _, err := newClients(*kubeconfig)
			if err != nil {
				return err
			}

			cidrSlice := make([]interface{}, len(cidrs.Value()))
			for i, c := range cidrs.Value() {
				cidrSlice[i] = c
			}

			vpc := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "k8s.ovn.org/v1beta1",
					"kind":       "VPC",
					"metadata": map[string]interface{}{
						"name": name,
					},
					"spec": map[string]interface{}{
						"subnets": []interface{}{
							map[string]interface{}{
								"name":  subnetName,
								"cidrs": cidrSlice,
								"type":  subnetType,
							},
						},
					},
				},
			}

			result, err := dyn.Resource(vpcGVR).Create(c.Context, vpc, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating VPC %q: %w", name, err)
			}

			fmt.Printf("vpc/%s created (subnet: %s, cidrs: %v, type: %s)\n", result.GetName(), subnetName, cidrs.Value(), subnetType)
			return nil
		},
	}
}
