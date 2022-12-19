/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/db"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/kubeobjects"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/tf"
	appsv1 "k8s.io/api/apps/v1"
)

// AzureAppReconciler reconciles a AzureApp object
type AzureAppReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	TfExecPath string
	BaseDir    string
}

var applyOpts = []client.PatchOption{client.ForceOwnership, client.FieldOwner("azureapp-controller")}

//+kubebuilder:rbac:groups=k8sapp.rdalbuquerque.dev,resources=azureapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8sapp.rdalbuquerque.dev,resources=azureapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8sapp.rdalbuquerque.dev,resources=azureapps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AzureApp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *AzureAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var l logr.Logger
	k := kubeobjects.NewKubeClient(r.Client, ctx, applyOpts)
	azapp := k8sappv0alpha1.AzureApp{}
	if err := r.Get(ctx, req.NamespacedName, &azapp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	l = log.Log.WithName(azapp.Name)
	l.Info("Reconciling AzureApp")
	if azapp.Status.ProvisioningState == "" {
		if err := k.SetProvisionState("Initiating Azure Workdir", &azapp); err != nil {
			return ctrl.Result{}, err
		}
	}
	l.Info("Setting up and initializing azure workdir")
	tf, err := tf.NewTerraformClient(r.TfExecPath, fmt.Sprintf("%s/terraform", r.BaseDir), &azapp, l)
	if err != nil {
		l.Error(err, "Unable to set Azure workdir for app")
		return ctrl.Result{}, err
	}
	if err := tf.GenerateTerraformVarFile(&azapp); err != nil {
		l.Error(err, "Unbable to generate terraform var file for app")
		return ctrl.Result{}, err
	}

	finalizer := "DestroyAzure"
	r.SetupFinalizer(finalizer, &azapp)
	if !azapp.ObjectMeta.DeletionTimestamp.IsZero() {
		l.Info("Removing Azure Resources")
		if err := k.SetProvisionState("Removing Azure Resources", &azapp); err != nil {
			return ctrl.Result{}, err
		}
		tf.DestroyAzureResources(&azapp)
		if err := r.RemoveFinalizer(finalizer, &azapp); err != nil {
			return ctrl.Result{}, err
		}
		l.Info("Done deleting Azure Resources")
		return ctrl.Result{}, nil
	}

	if err := tf.InitTerraform(); err != nil {
		l.Error(err, "Unable to initialize workdir for app")
		return ctrl.Result{}, err
	}

	hasAzureChanged, err := tf.CheckForAzureChanges()
	if err != nil {
		return ctrl.Result{}, err
	}

	if *hasAzureChanged {
		l.Info("Azure changes identified, applying them")
		if err := k.SetProvisionState("Provisioning Azure Resources", &azapp); err != nil {
			return ctrl.Result{}, err
		}
		err := tf.ReconcileAzureResources()
		if err != nil {
			return ctrl.Result{}, err
		}
		l.Info("Successfully applied Azure changes")
	} else {
		l.Info("No changes to Azure, moving on")
	}

	// Setup DB User
	if azapp.Spec.EnableDatabase {
		if azapp.Status.ProvisioningState == "Provisioning Azure Resources" {
			if err := k.SetProvisionState("Configuring DB User", &azapp); err != nil {
				return ctrl.Result{}, err
			}
		}
		sqlclient, err := db.NewServicePrincipalClient(
			os.Getenv("ARM_CLIENT_ID"),
			os.Getenv("ARM_CLIENT_SECRET"),
			"rdatestsv1",
			fmt.Sprintf("%s-db", azapp.Spec.Identifier),
			ctx,
		)
		if err != nil {
			l.Error(err, "Unable to instantiate sql client")
			return ctrl.Result{}, err
		}
		username := fmt.Sprintf("%s-app", azapp.Spec.Identifier)
		if err := sqlclient.CreateUser(username); err != nil {
			l.Error(err, "Unable to create user", "username", username)
			return ctrl.Result{}, err
		}
		if err := sqlclient.GrantOwner(username); err != nil {
			l.Error(err, "Unable to grant owner to user", "username", username)
			return ctrl.Result{}, err
		}
	}

	// Setup kubeobjects
	azappk8s := kubeobjects.AzAppKubeObjects
	appCredential, err := tf.GetAzureAppCredential()
	if err != nil {
		return ctrl.Result{}, err
	}
	if secret, err := r.desiredSecret(appCredential, &azapp); err != nil {
		return ctrl.Result{}, err
	} else {
		azappk8s = append(azappk8s, &secret)
		if deployment, err := r.desiredDeployment(&azapp, secret); err != nil {
			return ctrl.Result{}, err
		} else {
			azappk8s = append(azappk8s, &deployment)
		}
	}
	if service, err := r.desiredService(&azapp); err != nil {
		return ctrl.Result{}, err
	} else {
		azappk8s = append(azappk8s, &service)
	}
	if ingress, err := r.desiredIngress(&azapp); err != nil {
		return ctrl.Result{}, err
	} else {
		azappk8s = append(azappk8s, &ingress)
	}
	k.ApplyAll(azappk8s)

	azapp.Status.Deployment = azapp.Spec.Identifier

	if azapp.Status.ProvisioningState != "Provisioned" {
		if err := k.SetProvisionState("Provisioned", &azapp); err != nil {
			return ctrl.Result{}, err
		}
	}
	l.Info("Successfully reconciled AzureApp")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sappv0alpha1.AzureApp{}).
		Owns(&appsv1.Deployment{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(r)
}
