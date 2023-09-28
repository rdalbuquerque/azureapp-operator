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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/dependencies"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/kubeobjects"
)

// AzureAppReconciler reconciles a AzureApp object
type AzureAppReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	BaseDir    string
	kubeclient *kubeobjects.KubeClient
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

// this function should have the following phases:
// 1- manage external dependencies
//
//	1- manage terraform managed dependencies
//	2- manage other external dependencies
//
// 2- once external dependencies are good to go, manage kubernetes objects
func (r *AzureAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logr := logr.FromContextOrDiscard(ctx)
	logr.Info("Initializing reconcile loop")

	// map azure app being reconciled into azapp object
	azapp := k8sappv0alpha1.AzureApp{}
	if err := r.Get(ctx, req.NamespacedName, &azapp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// initiates clients
	r.kubeclient = kubeobjects.NewKubeClient(ctx, r.Client, applyOpts)
	tfclient, err := dependencies.NewTerraformClient(ctx, &azapp)
	if err != nil {
		logr.Info("error initiating terraform client")
		return ctrl.Result{}, err
	}

	// setup finalizer and evaluate DeletionTimestamp, if it's not zero, executes cleanup and removes finalizer
	if objdeleted, err := r.ManageFinalizer(ctx, azapp, tfclient); err != nil {
		return ctrl.Result{}, err
	} else if objdeleted {
		return ctrl.Result{}, nil
	}

	// reconcile external dependencies
	planfile, tfchanged, err := tfclient.CheckTerraformableExternalDependencies(ctx, &azapp)
	if err != nil {
		return ctrl.Result{}, err
	}
	if tfchanged {
		logr.Info("Reconciling AzureApp")
		if err := r.kubeclient.SetProvisionState("Reconciling external dependencies", &azapp); err != nil {
			return ctrl.Result{}, ignoreConflict(ctx, err)
		}
		if err := tfclient.ManageTerraformableExternalDependencies(ctx, &azapp, "apply", planfile); err != nil {
			return ctrl.Result{}, fmt.Errorf("error managing terraform dependencies: %s", err)
		}
		if err := dependencies.ManageOtherExternalDependencies(&azapp); err != nil {
			return ctrl.Result{}, fmt.Errorf("error managing other dependencies: %s", err)
		}

	}

	logr.Info("Checking certificate")
	ok, err := dependencies.CheckCertificate(&azapp)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ok {
		err := r.kubeclient.SetProvisionState("Waiting certificate", &azapp)
		return ctrl.Result{RequeueAfter: time.Duration(30) * time.Second}, ignoreConflict(ctx, err)
	}

	// reconcile kubernetes objects
	azappk8s, err := r.buildKubeObjects(ctx, azapp)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.kubeclient.ApplyAll(azappk8s); err != nil {
		return ctrl.Result{}, ignoreConflict(ctx, err)
	}
	if err := r.kubeclient.SetDeploymentName(azapp.Spec.Identifier, &azapp); err != nil {
		return ctrl.Result{}, ignoreConflict(ctx, err)
	}
	if err := r.kubeclient.SetProvisionState("Provisioned", &azapp); err != nil {
		return ctrl.Result{}, ignoreConflict(ctx, err)
	}
	logr.Info(fmt.Sprintf("Successfully reconciled AzureApp: %s", azapp.Name))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sappv0alpha1.AzureApp{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 3,
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				if req == nil {
					return log.Log
				}
				return log.Log.WithName(req.Name).WithValues("namespace", req.Namespace)
			}}).
		Complete(r)
}
