package dependencies

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/terraform-exec/tfexec"
	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/az"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/db"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/tf"
)

type TfDependenciesClient struct {
	tfc *tf.TfClient
}

func NewTerraformClient(azapp *k8sappv0alpha1.AzureApp) (*TfDependenciesClient, error) {
	tf, err := tf.NewTerraformClient(os.Getenv("TF_EXECUTABLE_PATH"), os.Getenv("TF_BASE_PATH"), azapp)
	return &TfDependenciesClient{tfc: tf}, err
}

func (tfd *TfDependenciesClient) CheckTerraformableExternalDependencies(ctx context.Context, azapp *k8sappv0alpha1.AzureApp) (string, bool, error) {
	logr := logr.FromContextOrDiscard(ctx)
	if err := tfd.tfc.GenerateTerraformVarFile(azapp); err != nil {
		return "", false, err
	}
	planfile := fmt.Sprintf("plan-%s", azapp.Name)
	outOption := tfexec.Out(planfile)
	logr.Info(fmt.Sprintf("Initiating terraform plan of app [%s]", azapp.Name))
	start := time.Now()
	changed, err := tfd.tfc.Plan(context.TODO(), outOption)
	elapsed := time.Since(start)
	logr.Info(fmt.Sprintf("Done terraform plan of app [%s], plan duration: %v", azapp.Name, elapsed))
	return planfile, changed, err
}

func (tfd *TfDependenciesClient) ManageTerraformableExternalDependencies(ctx context.Context, azapp *k8sappv0alpha1.AzureApp, phase string, planfile string) error {
	logr := logr.FromContextOrDiscard(ctx)
	switch phase {
	case "apply":
		logr.Info(fmt.Sprintf("Initiating terraform apply of app [%s]", azapp.Name))
		return tfd.tfc.ReconcileAzureResources(planfile)
	case "destroy":
		return tfd.tfc.DestroyAzureResources(ctx, azapp)
	default:
		return errors.New("invalid phase")
	}
}

func GetTerraformAppCredentialOutput(azapp *k8sappv0alpha1.AzureApp) (map[string]string, error) {
	//refactor so I don't have to instantiate the client again here
	tf, err := tf.NewTerraformClient(os.Getenv("TF_EXECUTABLE_PATH"), os.Getenv("TF_BASE_PATH"), azapp)
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
			os.Getenv("ARM_CLIENT_ID"),
			os.Getenv("ARM_CLIENT_SECRET"),
			"prdazureappoperatorsv1",
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
	return azclient.SslCertificateExists(fmt.Sprintf("%s-kv", azapp.Spec.Identifier))
}
