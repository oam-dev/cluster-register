/*
 Copyright 2021. The KubeVela Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spoke

import (
	"context"
	"time"

	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rabcv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ocmapiv1 "open-cluster-management.io/api/operator/v1"
	ocmworkv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/cluster-register/pkg/common"
)

func CleanSpokeClusterEnv(config *rest.Config) error {
	cli, err := client.New(config, client.Options{Scheme: common.Scheme})
	if err != nil {
		return err
	}

	if IsAppliedManifestWorkExist(cli) {
		klog.Fatal("AppliedManifestWork exist on the managed cluster,")
	}

	ctx := context.Background()
	err = wait.PollImmediate(1*time.Second, 2*time.Minute, func() (done bool, err error) {
		klusterlet := ocmapiv1.Klusterlet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "klusterlet",
			},
		}
		if err = cli.Delete(ctx, &klusterlet); client.IgnoreNotFound(err) != nil {
			return false, err
		}

		err = cli.Get(ctx, client.ObjectKeyFromObject(&klusterlet), &klusterlet)
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})

	if err != nil {
		return err
	}

	clusterRole := rabcv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "klusterlet",
		},
	}
	if err = cli.Delete(ctx, &clusterRole); client.IgnoreNotFound(err) != nil {
		return err
	}
	clusterRoleBinding := rabcv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "klusterlet",
		},
	}
	if err = cli.Delete(ctx, &clusterRoleBinding); client.IgnoreNotFound(err) != nil {
		return err
	}

	deployment := appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "klusterlet",
			Namespace: "open-cluster-management",
		},
	}
	if err = cli.Delete(ctx, &deployment); client.IgnoreNotFound(err) != nil {
		return err
	}

	serviceAccount := v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "klusterlet",
			Namespace: "open-cluster-management",
		},
	}
	if err = cli.Delete(ctx, &serviceAccount); client.IgnoreNotFound(err) != nil {
		return err
	}
	ns := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management-agent",
		},
	}
	if err = cli.Delete(ctx, &ns); client.IgnoreNotFound(err) != nil {
		return err
	}

	ocmNs := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management",
		},
	}
	if err = cli.Delete(ctx, &ocmNs); client.IgnoreNotFound(err) != nil {
		return err
	}

	bootstrapServiceAccount := v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-bootstrap",
			Namespace: "open-cluster-management",
		},
	}
	if err = cli.Delete(ctx, &bootstrapServiceAccount); client.IgnoreNotFound(err) != nil {
		return err
	}
	bootstrapClusterRole := rabcv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "system:open-cluster-management:bootstrap",
		},
	}
	if err = cli.Delete(ctx, &bootstrapClusterRole); client.IgnoreNotFound(err) != nil {
		return err
	}
	bootstrapClusterRoleBinding := rabcv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-bootstrap-sa",
		},
	}
	if err = cli.Delete(ctx, &bootstrapClusterRoleBinding); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

func IsAppliedManifestWorkExist(cli client.Client) bool {
	appliedManifest := ocmworkv1.AppliedManifestWorkList{}
	if err := cli.List(context.Background(), &appliedManifest); err != nil {
		klog.Fatal(err)
	}
	if len(appliedManifest.Items) > 0 {
		return true
	}
	return false
}
