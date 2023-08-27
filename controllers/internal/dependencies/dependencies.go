package dependencies

import (
	"context"
	"errors"
	"fmt"
	"os"

	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/db"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/internal/tf"
)

// Define your custom type
type TerraformPhase string

// Define the accepted strings as constants
const (
	Apply   TerraformPhase = "apply"
	Destroy TerraformPhase = "destroy"
)

func ManageTerraformableExternalDependencies(azapp *k8sappv0alpha1.AzureApp, phase TerraformPhase) error {
	tf, err := tf.NewTerraformClient(os.Getenv("TF_EXECUTABLE_PATH"), os.Getenv("TF_BASE_PATH"), azapp)
	if err != nil {
		return err
	}

	if err := tf.GenerateTerraformVarFile(azapp); err != nil {
		return err
	}

	if err := tf.InitTerraform(); err != nil {
		return err
	}

	switch phase {
	case Apply:
		return tf.ReconcileAzureResources()
	case Destroy:
		return tf.DestroyAzureResources(azapp)
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
