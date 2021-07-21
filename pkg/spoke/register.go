package spoke

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
	ocmapiv1 "open-cluster-management.io/api/operator/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/cluster-register/pkg/common"
)

type Cluster struct {
	Name string
	Args common.Args
	HubInfo
}

type SpokeInfo struct {
	CACert     string
	ClientCert string
	ClientKey  string
	APIServer  string
	KubeConfig string
}

func (s SpokeInfo) CreateKubeConfig() clientcmdapiv1.Config {

	var kubeConfig clientcmdapiv1.Config

	caCert, _ := base64.StdEncoding.DecodeString(s.CACert)
	clientCert, _ := base64.StdEncoding.DecodeString(s.ClientCert)
	clientKey, _ := base64.StdEncoding.DecodeString(s.ClientKey)

	kubeConfig.Clusters = []clientcmdapiv1.NamedCluster{
		{
			Name: "spoke",
			Cluster: clientcmdapiv1.Cluster{
				Server:                   s.APIServer,
				CertificateAuthorityData: caCert,
			},
		},
	}

	kubeConfig.Contexts = []clientcmdapiv1.NamedContext{
		{
			Name: "init",
			Context: clientcmdapiv1.Context{
				Cluster:   "spoke",
				AuthInfo:  "register-job",
				Namespace: "default",
			},
		},
	}
	kubeConfig.CurrentContext = "init"
	kubeConfig.AuthInfos = []clientcmdapiv1.NamedAuthInfo{
		{
			Name: "register-job",
			AuthInfo: clientcmdapiv1.AuthInfo{
				ClientCertificateData: clientCert,
				ClientKeyData:         clientKey,
			},
		},
	}
	res, _ := yaml.Marshal(kubeConfig)
	klog.Info(string(res))
	return kubeConfig
}

type HubInfo struct {
	KubeConfig *clientcmdapiv1.Config
	APIServer  string
}

//go:embed resource
var f embed.FS

func getHubAPIServer(hubConfig *clientcmdapiv1.Config) (string, error) {
	clusters := hubConfig.Clusters
	if len(clusters) != 1 {
		return "", fmt.Errorf("can not find the cluster in the cluster-info")
	}
	cluster := clusters[0].Cluster
	return cluster.Server, nil
}

func NewSpokeCluster(name string, config *rest.Config, hubConfig *clientcmdapiv1.Config) (*Cluster, error) {
	args := common.Args{
		Schema: common.Scheme,
	}
	err := args.SetConfig(config)
	if err != nil {
		return nil, err
	}
	err = args.SetClient()
	if err != nil {
		return nil, err
	}

	apiserver, err := getHubAPIServer(hubConfig)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		Name: name,
		Args: args,
		HubInfo: HubInfo{
			KubeConfig: hubConfig,
			APIServer:  apiserver,
		},
	}, nil
}

func (c *Cluster) InitSpokeClusterEnv(ctx context.Context) error {
	files := []string{
		"resource/namespace_agent.yaml",
		"resource/namespace.yaml",
		"resource/cluster_role.yaml",
		"resource/cluster_role_binding.yaml",
		"resource/klusterlets.crd.yaml",
		"resource/service_account.yaml",
	}
	// 1. apply ns rbac crd
	err := common.ApplyK8sResource(ctx, f, c.Args.Client, files)
	if err != nil {
		return err
	}

	// 2. render secret contains hub kubeconfig
	hubConfigSecret := "resource/bootstrap_hub_kubeconfig.yaml"
	err = applyHubKubeConfig(ctx, c.Args.Client, hubConfigSecret, c.HubInfo.KubeConfig)
	if err != nil {
		return err
	}

	// 3. apply deployment
	opreatorFile := []string{"resource/operator.yaml"}
	err = common.ApplyK8sResource(ctx, f, c.Args.Client, opreatorFile)
	if err != nil {
		return err
	}

	// 4. apply klusterlet
	klusterFile := "resource/klusterlets.cr.yaml"
	err = applyKlusterlet(ctx, c.Args.Client, klusterFile, c)
	if err != nil {
		return err
	}
	return nil
}

func applyHubKubeConfig(ctx context.Context, k8sClient client.Client, file string, kubeConfig *clientcmdapiv1.Config) error {
	path := strings.Split(file, "/")
	templateName := path[len(path)-1]
	t, err := template.New(templateName).Funcs(sprig.TxtFuncMap()).ParseFS(f, file)
	if err != nil {
		klog.Error(err, "Fail to get Template from file", "name", file)
		return err
	}

	kubeConfigData, err := yaml.Marshal(kubeConfig)
	if err != nil {
		klog.Error(err)
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, string(kubeConfigData))
	if err != nil {
		klog.Error(err)
		return err
	}

	kubeConfigSecret := new(corev1.Secret)
	err = yaml.Unmarshal(buf.Bytes(), kubeConfigSecret)
	if err != nil {
		klog.Error(err)
		return err
	}

	err = k8sClient.Get(ctx, client.ObjectKey{Namespace: kubeConfigSecret.Namespace, Name: kubeConfigSecret.Name}, kubeConfigSecret)
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.V(common.LogDebug).InfoS("create secret", "object", klog.KObj(kubeConfigSecret))
			return k8sClient.Create(ctx, kubeConfigSecret)
		}
		return err
	}

	klog.V(common.LogDebug).InfoS("update secret", "object", klog.KObj(kubeConfigSecret))
	return k8sClient.Update(ctx, kubeConfigSecret)
}

func applyKlusterlet(ctx context.Context, k8sClient client.Client, file string, cluster *Cluster) error {
	t, err := template.ParseFS(f, file)
	if err != nil {
		klog.Error(err, "Fail to get Template from file", "name", file)
		return err
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, cluster)
	if err != nil {
		klog.Error(err, "Fail to render klusterlet")
		return err
	}

	klusterlet := new(ocmapiv1.Klusterlet)
	err = yaml.Unmarshal(buf.Bytes(), klusterlet)
	if err != nil {
		klog.Error(err, "Fail to Unmarshal klusterlet")
		return err
	}

	err = k8sClient.Get(ctx, client.ObjectKey{Name: klusterlet.Name, Namespace: klusterlet.Namespace}, klusterlet)
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.V(common.LogDebug).InfoS("create klusterlet", "object", klog.KObj(klusterlet))
			return k8sClient.Create(ctx, klusterlet)
		}
		return err
	}
	klog.V(common.LogDebug).InfoS("update klusterlet", "object", klog.KObj(klusterlet))
	return k8sClient.Update(ctx, klusterlet)
}
