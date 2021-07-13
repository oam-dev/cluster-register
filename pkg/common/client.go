package common

import (
	"context"
	"embed"

	"github.com/ghodss/yaml"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	OpenClusterManagementNamespace    = "open-cluster-management"
	BootstrapSAName                   = "cluster-bootstrap"
	HubClusterName                    = "hub"
)

type Args struct {
	KubeConfig *rest.Config
	Schema     *runtime.Scheme
	Client     client.Client
}

func (a *Args) SetConfig(kconfig *rest.Config) error {
	if kconfig != nil {
		a.KubeConfig = kconfig
		return nil
	}
	kubeConfig, err := config.GetConfig()
	if err != nil {
		return err
	}
	a.KubeConfig = kubeConfig
	return nil
}

func (a *Args) SetClient() error {
	if a.KubeConfig == nil {
		err := a.SetConfig(nil)
		if err != nil {
			return err
		}
	}

	if a.Schema == nil {
		a.Schema = Scheme
	}

	newClient, err := client.New(a.KubeConfig, client.Options{Scheme: a.Schema})
	if err != nil {
		return err
	}
	a.Client = newClient
	return nil
}

func ApplyK8sResource(ctx context.Context, f embed.FS, k8sClient client.Client, files []string) error {
	for _, file := range files {
		data, err := f.ReadFile(file)
		if err != nil {
			klog.Error(err, "Fail to read embed file ", "name:", file)
			return err
		}
		k8sObject := new(unstructured.Unstructured)
		err = yaml.Unmarshal(data, k8sObject)
		if err != nil {
			klog.Error(err, "Fail to unmarshal file", "name", file)
			return err
		}
		err = CreateOrUpdateResource(ctx, k8sClient, k8sObject)
		if err != nil {
			klog.InfoS("Fail to create resource", "object", klog.KObj(k8sObject), "apiVersion", k8sObject.GetAPIVersion(), "kind", k8sObject.GetKind())
			return err
		}
		klog.InfoS("Successfully create resource", "object", klog.KObj(k8sObject), "apiVersion", k8sObject.GetAPIVersion(), "kind", k8sObject.GetKind())
	}
	return nil
}

func CreateOrUpdateResource(ctx context.Context, k8sClient client.Client, resource *unstructured.Unstructured) error {
	objKey := client.ObjectKey{Name: resource.GetName(), Namespace: resource.GetNamespace()}
	if err := k8sClient.Get(ctx, objKey, resource); err != nil {
		if kerrors.IsNotFound(err) {
			return k8sClient.Create(ctx, resource)
		}
		return err
	}
	return k8sClient.Update(ctx, resource)
}
