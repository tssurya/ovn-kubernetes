package clustermanager

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	hotypes "github.com/ovn-org/ovn-kubernetes/go-controller/hybrid-overlay/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	egressfirewallfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressfirewall/v1/apis/clientset/versioned/fake"
	egressipfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressip/v1/apis/clientset/versioned/fake"
	egressqosfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressqos/v1/apis/clientset/versioned/fake"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/kube"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

var _ = ginkgo.Describe("Cluster Manager operations", func() {
	var (
		app      *cli.App
		f        *factory.WatchFactory
		stopChan chan struct{}
		wg       *sync.WaitGroup
	)

	const (
		clusterIPNet             string = "10.1.0.0"
		clusterCIDR              string = clusterIPNet + "/16"
		hybridOverlayClusterCIDR string = "11.1.0.0/16/24"
	)

	ginkgo.BeforeEach(func() {
		// Restore global default values before each testcase
		gomega.Expect(config.PrepareTestConfig()).To(gomega.Succeed())

		app = cli.NewApp()
		app.Name = "test"
		app.Flags = config.Flags
		stopChan = make(chan struct{})
		wg = &sync.WaitGroup{}
	})

	ginkgo.AfterEach(func() {
		close(stopChan)
		f.Shutdown()
		wg.Wait()
	})

	ginkgo.It("Cluster Manager Node subnet allocations and node id allocations", func() {

		app.Action = func(ctx *cli.Context) error {
			nodes := []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node3",
					},
				}}
			kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
				Items: nodes,
			})
			egressFirewallFakeClient := &egressfirewallfake.Clientset{}
			egressIPFakeClient := &egressipfake.Clientset{}
			egressQoSFakeClient := &egressqosfake.Clientset{}
			fakeClient := &util.OVNClientset{
				KubeClient:           kubeFakeClient,
				EgressIPClient:       egressIPFakeClient,
				EgressFirewallClient: egressFirewallFakeClient,
				EgressQoSClient:      egressQoSFakeClient,
			}

			_, err := config.InitConfig(ctx, nil, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			config.Kubernetes.HostNetworkNamespace = ""

			f, err = factory.NewClusterManagerWatchFactory(fakeClient)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = f.Start()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			clusterManager := NewClusterManager(fakeClient, f, stopChan, wg,
				record.NewFakeRecorder(0))
			gomega.Expect(clusterManager).NotTo(gomega.BeNil())

			clusterManager.InitSubnetAllocatorRanges(config.Default.ClusterSubnets)

			if config.HybridOverlay.ClusterSubnets != nil {
				clusterManager.InitHybridOverlaySubnetAllocatorRanges(config.HybridOverlay.ClusterSubnets)
			}
			gomega.Expect(clusterManager.WatchNodes()).To(gomega.Succeed())

			// Check that cluster manager has set the subnet annotation for each node.
			for _, n := range nodes {
				gomega.Eventually(func() ([]*net.IPNet, error) {
					updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}

					return util.ParseNodeHostSubnetAnnotation(updatedNode, types.DefaultNetworkName)
				}, 2).Should(gomega.HaveLen(1))
			}

			for _, n := range nodes {
				gomega.Eventually(func() error {
					updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}

					nodeId, ok := updatedNode.Annotations[util.OvnNodeId]
					if !ok {
						return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
					}

					_, err = strconv.Atoi(nodeId)
					if err != nil {
						return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
					}

					_, ok = updatedNode.Annotations["k8s.ovn.org/ovn-node-transit-switch-port-ips"]
					if !ok {
						return fmt.Errorf("expected node annotation for node %s to have transit switch port ips allocated", n.Name)
					}

					transitSwitchIps, err := util.ParseNodeTransitSwitchPortAddresses(updatedNode)
					if err != nil {
						return fmt.Errorf("error parsing transit switch ip annotations for the node %s", n.Name)
					}

					if len(transitSwitchIps) < 1 {
						return fmt.Errorf("transit switch ips for node %s not allocated", n.Name)
					}
					return nil
				}).ShouldNot(gomega.HaveOccurred())
			}

			// Clear the subnet annotation of node 1 and make sure it is re-allocated by cluster manager.
			nodeAnnotator := kube.NewNodeAnnotator(&kube.Kube{kubeFakeClient, egressIPFakeClient, egressFirewallFakeClient, nil}, "node1")
			util.DeleteNodeHostSubnetAnnotation(nodeAnnotator)
			err = nodeAnnotator.Run()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func() ([]*net.IPNet, error) {
				updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
				if err != nil {
					return nil, err
				}

				return util.ParseNodeHostSubnetAnnotation(updatedNode, types.DefaultNetworkName)
			}, 2).Should(gomega.HaveLen(1))

			node1, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
			node1Id := node1.Annotations[util.OvnNodeId]

			// Clear the nodeId annotation from node 1.
			nodeAnnotations := node1.Annotations
			for k, v := range nodeAnnotations {
				nodeAnnotator.Set(k, v)
			}
			nodeAnnotator.Delete(util.OvnNodeId)
			err = nodeAnnotator.Run()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
				if err != nil {
					return err
				}

				updatedNodeId, ok := updatedNode.Annotations[util.OvnNodeId]
				if !ok {
					return fmt.Errorf("expected node annotation for node - node1 to have node id allocated")
				}

				_, err = strconv.Atoi(updatedNodeId)
				if err != nil {
					return fmt.Errorf("expected node annotation for node - node1 to be an integer value, got %s", updatedNodeId)
				}

				gomega.Expect(node1Id).To(gomega.Equal(updatedNodeId))
				return nil
			}).ShouldNot(gomega.HaveOccurred())

			// Clear the transit switch port ip annotation from node 1.
			node1, _ = fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
			nodeAnnotations = node1.Annotations
			for k, v := range nodeAnnotations {
				nodeAnnotator.Set(k, v)
			}
			node1TransitSwitchIps := node1.Annotations["k8s.ovn.org/ovn-node-transit-switch-port-ips"]
			nodeAnnotator.Delete("k8s.ovn.org/ovn-node-transit-switch-port-ips")
			err = nodeAnnotator.Run()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
				if err != nil {
					return err
				}

				updatedNode1TransitSwitchIps, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-transit-switch-port-ips"]
				if !ok {
					return fmt.Errorf("expected node annotation for node node1 to have transit switch port ips allocated")
				}

				transitSwitchIps, err := util.ParseNodeTransitSwitchPortAddresses(updatedNode)
				if err != nil {
					return fmt.Errorf("error parsing transit switch ip annotations for the node node1")
				}

				if len(transitSwitchIps) < 1 {
					return fmt.Errorf("transit switch ips for node node1 not allocated")
				}
				gomega.Expect(node1TransitSwitchIps).To(gomega.Equal(updatedNode1TransitSwitchIps))
				return nil
			}).ShouldNot(gomega.HaveOccurred())
			return nil
		}

		err := app.Run([]string{
			app.Name,
			"-cluster-subnets=" + clusterCIDR,
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})

	ginkgo.It("Cluster Manager Node subnet allocations - hybrid and linux nodes", func() {

		app.Action = func(ctx *cli.Context) error {
			nodes := []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node3",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "winnode",
						Labels: map[string]string{v1.LabelOSStable: "windows"},
					},
				}}
			kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
				Items: nodes,
			})
			egressFirewallFakeClient := &egressfirewallfake.Clientset{}
			egressIPFakeClient := &egressipfake.Clientset{}
			egressQoSFakeClient := &egressqosfake.Clientset{}
			fakeClient := &util.OVNClientset{
				KubeClient:           kubeFakeClient,
				EgressIPClient:       egressIPFakeClient,
				EgressFirewallClient: egressFirewallFakeClient,
				EgressQoSClient:      egressQoSFakeClient,
			}

			_, err := config.InitConfig(ctx, nil, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			config.Kubernetes.HostNetworkNamespace = ""

			f, err = factory.NewClusterManagerWatchFactory(fakeClient)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = f.Start()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			clusterManager := NewClusterManager(fakeClient, f, stopChan, wg,
				record.NewFakeRecorder(0))
			gomega.Expect(clusterManager).NotTo(gomega.BeNil())
			clusterManager.InitSubnetAllocatorRanges(config.Default.ClusterSubnets)

			if config.HybridOverlay.ClusterSubnets != nil {
				clusterManager.InitHybridOverlaySubnetAllocatorRanges(config.HybridOverlay.ClusterSubnets)
			}
			gomega.Expect(clusterManager.WatchNodes()).To(gomega.Succeed())

			// Check that cluster manager has set the subnet annotation for each node.
			for _, n := range nodes {
				if n.Name == "winnode" {
					continue
				}

				gomega.Eventually(func() ([]*net.IPNet, error) {
					updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}

					return util.ParseNodeHostSubnetAnnotation(updatedNode, types.DefaultNetworkName)
				}, 2).Should(gomega.HaveLen(1))
			}

			// Windows node should be allocated a subnet
			gomega.Eventually(func() (map[string]string, error) {
				updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "winnode", metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				return updatedNode.Annotations, nil
			}, 2).Should(gomega.HaveKey(hotypes.HybridOverlayNodeSubnet))

			gomega.Eventually(func() error {
				updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "winnode", metav1.GetOptions{})
				if err != nil {
					return err
				}
				_, err = util.ParseNodeHostSubnetAnnotation(updatedNode, types.DefaultNetworkName)
				return err
			}, 2).Should(gomega.MatchError("could not find \"k8s.ovn.org/node-subnets\" annotation"))

			return nil
		}

		err := app.Run([]string{
			app.Name,
			"--no-hostsubnet-nodes=kubernetes.io/os=windows",
			"-cluster-subnets=" + clusterCIDR,
			"-gateway-mode=shared",
			"-enable-hybrid-overlay",
			"-hybrid-overlay-cluster-subnets=" + hybridOverlayClusterCIDR,
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})
})
