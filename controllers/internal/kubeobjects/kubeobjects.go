package kubeobjects

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var AzAppKubeObjects []client.Object

type KubeClient struct {
	client.Client
	context      context.Context
	applyOptions []client.PatchOption
}

func NewKubeClient(ctx context.Context, c client.Client, applyOptions []client.PatchOption) *KubeClient {
	return &KubeClient{
		Client:       c,
		context:      ctx,
		applyOptions: applyOptions,
	}
}

func (k *KubeClient) ApplyAll(kubeobjects []client.Object) error {
	logr := logr.FromContextOrDiscard(k.context)
	logr.Info("Applying changes to kubernetes objects")
	for _, ko := range kubeobjects {
		if err := k.Patch(k.context, ko, client.Apply, k.applyOptions...); err != nil {
			return err
		}
	}
	return nil
}

func (k *KubeClient) SetProvisionState(provState string, azapp *k8sappv0alpha1.AzureApp) error {
	logr := logr.FromContextOrDiscard(k.context)
	if provState != azapp.Status.ProvisioningState {
		logr.Info(fmt.Sprintf("Setting provisioning state for app [%s]", azapp.Name))
		originalAzapp := azapp.DeepCopy()
		azapp.Status.ProvisioningState = provState
		patch := client.MergeFrom(originalAzapp)
		if err := k.Status().Patch(k.context, azapp, patch); err != nil {
			return err
		}
		logr.Info(fmt.Sprintf("Successfully set provisioning state for app [%s] to: %s", azapp.Name, provState))
	}
	return nil
}

func (k *KubeClient) SetDeploymentName(deployment string, azapp *k8sappv0alpha1.AzureApp) error {
	logr := logr.FromContextOrDiscard(k.context)
	if deployment != azapp.Status.Deployment {
		logr.Info(fmt.Sprintf("Setting provisioning state for app [%s]", azapp.Name))
		originalAzapp := azapp.DeepCopy()
		azapp.Status.Deployment = deployment
		patch := client.MergeFrom(originalAzapp)
		if err := k.Status().Patch(k.context, azapp, patch); err != nil {
			return err
		}
		logr.Info(fmt.Sprintf("Successfully set deployment state for app [%s] to: %s", azapp.Name, deployment))
	}
	return nil
}
