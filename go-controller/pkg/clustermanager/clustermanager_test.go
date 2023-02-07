package clustermanager

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	hotypes "github.com/ovn-org/ovn-kubernetes/go-controller/hybrid-overlay/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/kube"
	ovntypes "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	"github.com/urfave/cli/v2"
)

var _ = ginkgo.Describe("Cluster Manager", func() {
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
		joinSubnetCIDR           string = "100.64.0.0/16/19"
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
		if f != nil {
			f.Shutdown()
		}
		wg.Wait()
	})

	ginkgo.Context("Node subnet allocations", func() {
		ginkgo.It("Linux nodes", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				c, cancel := context.WithCancel(ctx.Context)
				defer cancel()
				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(c)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has set the subnet annotation for each node.
				for _, n := range nodes {
					gomega.Eventually(func() ([]*net.IPNet, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return util.ParseNodeHostSubnetAnnotation(updatedNode, ovntypes.DefaultNetworkName)
					}, 2).Should(gomega.HaveLen(1))
				}

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Linux nodes - clear subnet annotations", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				c, cancel := context.WithCancel(ctx.Context)
				defer cancel()
				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(c)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has set the subnet annotation for each node.
				for _, n := range nodes {
					gomega.Eventually(func() ([]*net.IPNet, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return util.ParseNodeHostSubnetAnnotation(updatedNode, ovntypes.DefaultNetworkName)
					}, 2).Should(gomega.HaveLen(1))
				}

				// Clear the subnet annotation of nodes and make sure it is re-allocated by cluster manager.
				for _, n := range nodes {
					nodeAnnotator := kube.NewNodeAnnotator(&kube.Kube{kubeFakeClient}, n.Name)
					util.DeleteNodeHostSubnetAnnotation(nodeAnnotator)
					err = nodeAnnotator.Run()
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}

				// Check that cluster manager has reset the subnet annotation for each node.
				for _, n := range nodes {
					gomega.Eventually(func() ([]*net.IPNet, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return util.ParseNodeHostSubnetAnnotation(updatedNode, ovntypes.DefaultNetworkName)
					}, 2).Should(gomega.HaveLen(1))
				}

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Hybrid and linux nodes", func() {

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
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				c, cancel := context.WithCancel(ctx.Context)
				defer cancel()
				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(c)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

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

						return util.ParseNodeHostSubnetAnnotation(updatedNode, ovntypes.DefaultNetworkName)
					}, 2).Should(gomega.HaveLen(1))
				}

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

		ginkgo.It("Hybrid nodes - clear subnet annotations", func() {

			app.Action = func(ctx *cli.Context) error {
				nodes := []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "winnode1",
							Labels: map[string]string{v1.LabelOSStable: "windows"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "winnode2",
							Labels: map[string]string{v1.LabelOSStable: "windows"},
						},
					},
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				c, cancel := context.WithCancel(ctx.Context)
				defer cancel()
				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(c)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has set the subnet annotation for each node.
				for _, n := range nodes {
					gomega.Eventually(func() (map[string]string, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						return updatedNode.Annotations, nil
					}, 2).Should(gomega.HaveKey(hotypes.HybridOverlayNodeSubnet))

					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}
						_, err = util.ParseNodeHostSubnetAnnotation(updatedNode, ovntypes.DefaultNetworkName)
						return err
					}, 2).Should(gomega.MatchError("could not find \"k8s.ovn.org/node-subnets\" annotation"))
				}

				// Clear the subnet annotation of nodes and make sure it is re-allocated by cluster manager.
				for _, n := range nodes {
					nodeAnnotator := kube.NewNodeAnnotator(&kube.Kube{kubeFakeClient}, n.Name)

					nodeAnnotations := n.Annotations
					for k, v := range nodeAnnotations {
						nodeAnnotator.Set(k, v)
					}
					nodeAnnotator.Delete(hotypes.HybridOverlayNodeSubnet)
					err = nodeAnnotator.Run()
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}

				for _, n := range nodes {
					gomega.Eventually(func() (map[string]string, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}
						return updatedNode.Annotations, nil
					}, 2).Should(gomega.HaveKey(hotypes.HybridOverlayNodeSubnet))

					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}
						_, err = util.ParseNodeHostSubnetAnnotation(updatedNode, ovntypes.DefaultNetworkName)
						return err
					}, 2).Should(gomega.MatchError("could not find \"k8s.ovn.org/node-subnets\" annotation"))
				}
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

	ginkgo.Context("Node Id allocations", func() {
		ginkgo.It("check for node id allocations", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has allocated id for each node
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						nodeId, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-id"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
						}

						_, err = strconv.Atoi(nodeId)
						if err != nil {
							return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
						}
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("clear the node ids and check", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				nodeIds := make(map[string]string)
				// Check that cluster manager has allocated id for each node before clearing
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						nodeId, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-id"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
						}

						_, err = strconv.Atoi(nodeId)
						if err != nil {
							return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
						}

						nodeIds[n.Name] = nodeId
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				// Clear the node id annotation of nodes and make sure it is reset by cluster manager
				// with the same ids.
				for _, n := range nodes {
					nodeAnnotator := kube.NewNodeAnnotator(&kube.Kube{kubeFakeClient}, n.Name)

					nodeAnnotations := n.Annotations
					for k, v := range nodeAnnotations {
						nodeAnnotator.Set(k, v)
					}
					nodeAnnotator.Delete("k8s.ovn.org/ovn-node-id")
					err = nodeAnnotator.Run()
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}

				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						nodeId, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-id"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
						}

						_, err = strconv.Atoi(nodeId)
						if err != nil {
							return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
						}

						gomega.Expect(nodeId).To(gomega.Equal(nodeIds[n.Name]))
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Stop and start a new cluster manager and verify the node ids", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				wg1 := &sync.WaitGroup{}
				clusterManager, err := NewClusterManager(fakeClient, f, "cm1", wg1, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				// Check that cluster manager has allocated id for each node before clearing
				nodeIds := make(map[string]string)
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						nodeId, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-id"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
						}

						_, err = strconv.Atoi(nodeId)
						if err != nil {
							return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
						}

						nodeIds[n.Name] = nodeId
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				updatedNodes := []v1.Node{}
				for _, n := range nodes {
					updatedNode, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
					updatedNodes = append(updatedNodes, *updatedNode)
				}
				// stop the cluster manager and start a new instance and make sure the node ids are same.
				clusterManager.Stop()
				wg1.Wait()

				// Close the watch factory and create a new one
				f.Shutdown()
				kubeFakeClient = fake.NewSimpleClientset(&v1.NodeList{
					Items: updatedNodes,
				})
				fakeClient = &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}
				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				cm2, err := NewClusterManager(fakeClient, f, "cm2", wg, nil)
				gomega.Expect(cm2).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = cm2.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer cm2.Stop()

				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						nodeId, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-id"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
						}

						_, err = strconv.Atoi(nodeId)
						if err != nil {
							return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
						}

						gomega.Expect(nodeId).To(gomega.Equal(nodeIds[n.Name]))
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Stop cluster manager, set duplicate id, restart and verify the node ids", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				wg1 := &sync.WaitGroup{}
				clusterManager, err := NewClusterManager(fakeClient, f, "cm1", wg1, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				nodeIds := make(map[string]string)
				// Check that cluster manager has allocated id for each node before clearing
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						nodeId, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-id"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
						}

						_, err = strconv.Atoi(nodeId)
						if err != nil {
							return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
						}

						nodeIds[n.Name] = nodeId
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				// stop the cluster manager.
				clusterManager.Stop()
				wg1.Wait()

				updatedNodes := []v1.Node{}
				node2, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node2", metav1.GetOptions{})
				for _, n := range nodes {
					updatedNode, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
					if updatedNode.Name == "node3" {
						// Make the id of node3 duplicate.
						updatedNode.Annotations["k8s.ovn.org/ovn-node-id"] = node2.Annotations["k8s.ovn.org/ovn-node-id"]
					}
					updatedNodes = append(updatedNodes, *updatedNode)
				}

				// Close the watch factory and create a new one
				f.Shutdown()
				kubeFakeClient = fake.NewSimpleClientset(&v1.NodeList{
					Items: updatedNodes,
				})
				fakeClient = &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}
				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				// Start a new cluster manager
				cm2, err := NewClusterManager(fakeClient, f, "cm2", wg, nil)
				gomega.Expect(cm2).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = cm2.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer cm2.Stop()

				// Get the node ids of node2 and node3 and make sure that they are not equal
				gomega.Eventually(func() error {
					n2, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node2", metav1.GetOptions{})
					n3, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node3", metav1.GetOptions{})
					n2Id := n2.Annotations["k8s.ovn.org/ovn-node-id"]
					n3Id := n3.Annotations["k8s.ovn.org/ovn-node-id"]
					if n2Id == n3Id {
						return fmt.Errorf("expected node annotation for node2 and node3 to be not equal, but they are : node id %s", n2Id)
					}
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
	})

	ginkgo.Context("Zone join switch subnet allocations", func() {
		ginkgo.It("verify the node annotations", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has set the zone join subnet annotation for each node.
				for _, n := range nodes {
					gomega.Eventually(func() ([]*net.IPNet, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return util.ParseZoneJoinSubnetsAnnotation(updatedNode, ovntypes.DefaultNetworkName)
					}, 2).Should(gomega.HaveLen(1))
				}

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("change the node zone and check the zone subnets", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has set the zone join subnet annotation for each node.
				// All the zones belong to the global zone.
				for _, n := range nodes {
					gomega.Eventually(func() ([]*net.IPNet, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return util.ParseZoneJoinSubnetsAnnotation(updatedNode, ovntypes.DefaultNetworkName)
					}, 2).Should(gomega.HaveLen(1))
				}

				// Check that cluster manager has allocated id for each node and subnet before changing the node zones.
				// This would avoid any race conditions while updating the node annotations as the fake client
				// doesn't handle the retry conflicts.
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						nodeId, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-id"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have node id allocated", n.Name)
						}

						_, err = strconv.Atoi(nodeId)
						if err != nil {
							return fmt.Errorf("expected node annotation for node %s to be an integer value, got %s", n.Name, nodeId)
						}

						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}
				for _, n := range nodes {
					gomega.Eventually(func() ([]*net.IPNet, error) {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return util.ParseNodeHostSubnetAnnotation(updatedNode, ovntypes.DefaultNetworkName)
					}, 2).Should(gomega.HaveLen(1))
				}

				node1, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				globalZoneJoinSubnetForNode1 := node1.Annotations["k8s.ovn.org/zone-join-subnets"]

				node2, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node2", metav1.GetOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				globalZoneJoinSubnetForNode2 := node2.Annotations["k8s.ovn.org/zone-join-subnets"]
				gomega.Expect(globalZoneJoinSubnetForNode1).To(gomega.Equal(globalZoneJoinSubnetForNode2))

				// Change the zone of nodes.
				for _, n := range nodes {
					zoneChanged := false
					// This loop makes sure that the node annotation has been updated successfully
					for !zoneChanged {
						updatedNode, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						nodeAnnotator := kube.NewNodeAnnotator(&kube.Kube{kubeFakeClient}, updatedNode.Name)

						nodeAnnotations := updatedNode.Annotations
						z, ok := updatedNode.Annotations["k8s.ovn.org/ovn-zone"]
						if !ok || z != n.Name+"-zone" {
							for k, v := range nodeAnnotations {
								nodeAnnotator.Set(k, v)
							}
							nodeAnnotator.Set("k8s.ovn.org/ovn-zone", n.Name+"-zone")
							err = nodeAnnotator.Run()
							gomega.Expect(err).NotTo(gomega.HaveOccurred())
							zoneChanged = false
						} else {
							zoneChanged = true
						}
					}
				}

				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						updatedNodeJoinSubnetAnnotation, ok := updatedNode.Annotations["k8s.ovn.org/zone-join-subnets"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have zone join subnets", updatedNode.Name)
						}

						if updatedNodeJoinSubnetAnnotation == globalZoneJoinSubnetForNode1 {
							return fmt.Errorf("join subnet annotation for node %s should have different subnet than the global zone", updatedNode.Name)
						}
						joinSubnets, err := util.ParseZoneJoinSubnetsAnnotation(updatedNode, ovntypes.DefaultNetworkName)
						if err != nil {
							return fmt.Errorf("error parsing zone join subnet annotation for the node %s", updatedNode.Name)
						}

						if len(joinSubnets) < 1 {
							return fmt.Errorf("zone join subnet annotation for node1 is empty")
						}
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				// Reset the zone of nodes to global.
				for _, n := range nodes {
					zoneChanged := false
					// This loop makes sure that the node annotation has been updated successfully
					for !zoneChanged {
						updatedNode, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						nodeAnnotator := kube.NewNodeAnnotator(&kube.Kube{kubeFakeClient}, updatedNode.Name)

						nodeAnnotations := updatedNode.Annotations
						z, ok := updatedNode.Annotations["k8s.ovn.org/ovn-zone"]
						if !ok || z != "global" {
							for k, v := range nodeAnnotations {
								nodeAnnotator.Set(k, v)
							}
							nodeAnnotator.Set("k8s.ovn.org/ovn-zone", "global")
							err = nodeAnnotator.Run()
							gomega.Expect(err).NotTo(gomega.HaveOccurred())
							zoneChanged = false
						} else {
							zoneChanged = true
						}
					}
				}

				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						updatedNodeJoinSubnetAnnotation, ok := updatedNode.Annotations["k8s.ovn.org/zone-join-subnets"]
						if !ok {
							return fmt.Errorf("expected node annotation for node %s to have zone join subnets", updatedNode.Name)
						}

						if updatedNodeJoinSubnetAnnotation != globalZoneJoinSubnetForNode1 {
							return fmt.Errorf("join subnet annotation for node %s should have the global zone join subnet", updatedNode.Name)
						}
						joinSubnets, err := util.ParseZoneJoinSubnetsAnnotation(updatedNode, ovntypes.DefaultNetworkName)
						if err != nil {
							return fmt.Errorf("error parsing zone join subnet annotation for the node %s", updatedNode.Name)
						}

						if len(joinSubnets) < 1 {
							return fmt.Errorf("zone join subnet annotation for node1 is empty")
						}
						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}
				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("Transit switch port IP allocations", func() {
		ginkgo.It("Interconnect enabled", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has allocated id transit switch port ips for each node
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						_, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-transit-switch-port-ips"]
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

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
				"--enable-interconnect",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Interconnect enabled - clear the transit switch port ips and check", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has allocated id transit switch port ips for each node
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						_, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-transit-switch-port-ips"]
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

				// Clear the transit switch port ip annotation from node 1.
				node1, _ := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
				nodeAnnotations := node1.Annotations
				nodeAnnotator := kube.NewNodeAnnotator(&kube.Kube{kubeFakeClient}, "node1")
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
				"--enable-interconnect",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Interconnect disabled", func() {
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
				}
				kubeFakeClient := fake.NewSimpleClientset(&v1.NodeList{
					Items: nodes,
				})
				fakeClient := &util.OVNClusterManagerClientset{
					KubeClient: kubeFakeClient,
				}

				_, err := config.InitConfig(ctx, nil, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				config.Kubernetes.HostNetworkNamespace = ""

				f, err = factory.NewClusterManagerWatchFactory(fakeClient)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = f.Start()
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				clusterManager, err := NewClusterManager(fakeClient, f, "identity", wg, nil)
				gomega.Expect(clusterManager).NotTo(gomega.BeNil())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = clusterManager.Start(ctx.Context)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				defer clusterManager.Stop()

				// Check that cluster manager has allocated id transit switch port ips for each node
				for _, n := range nodes {
					gomega.Eventually(func() error {
						updatedNode, err := fakeClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), n.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						_, ok := updatedNode.Annotations["k8s.ovn.org/ovn-node-transit-switch-port-ips"]
						if ok {
							return fmt.Errorf("not expected node annotation for node %s to have transit switch port ips allocated", n.Name)
						}

						return nil
					}).ShouldNot(gomega.HaveOccurred())
				}

				return nil
			}

			err := app.Run([]string{
				app.Name,
				"-cluster-subnets=" + clusterCIDR,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

})
