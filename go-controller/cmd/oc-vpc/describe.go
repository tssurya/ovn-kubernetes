package main

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var udnGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1",
	Resource: "userdefinednetworks",
}

func newDescribeCommand(kubeconfig *string) *cli.Command {
	return &cli.Command{
		Name:      "describe",
		Usage:     "Describe a VPC and its subnets",
		ArgsUsage: "<vpc-name>",
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return fmt.Errorf("VPC name is required")
			}
			vpcName := c.Args().First()

			dyn, k8s, err := newClients(*kubeconfig)
			if err != nil {
				return err
			}

			vpc, err := dyn.Resource(vpcGVR).Get(c.Context, vpcName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("getting VPC %q: %w", vpcName, err)
			}

			fmt.Printf("Name:    %s\n", vpc.GetName())
			fmt.Printf("Created: %s\n", vpc.GetCreationTimestamp().Format("2006-01-02 15:04:05 MST"))

			printConditions(vpc)

			subnets, _, _ := unstructuredNestedSlice(vpc.Object, "spec", "subnets")
			if len(subnets) == 0 {
				fmt.Println("\nSubnets: (none)")
				return nil
			}

			fmt.Printf("\nSubnets (%d):\n", len(subnets))
			w := tabwriter.NewWriter(c.App.Writer, 0, 4, 2, ' ', 0)
			fmt.Fprintf(w, "  NAME\tTYPE\tCIDRS\tNAMESPACE\tUDN STATUS\n")

			for _, s := range subnets {
				m, ok := s.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := m["name"].(string)
				subType, _ := m["type"].(string)
				if subType == "" {
					subType = "Private"
				}
				cidrList := extractCIDRs(m)
				nsName := fmt.Sprintf("%s-%s", vpcName, name)

				nsStatus := "NotFound"
				_, err := k8s.CoreV1().Namespaces().Get(c.Context, nsName, metav1.GetOptions{})
				if err == nil {
					nsStatus = "Active"
				}

				udnStatus := "-"
				udn, err := dyn.Resource(udnGVR).Namespace(nsName).Get(c.Context, nsName, metav1.GetOptions{})
				if err == nil {
					udnStatus = extractUDNStatus(udn)
				}

				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", name, subType, cidrList, nsStatus, udnStatus)
			}
			w.Flush()

			return nil
		},
	}
}

func printConditions(vpc *unstructured.Unstructured) {
	conditions, found, _ := unstructuredNestedSlice(vpc.Object, "status", "conditions")
	if !found || len(conditions) == 0 {
		fmt.Println("Status:  No conditions")
		return
	}
	for _, cond := range conditions {
		m, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := m["type"].(string)
		status, _ := m["status"].(string)
		reason, _ := m["reason"].(string)
		message, _ := m["message"].(string)
		fmt.Printf("Status:  %s=%s (%s) %s\n", condType, status, reason, message)
	}
}

func extractCIDRs(subnet map[string]interface{}) string {
	raw, ok := subnet["cidrs"]
	if !ok {
		return "-"
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return "-"
	}
	parts := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func extractUDNStatus(udn *unstructured.Unstructured) string {
	conditions, found, _ := unstructuredNestedSlice(udn.Object, "status", "conditions")
	if !found || len(conditions) == 0 {
		return "Pending"
	}
	for _, cond := range conditions {
		m, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		if m["type"] == "NetworkReady" {
			if m["status"] == "True" {
				return "Ready"
			}
			reason, _ := m["reason"].(string)
			return fmt.Sprintf("NotReady (%s)", reason)
		}
	}
	return "Unknown"
}
