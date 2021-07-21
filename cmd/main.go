package main

import (
	"context"
	"encoding/base64"
	"flag"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/oam-dev/cluster-register/pkg/hub"
	"github.com/oam-dev/cluster-register/pkg/spoke"
)

func main() {
	var clusterName string
	var hubIP string
	var decode bool
	var spokeInfo spoke.SpokeInfo
	flag.StringVar(&hubIP, "hub-api-server", "", "external apiserver address of hub cluster")
	flag.StringVar(&clusterName, "cluster-name", "", "name of managed cluster")
	flag.StringVar(&spokeInfo.CACert, "cluster-ca-cert", "", "ca certificate of managed cluster")
	flag.StringVar(&spokeInfo.ClientCert, "client-cert", "", "ca certificate of client for TLS auth")
	flag.StringVar(&spokeInfo.ClientKey, "client-key", "", "key of client for TLS auth")
	flag.StringVar(&spokeInfo.APIServer, "api-server-internet", "", "external apiserver address of managed cluster")
	flag.StringVar(&spokeInfo.KubeConfig, "kube-config", "", "kubeconfig of managed cluster")
	flag.BoolVar(&decode, "decode", false, "decode the parameter")

	flag.Parse()

	if decode {
		clusterName = DecodeParameter(clusterName)
		spokeInfo.CACert = DecodeParameter(spokeInfo.CACert)
		spokeInfo.ClientCert = DecodeParameter(spokeInfo.ClientCert)
		spokeInfo.ClientKey = DecodeParameter(spokeInfo.ClientKey)
		spokeInfo.APIServer = DecodeParameter(spokeInfo.APIServer)
		spokeInfo.KubeConfig = DecodeParameter(spokeInfo.KubeConfig)
	}

	ctx := context.Background()

	// 1. connect to hub-cluster, which job(ocm-register-assistant) was deployed to
	hubCluster, err := hub.NewHubCluster(nil)
	if err != nil {
		klog.InfoS("Fail to create client connect to hub cluster")
		os.Exit(1)
	}

	var spokeConfig *rest.Config
	if len(spokeInfo.KubeConfig) != 0 {
		spokeConfig, err = hubCluster.GetSpokeClusterConfig(spokeInfo.KubeConfig)
		if err != nil || spokeConfig == nil {
			klog.InfoS("Fail to get spoke-cluster kubeconfig", "err", err)
			os.Exit(1)
		}
	} else {
		legoConfig := spokeInfo.CreateKubeConfig()
		spokeConfig, err = hub.ConvertSpokeKubeConfig(&legoConfig)
		if err != nil || spokeConfig == nil {
			klog.InfoS("Fail to convert spoke-cluster kubeconfig", "err", err)
			os.Exit(1)
		}
	}

	klog.Info("generate the token for spoke-cluster to connect hub-cluster")
	hubKubeConfig, err := hubCluster.GenerateHubClusterKubeConfig(ctx, hubIP)
	if err != nil {
		klog.InfoS("Fail to generate the token for spoke-cluster", "err", err)
		os.Exit(1)
	}

	// 2. connect to spoke-cluster
	spokeCluster, err := spoke.NewSpokeCluster(clusterName, spokeConfig, hubKubeConfig)
	if err != nil {
		klog.InfoS("Fail to connect spoke cluster", "err", err)
		os.Exit(1)
	}

	klog.InfoS("prepare the env for spoke-cluster", "name", clusterName)
	err = spokeCluster.InitSpokeClusterEnv(ctx)
	if err != nil {
		klog.InfoS("Fail to prepare the env for spoke-cluster", "err", err)
		os.Exit(1)
	}

	klog.Info("wait for spoke-cluster register request")
	ready, err := hubCluster.Wait4SpokeClusterReady(ctx, clusterName)
	if err != nil || !ready {
		klog.Error(err, "Fail to waiting for register request")
		os.Exit(1)
	}

	klog.Info("approve spoke cluster csr")
	err = hubCluster.RegisterSpokeCluster(ctx, spokeCluster.Name)
	if err != nil {
		klog.Error(err, "Fail to approve spoke cluster")
		os.Exit(1)
	}
	klog.InfoS("successfully register cluster", "name", clusterName)

	os.Exit(0)
}

func DecodeParameter(data string) string {
	decode, _ := base64.StdEncoding.DecodeString(data)
	return string(decode)
}
