package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func newAddSubnetCommand(kubeconfig *string) *cli.Command {
	var (
		cidrs          cli.StringSlice
		subnetType     string
		availZoneNodes string
	)

	return &cli.Command{
		Name:      "add-subnet",
		Usage:     "Add a subnet to an existing VPC",
		ArgsUsage: "<vpc-name> <subnet-name>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:        "cidr",
				Usage:       "subnet CIDR (can be specified multiple times for dual-stack)",
				Required:    true,
				Destination: &cidrs,
			},
			&cli.StringFlag{
				Name:        "type",
				Usage:       "subnet type: Public, Private, Isolated, VPNOnly (default: Private)",
				Value:       "Private",
				Destination: &subnetType,
			},
			&cli.StringFlag{
				Name:        "availability-zone-nodes",
				Usage:       "comma-separated key=value node selector for availability zone",
				Destination: &availZoneNodes,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 2 {
				return fmt.Errorf("usage: oc vpc add-subnet <vpc-name> <subnet-name> --cidr <cidr>")
			}
			vpcName := c.Args().Get(0)
			subnetName := c.Args().Get(1)

			dyn, _, err := newClients(*kubeconfig)
			if err != nil {
				return err
			}

			existing, err := dyn.Resource(vpcGVR).Get(c.Context, vpcName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("getting VPC %q: %w", vpcName, err)
			}

			subnets, _, _ := unstructuredNestedSlice(existing.Object, "spec", "subnets")

			for _, s := range subnets {
				if m, ok := s.(map[string]interface{}); ok {
					if m["name"] == subnetName {
						return fmt.Errorf("subnet %q already exists in VPC %q", subnetName, vpcName)
					}
				}
			}

			newSubnet := map[string]interface{}{
				"name":  subnetName,
				"cidrs": toInterfaceSlice(cidrs.Value()),
				"type":  subnetType,
			}

			if availZoneNodes != "" {
				nodeSelector := parseKeyValuePairs(availZoneNodes)
				newSubnet["availabilityZone"] = map[string]interface{}{
					"clusterSelector": map[string]interface{}{},
					"nodeSelector":    nodeSelector,
				}
			}

			subnets = append(subnets, newSubnet)

			patch := map[string]interface{}{
				"spec": map[string]interface{}{
					"subnets": subnets,
				},
			}
			patchBytes, err := json.Marshal(patch)
			if err != nil {
				return fmt.Errorf("marshaling patch: %w", err)
			}

			_, err = dyn.Resource(vpcGVR).Patch(c.Context, vpcName, k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("patching VPC %q: %w", vpcName, err)
			}

			fmt.Printf("subnet %q added to vpc/%s\n", subnetName, vpcName)
			return nil
		},
	}
}

func newRemoveSubnetCommand(kubeconfig *string) *cli.Command {
	return &cli.Command{
		Name:      "remove-subnet",
		Usage:     "Remove a subnet from an existing VPC",
		ArgsUsage: "<vpc-name> <subnet-name>",
		Action: func(c *cli.Context) error {
			if c.NArg() < 2 {
				return fmt.Errorf("usage: oc vpc remove-subnet <vpc-name> <subnet-name>")
			}
			vpcName := c.Args().Get(0)
			subnetName := c.Args().Get(1)

			dyn, _, err := newClients(*kubeconfig)
			if err != nil {
				return err
			}

			existing, err := dyn.Resource(vpcGVR).Get(c.Context, vpcName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("getting VPC %q: %w", vpcName, err)
			}

			subnets, _, _ := unstructuredNestedSlice(existing.Object, "spec", "subnets")

			filtered := make([]interface{}, 0, len(subnets))
			found := false
			for _, s := range subnets {
				if m, ok := s.(map[string]interface{}); ok {
					if m["name"] == subnetName {
						found = true
						continue
					}
				}
				filtered = append(filtered, s)
			}

			if !found {
				return fmt.Errorf("subnet %q not found in VPC %q", subnetName, vpcName)
			}

			patch := map[string]interface{}{
				"spec": map[string]interface{}{
					"subnets": filtered,
				},
			}
			patchBytes, err := json.Marshal(patch)
			if err != nil {
				return fmt.Errorf("marshaling patch: %w", err)
			}

			_, err = dyn.Resource(vpcGVR).Patch(c.Context, vpcName, k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("patching VPC %q: %w", vpcName, err)
			}

			fmt.Printf("subnet %q removed from vpc/%s\n", subnetName, vpcName)
			return nil
		},
	}
}

func unstructuredNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	var current interface{} = obj
	for _, f := range fields {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		current, ok = m[f]
		if !ok {
			return nil, false, nil
		}
	}
	s, ok := current.([]interface{})
	return s, ok, nil
}

func toInterfaceSlice(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func parseKeyValuePairs(s string) map[string]interface{} {
	result := make(map[string]interface{})
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}
