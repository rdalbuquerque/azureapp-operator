package controllers

import (
	"context"

	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

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
