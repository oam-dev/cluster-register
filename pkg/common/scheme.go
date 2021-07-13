package common

import (
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"
	ocmapiv1 "open-cluster-management.io/api/operator/v1"
)

var (
	Scheme = k8sruntime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(Scheme)
	_ = crdv1.AddToScheme(Scheme)
	_ = ocmapiv1.Install(Scheme)
	_ = ocmclusterv1.Install(Scheme)
}
