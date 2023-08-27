package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/dependencies"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/kubeobjects"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var ErrFileNotExist = errors.New("spec file does not exist")

func (r *AzureAppReconciler) desiredDeployment(azapp *k8sappv0alpha1.AzureApp, appCreds corev1.Secret) (appsv1.Deployment, error) {
	replicas := new(int32)
	*replicas = 1

	var envVars []corev1.EnvVar
	for k, v := range azapp.Spec.EnvVars {
		envVar := corev1.EnvVar{Name: k, Value: v}
		envVars = append(envVars, envVar)
	}
	envVars = append(envVars, corev1.EnvVar{
		Name: "AZURE_APP_ID",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: appCreds.ObjectMeta.Name},
				Key:                  "AZURE_APP_ID",
			},
		}})
	envVars = append(envVars, corev1.EnvVar{
		Name: "AZURE_APP_SECRET",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: appCreds.ObjectMeta.Name},
				Key:                  "AZURE_APP_SECRET",
			},
		}})

	depl := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azapp.Spec.Identifier,
			Namespace: azapp.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas, // won't be nil because defaulting
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"azureapp": azapp.Spec.Identifier},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"azureapp": azapp.Spec.Identifier},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  azapp.Spec.Identifier,
							Image: azapp.Spec.ContainerImage,
							Env:   envVars,
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(azapp, &depl, r.Scheme); err != nil {
		return depl, err
	}

	return depl, nil
}

func (r *AzureAppReconciler) desiredIngress(azapp *k8sappv0alpha1.AzureApp) (networkingv1.Ingress, error) {
	pathType := new(networkingv1.PathType)
	*pathType = "Prefix"
	ing := networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{APIVersion: networkingv1.SchemeGroupVersion.String(), Kind: "Ingress"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azapp.Spec.Identifier,
			Namespace: azapp.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: azapp.Spec.Url,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: azapp.Spec.Identifier,
											Port: networkingv1.ServiceBackendPort{Number: azapp.Spec.ServingPort},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(azapp, &ing, r.Scheme); err != nil {
		return ing, err
	}

	return ing, nil
}

func (r *AzureAppReconciler) desiredService(azapp *k8sappv0alpha1.AzureApp) (corev1.Service, error) {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azapp.Spec.Identifier,
			Namespace: azapp.Namespace,
			Labels:    map[string]string{"azureapp": azapp.Spec.Identifier},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: azapp.Spec.ServingPort, Protocol: "TCP", TargetPort: intstr.FromInt(int(azapp.Spec.ServingPort))},
			},
			Selector: map[string]string{"azureapp": azapp.Spec.Identifier},
			Type:     corev1.ServiceTypeNodePort,
		},
	}

	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(azapp, &svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}

func (r *AzureAppReconciler) desiredSecret(azappCred map[string]string, azapp *k8sappv0alpha1.AzureApp) (corev1.Secret, error) {
	secretMap := make(map[string]string)
	secretMap["AZURE_APP_ID"] = azappCred["appId"]
	secretMap["AZURE_APP_SECRET"] = azappCred["appSecret"]
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      azapp.Spec.Identifier,
			Namespace: azapp.Namespace,
			Labels:    map[string]string{"azureapp": azapp.Spec.Identifier},
		},
		StringData: secretMap,
	}

	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(azapp, &secret, r.Scheme); err != nil {
		return secret, err
	}

	return secret, nil
}

func (r *AzureAppReconciler) buildKubeObjects(azapp k8sappv0alpha1.AzureApp) ([]client.Object, error) {
	azappk8s := kubeobjects.AzAppKubeObjects
	appCredential, err := dependencies.GetTerraformAppCredentialOutput(&azapp)
	if err != nil {
		return nil, err
	}

	secret, err := r.desiredSecret(appCredential, &azapp)
	if err != nil {
		return nil, err
	}
	deployment, err := r.desiredDeployment(&azapp, secret)
	if err != nil {
		return nil, err
	}
	service, err := r.desiredService(&azapp)
	if err != nil {
		return nil, err
	}
	ingress, err := r.desiredIngress(&azapp)
	if err != nil {
		return nil, err
	}
	return append(azappk8s, &secret, &deployment, &service, &ingress), nil
}

func (r *AzureAppReconciler) SetupFinalizer(finalizerName string, azapp *k8sappv0alpha1.AzureApp) error {
	if !controllerutil.ContainsFinalizer(azapp, finalizerName) {
		controllerutil.AddFinalizer(azapp, finalizerName)
		if err := r.Update(context.Background(), azapp); err != nil {
			return err
		}
	}
	return nil
}

func (r *AzureAppReconciler) RemoveFinalizer(finalizerName string, azapp *k8sappv0alpha1.AzureApp) error {
	controllerutil.RemoveFinalizer(azapp, finalizerName)
	if err := r.Update(context.Background(), azapp); err != nil {
		return err
	}
	return nil
}

func (r *AzureAppReconciler) ManageFinalizer(ctx context.Context, azapp k8sappv0alpha1.AzureApp) error {
	logr := logr.FromContextOrDiscard(ctx)
	finalizer := "DestroyAzureResources"
	r.SetupFinalizer(finalizer, &azapp)
	if !azapp.ObjectMeta.DeletionTimestamp.IsZero() {
		logr.Info("Removing Azure Resources")
		if err := r.kubeclient.SetProvisionState("Removing Azure resources", &azapp); err != nil {
			return err
		}
		if err := dependencies.ManageTerraformableExternalDependencies(&azapp, "destroy"); err != nil {
			return err
		}
		if err := r.RemoveFinalizer(finalizer, &azapp); err != nil {
			return err
		}
		logr.Info(fmt.Sprintf("Done deleting Azure Resources for app: %s", azapp.Name))
	}
	return nil
}

func getPreviousSpec(azapp *k8sappv0alpha1.AzureApp) (*k8sappv0alpha1.AzureAppSpec, error) {
	azappSpecFile := fmt.Sprintf("%s/%s/spec.auto.tfvars.json", os.Getenv("TF_BASE_PATH"), azapp.Name)
	content, err := ioutil.ReadFile(azappSpecFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotExist
		}
		return nil, err
	}

	var previousSpec k8sappv0alpha1.AzureAppSpec
	err = json.Unmarshal(content, &previousSpec)
	if err != nil {
		return nil, err
	}

	return &previousSpec, nil
}

func shouldReconcile(azapp *k8sappv0alpha1.AzureApp) (bool, error) {
	previousSpec, err := getPreviousSpec(azapp)
	if err != nil {
		if err == ErrFileNotExist {
			return true, nil
		}
		return false, err
	}

	previousSpecValue := *previousSpec
	if reflect.DeepEqual(previousSpecValue, azapp.Spec) {
		return false, nil
	}
	return true, nil
}

func ignoreConflict(ctx context.Context, err error) error {
	logr := logr.FromContextOrDiscard(ctx)
	if k8serr.IsConflict(err) {
		logr.Info("Ignoring conflict error")
		return nil
	}
	return err
}
