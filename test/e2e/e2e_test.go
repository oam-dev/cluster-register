//go:build e2e

// Run via `make e2e` or test/e2e/scripts/run.sh, which bring up the kind
// clusters this test expects and set its env vars.
package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"
	ocmapiv1 "open-cluster-management.io/api/operator/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/cluster-register/pkg/common"
	"github.com/oam-dev/cluster-register/pkg/hub"
	"github.com/oam-dev/cluster-register/pkg/spoke"
)

func TestClusterRegistrationLifecycle(t *testing.T) {
	hubKubeconfig := requireEnv(t, "E2E_HUB_KUBECONFIG")
	spokeKubeconfig := requireEnv(t, "E2E_SPOKE_KUBECONFIG")
	hubAPIServer := requireEnv(t, "E2E_HUB_API_SERVER")
	clusterName := envOrDefault("E2E_CLUSTER_NAME", "e2e-spoke")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	hubRestConfig, err := clientcmd.BuildConfigFromFlags("", hubKubeconfig)
	if err != nil {
		t.Fatalf("failed to build hub rest config: %v", err)
	}
	spokeRestConfig, err := clientcmd.BuildConfigFromFlags("", spokeKubeconfig)
	if err != nil {
		t.Fatalf("failed to build spoke rest config: %v", err)
	}

	hubCluster, err := hub.NewHubCluster(hubRestConfig)
	if err != nil {
		t.Fatalf("failed to connect to hub cluster: %v", err)
	}

	t.Log("generating hub bootstrap kubeconfig for the spoke cluster")
	hubBootstrapKubeConfig, err := hubCluster.GenerateHubClusterKubeConfig(ctx, hubAPIServer)
	if err != nil {
		t.Fatalf("failed to generate hub bootstrap kubeconfig: %v", err)
	}

	spokeCluster, err := spoke.NewSpokeCluster(clusterName, spokeRestConfig, hubBootstrapKubeConfig)
	if err != nil {
		t.Fatalf("failed to connect to spoke cluster: %v", err)
	}

	t.Log("register: applying klusterlet operator + CR to the spoke cluster")
	if err := spokeCluster.InitSpokeClusterEnv(ctx); err != nil {
		t.Fatalf("failed to init spoke cluster env: %v", err)
	}

	t.Log("register: waiting for the klusterlet operator to become ready")
	if err := spokeCluster.WaitForRegistrationOperatorReady(ctx); err != nil {
		t.Fatalf("registration operator never became ready: %v", err)
	}

	t.Log("register: waiting for the registration agent to become ready")
	if err := spokeCluster.WaitForRegistrationAgentReady(ctx); err != nil {
		t.Fatalf("registration agent never became ready: %v", err)
	}

	t.Log("register: waiting for the spoke's CSR to appear on the hub")
	if err := hubCluster.WaitForCSRCreated(ctx, clusterName); err != nil {
		t.Fatalf("csr was never created on the hub: %v", err)
	}

	t.Log("approve: approving the CSR and accepting the managed cluster on the hub")
	if err := hubCluster.RegisterSpokeCluster(ctx, clusterName); err != nil {
		t.Fatalf("failed to approve spoke cluster: %v", err)
	}

	t.Log("verify-joined: waiting for the ManagedCluster to report Joined=True")
	if err := waitForManagedClusterJoined(ctx, hubCluster.Client, clusterName); err != nil {
		t.Fatalf("managed cluster never joined: %v", err)
	}

	mc := &ocmclusterv1.ManagedCluster{}
	if err := hubCluster.Client.Get(ctx, client.ObjectKey{Name: clusterName}, mc); err != nil {
		t.Fatalf("failed to fetch joined managed cluster: %v", err)
	}
	if !mc.Spec.HubAcceptsClient {
		t.Fatalf("managed cluster %q joined but hub never accepted it (spec.hubAcceptsClient=false)", clusterName)
	}
	if !meta.IsStatusConditionTrue(mc.Status.Conditions, ocmclusterv1.ManagedClusterConditionHubAccepted) {
		t.Fatalf("managed cluster %q joined but is missing condition %q", clusterName, ocmclusterv1.ManagedClusterConditionHubAccepted)
	}

	t.Log("clean: removing the klusterlet and agent namespaces from the spoke cluster")
	if err := spoke.CleanSpokeClusterEnv(spokeRestConfig); err != nil {
		t.Fatalf("failed to clean spoke cluster env: %v", err)
	}

	t.Log("clean: verifying the klusterlet was actually removed from the spoke cluster")
	if err := waitForKlusterletRemoved(ctx, spokeRestConfig); err != nil {
		t.Fatalf("klusterlet was not fully cleaned up: %v", err)
	}
}

func waitForManagedClusterJoined(ctx context.Context, c client.Client, name string) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		mc := &ocmclusterv1.ManagedCluster{}
		if err := c.Get(ctx, client.ObjectKey{Name: name}, mc); err != nil {
			if kerrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return meta.IsStatusConditionTrue(mc.Status.Conditions, ocmclusterv1.ManagedClusterConditionJoined), nil
	})
}

func waitForKlusterletRemoved(ctx context.Context, spokeConfig *rest.Config) error {
	cli, err := client.New(spokeConfig, client.Options{Scheme: common.Scheme})
	if err != nil {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		klusterlet := &ocmapiv1.Klusterlet{}
		err := cli.Get(ctx, client.ObjectKey{Name: "klusterlet"}, klusterlet)
		if kerrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Fatalf("%s must be set; run this suite via `make e2e` or test/e2e/scripts/run.sh", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
