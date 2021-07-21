# cluster-register

cluster-register can help KubeVela users to register a new cluster in a multi-cluster environment. 

## Prerequisite

1. Prepare one Kubernetes cluster to function as the hub and one Kubernetes cluster as spoke cluster.
For example, use `kind` to create hub cluster and spoke cluster. To use `kind`, you will need `docker` installed and running.

```shell
kind create cluster --name hub
kind create cluster --name cluster1
```

2. Install KubeVela in hub cluster

```shell
kubectl config use-context kind-hub
helm install --create-namespace -n vela-system kubevela kubevela/vela-core
```

## Usage

cluster-register supports registering Managed Cluster by OCM.

1. Use Initializer `ocm-cluster-manager` to create a Hub Cluster environment.

```shell
# change to hub cluster
kubectl config use-context kind-hub
```

```shell
kubectl apply -f https://raw.githubusercontent.com/oam-dev/kubevela/master/vela-templates/addons/auto-gen/ocm-cluster-manager.yaml
```

2. Export the kubeconfig of the Managed Cluster and store it in the Secret of the Hub Cluster

```shell
# 1. export cluster1 kubeconfig to .cluster1-kubeconfig
kind get kubeconfig --name cluster1  --internal > .cluster1-kubeconfig
# 2. store kubeconfig in secret of hub-cluster
kubectl create secret generic spoke-kubeconfig --from-file=kubeconfig=.cluster1-kubeconfig --from-literal=name=kind-cluster1
```

cluster-register also supports combine the kubeconfig of the spoke cluster with the certificate and key provided by the user.
So the Secret should provide the necessary values like the following example:

```yaml
apiVersion: v1
data:
  # api_server_internet maps to clusters[0].cluster.server in kubeconfig, represent to the apiserver of spoke cluster
  api_server_internet: XXXXX
  # client_cert maps to users[0].user.client-certificate-data
  client_cert: XXXXX
  # client_key maps to users[0].user.client-key-data
  client_key: XXXXX
  # cluster_ca_cert maps to clusters[0].cluster.certificate-authority-data
  cluster_ca_cert: XXXXX
  # You can also choose to provide a kubeconfig file, cluster-register will give priority to the user-provided kubeconfig
  kubeconfig: XXXXX
  name: kind-cluster1
kind: Secret
metadata:
  name: spoke-kubeconfig
type: Opaque
```

3. Create the cluster-register Job

```shell
kubectl apply -f manifest
```

if hub cluster and spoke cluster are not in the same VPC, you should provide the external address of the hub cluster.

```yaml
apiVersion: core.oam.dev/v1beta1
kind: Application
metadata:
  name: cluster-register
  namespace: default
spec:
  components:
    - name: register
      type: cluster-register
      properties:
        clusterSecret: spoke-kubeconfig
        hubAPIServer: "apiserver address"
```
4. Wait for the Managed Cluster is available

```shell
$ kubectl get managedclusters.cluster.open-cluster-management.io --watch
NAME            HUB ACCEPTED   MANAGED CLUSTER URLS             JOINED   AVAILABLE   AGE
kind-cluster1   true           https://hub-control-plane:6443   True     True        78m
```

5. Delete the Secret

```shell
kubectl delete secret spoke-kubeconfig
```