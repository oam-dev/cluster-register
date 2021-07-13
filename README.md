# cluster-register

cluster-register can help KubeVela users to register a new cluster in a multi-cluster environment. 

## Usage

Now cluster-register supports registering Managed Cluster in OCM. The usage is as follows:

1. Use `Initializer` to create an Hub Cluster environment.

```shell
kubectl apply -f https://raw.githubusercontent.com/oam-dev/kubevela/master/vela-templates/addons/auto-gen/ocm-cluster-manager.yaml
```

2. Export the kubeconfig of the Managed Cluster and store it in the Secret of the Hub Cluster

```shell
# 1. export cluster1 kubeconfig to .cluster1-kubeconfig
# 2. store kubeconfig in secret of hub-cluster
kubectl create secret generic spoke-kubeconfig --from-file=kubeconfig=.cluster1-kubeconfig --from-literal=name=kind-cluster1
```

3. Create the cluster-register Job

```shell
kubectl apply -f manifest
```

4. Wait for the Managed Cluster is available

```shell
$ kubectl get managedclusters.cluster.open-cluster-management.io
NAME            HUB ACCEPTED   MANAGED CLUSTER URLS             JOINED   AVAILABLE   AGE
kind-cluster1   true           https://hub-control-plane:6443   True     True        78m
```

5. Delete the Secret

```shell
kubectl delete secret spoke-kubeconfig
```