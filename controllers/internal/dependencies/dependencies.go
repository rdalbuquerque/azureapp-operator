package dependencies

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/terraform-exec/tfexec"
	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/config"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/az"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/db"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/tf"
)

type TfDependenciesClient struct {
	tfc *tf.TfClient
}

func NewTerraformClient(ctx context.Context, azapp *k8sappv0alpha1.AzureApp) (*TfDependenciesClient, error) {
	tf, err := tf.NewTerraformClient(ctx, config.Config.TerraformExecutablePath, config.Config.TerraformBasePath, azapp)
	return &TfDependenciesClient{tfc: tf}, err
}

func (tfd *TfDependenciesClient) CheckTerraformableExternalDependencies(ctx context.Context, azapp *k8sappv0alpha1.AzureApp) (string, bool, error) {
	logr := logr.FromContextOrDiscard(ctx)
	planfile := fmt.Sprintf("plan-%s", azapp.Name)
	outOption := tfexec.Out(planfile)
	logr.Info(fmt.Sprintf("Initiating terraform plan of app [%s]", azapp.Name))
	start := time.Now()
	parallelism := tfexec.Parallelism(1)
	changed, err := tfd.tfc.Plan(context.TODO(), outOption, parallelism)
	elapsed := time.Since(start)
	logr.Info(fmt.Sprintf("[%s] plan duration: %v", azapp.Name, elapsed))
	return planfile, changed, err
}

func (tfd *TfDependenciesClient) ManageTerraformableExternalDependencies(ctx context.Context, azapp *k8sappv0alpha1.AzureApp, phase string, planfile string) error {
	logr := logr.FromContextOrDiscard(ctx)
	var err error
	start := time.Now()
	switch phase {
	case "apply":
		logr.Info(fmt.Sprintf("Initiating terraform apply of app [%s]", azapp.Name))
		err = tfd.tfc.ReconcileAzureResources(planfile)
	case "destroy":
		err = tfd.tfc.DestroyAzureResources(ctx, azapp)
	default:
		return errors.New("invalid phase")
	}
	elapsed := time.Since(start)
	logr.Info(fmt.Sprintf("[%s] %s duration: %v", azapp.Name, phase, elapsed))
	return err
}

func GetTerraformAppCredentialOutput(ctx context.Context, azapp *k8sappv0alpha1.AzureApp) (map[string]string, error) {
	//refactor so I don't have to instantiate the client again here
	tf, err := tf.NewTerraformClient(ctx, config.Config.TerraformExecutablePath, config.Config.TerraformBasePath, azapp)
	if err != nil {
		return nil, err
	}

	return tf.GetAzureAppCredential()
}

func ManageOtherExternalDependencies(azapp *k8sappv0alpha1.AzureApp) error {
	// currently, for this project, database user is the only external dependency not manageable by terraform
	// Setup DB User
	if azapp.Spec.EnableDatabase {
		sqlclient, err := db.NewServicePrincipalClient(
			config.Config.ARMClientID,
			config.Config.ARMClientSecret,
			config.Config.DefaultSQLServer,
			fmt.Sprintf("%s-db", azapp.Spec.Identifier),
			context.TODO(),
		)
		if err != nil {
			return err
		}
		username := fmt.Sprintf("%s-app", azapp.Spec.Identifier)
		if err := sqlclient.CreateUser(username); err != nil {
			return err
		}
		if err := sqlclient.GrantOwner(username); err != nil {
			return err
		}
	}
	return nil
}

func CheckCertificate(azapp *k8sappv0alpha1.AzureApp) (bool, error) {
	azclient, err := az.NewAzureClient()
	if err != nil {
		return false, err
	}
	return azclient.TlsCertificateExists(fmt.Sprintf("%s-kv", azapp.Spec.Identifier))
}
