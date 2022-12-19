package kubeobjects

import (
	"context"

	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var AzAppKubeObjects []client.Object

type KubeClient struct {
	client.Client
	context      context.Context
	applyOptions []client.PatchOption
}

func NewKubeClient(c client.Client, ctx context.Context, applyOptions []client.PatchOption) *KubeClient {
	return &KubeClient{
		Client:       c,
		context:      ctx,
		applyOptions: applyOptions,
	}
}

func (k *KubeClient) ApplyAll(kubeobjects []client.Object) error {
	for _, ko := range kubeobjects {
		if err := k.Patch(k.context, ko, client.Apply, k.applyOptions...); err != nil {
			return err
		}
	}
	return nil
}

func (k *KubeClient) SetProvisionState(provState string, azapp *k8sappv0alpha1.AzureApp) error {
	azapp.Status.ProvisioningState = provState
	if err := k.Status().Update(k.context, azapp); err != nil {
		return err
	}
	return nil
}
