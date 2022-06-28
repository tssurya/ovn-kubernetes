package ovn

import (
	"sync"
	"sync/atomic"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	libovsdbclient "github.com/ovn-org/libovsdb/client"
	cm "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/clustermanager"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	egressfirewallfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressfirewall/v1/apis/clientset/versioned/fake"
	egressipfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressip/v1/apis/clientset/versioned/fake"
	egressqosfake "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressqos/v1/apis/clientset/versioned/fake"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	addressset "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/ovn/address_set"
	lsm "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/ovn/logical_switch_manager"
	libovsdbtest "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/testing/libovsdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"

	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

var _ = ginkgo.Describe("OVN Interconnect Operations", func() {
	var (
		app             *cli.App
		f               *factory.WatchFactory
		stopChan        chan struct{}
		wg              *sync.WaitGroup
		libovsdbCleanup *libovsdbtest.Cleanup
	)

	const (
		clusterIPNet   string = "10.1.0.0"
		clusterCIDR    string = clusterIPNet + "/16"
		joinSubnetCIDR string = "100.64.0.0/16/19"
	)

	ginkgo.BeforeEach(func() {
		// Restore global default values before each testcase
		//gomega.Expect(config.PrepareTestConfig()).To(gomega.Succeed())

		app = cli.NewApp()
		app.Name = "test"
		app.Flags = config.Flags
		stopChan = make(chan struct{})
		wg = &sync.WaitGroup{}
		libovsdbCleanup = nil
	})

	ginkgo.AfterEach(func() {
		close(stopChan)
		wg.Wait()
		if libovsdbCleanup != nil {
			libovsdbCleanup.Cleanup()
		}
		f.Shutdown()
		wg.Wait()
	})

	ginkgo.It("OVN Interconnect test", func() {
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

			expectedClusterLBGroup := newLoadBalancerGroup()

			dbSetup := libovsdbtest.TestSetup{}

			_, err := config.InitConfig(ctx, nil, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			config.Kubernetes.HostNetworkNamespace = ""

			f, err = factory.NewClusterManagerWatchFactory(fakeClient)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = f.Start()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			clusterManager := cm.NewClusterManager(fakeClient, f, stopChan, wg,
				record.NewFakeRecorder(0))
			gomega.Expect(clusterManager).NotTo(gomega.BeNil())

			clusterManager.InitSubnetAllocatorRanges(config.Default.ClusterSubnets)

			clusterManager.InitZoneSubnetAllocatorRanges(config.ClusterManager.ZoneJoinSubnets)

			gomega.Expect(clusterManager.WatchNodes()).To(gomega.Succeed())

			var libovsdbOvnNBClient, libovsdbOvnSBClient libovsdbclient.Client
			libovsdbOvnNBClient, libovsdbOvnSBClient, libovsdbCleanup, err = libovsdbtest.NewNBSBTestHarness(dbSetup)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			clusterController := NewOvnController(fakeClient, f, stopChan, addressset.NewFakeAddressSetFactory(),
				libovsdbOvnNBClient, libovsdbOvnSBClient,
				record.NewFakeRecorder(10), wg)
			gomega.Expect(clusterController).NotTo(gomega.BeNil())

			clusterController.SCTPSupport = true
			clusterController.loadBalancerGroupUUID = ""
			clusterController.defaultCOPPUUID, err = EnsureDefaultCOPP(clusterController.nbClient)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			clusterController.joinSwIPManager, _ = lsm.NewJoinLogicalSwitchIPManager(clusterController.nbClient, expectedClusterLBGroup.UUID, []string{"node1"}, getJoinSwitchSubnets())

			existingNodes, err := clusterController.kube.GetNodes()
			if err == nil {
				for _, node := range existingNodes.Items {
					clusterController.localZoneNodes.Store(node.Name, true)
				}
			}

			//assuming all the pods have finished processing
			atomic.StoreUint32(&clusterController.allInitialPodsProcessed, 1)
			// Let the real code run and ensure OVN database sync
			gomega.Expect(clusterController.WatchNodes()).To(gomega.Succeed())

			return nil
		}

		err := app.Run([]string{
			app.Name,
			"-cluster-subnets=" + clusterCIDR,
			"-init-cluster-manager",
			"-zone-join-switch-subnets=" + joinSubnetCIDR,
			"-enable-interconnect",
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})
})
