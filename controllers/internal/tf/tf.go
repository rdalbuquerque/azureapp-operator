package tf

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/hashicorp/terraform-exec/tfexec"
	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/config"
)

type TfClient struct {
	*tfexec.Terraform
}

func NewTerraformClient(ctx context.Context, tfExePath, tfBaseDir string, azapp *k8sappv0alpha1.AzureApp) (*TfClient, error) {
	workdir := fmt.Sprintf("%s/%s", tfBaseDir, azapp.Name)
	if err := os.Mkdir(workdir, os.FileMode(0666)); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
	}
	if err := os.Chmod(workdir, os.FileMode(0777)); err != nil {
		return nil, err
	}
	tf, err := tfexec.NewTerraform(workdir, tfExePath)
	if err != nil {
		return nil, err
	}
	if err := renderTerraformMain(azapp, tfBaseDir); err != nil {
		return nil, err
	}
	if err := tf.Init(ctx); err != nil {
		return nil, err
	}
	return &TfClient{
		Terraform: tf,
	}, nil
}

type tfBackendInfo struct {
	ResourceGroup  string
	StorageAccount string
	Container      string
	Key            string
}

func renderTerraformMain(azapp *k8sappv0alpha1.AzureApp, tfDir string) error {
	backendInfo := tfBackendInfo{}
	backendInfo.ResourceGroup = config.Config.TerraformBackendResourceGroup
	backendInfo.StorageAccount = config.Config.TerraformBackendStorageAccount
	backendInfo.Container = config.Config.TerraformBackendContainer
	backendInfo.Key = azapp.Name

	tmplFile := fmt.Sprintf("%s/main.tf.gotmpl", tfDir)
	maintf, _ := os.Create(fmt.Sprintf("%s/%s/main.tf", tfDir, azapp.Name))
	tmplName := path.Base(tmplFile)
	tmpl, err := template.New(tmplName).ParseFiles(tmplFile)
	if err != nil {
		return err
	}
	return tmpl.Execute(maintf, backendInfo)
}

func (tf *TfClient) GenerateTerraformVarFile(azapp *k8sappv0alpha1.AzureApp) error {
	tfvarFileName := fmt.Sprintf("%s/spec.auto.tfvars.json", tf.WorkingDir())
	jsonspec, err := json.Marshal(azapp.Spec)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(tfvarFileName, jsonspec, 0666)
	if err != nil {
		return err
	}

	return nil
}

func (tf *TfClient) GetAzureAppCredential() (map[string]string, error) {
	appCreds := make(map[string]string)
	output, err := tf.Output(context.Background())
	if err != nil {
		return nil, err
	}
	appCreds["appId"] = string(output["app_id"].Value)
	appCreds["appSecret"] = string(output["app_secret"].Value)
	return appCreds, nil
}

func (tf *TfClient) ReconcileAzureResources(planfile string) error {
	if err := os.Chdir(tf.WorkingDir()); err != nil {
		return err
	}
	return tf.Apply(context.Background())
}

func (tf *TfClient) DestroyAzureResources(ctx context.Context, azapp *k8sappv0alpha1.AzureApp) error {
	if err := os.Chdir(tf.WorkingDir()); err != nil {
		return err
	}
	if err := tf.Destroy(context.TODO()); err == nil {
		if err := tf.deleteStateFile(ctx, azapp); err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func (tf *TfClient) deleteStateFile(ctx context.Context, azapp *k8sappv0alpha1.AzureApp) error {
	azcred, err := azidentity.NewClientSecretCredential(config.Config.ARMTenantID, config.Config.ARMClientID, config.Config.ARMClientSecret, nil)
	if err != nil {
		return err
	}
	bbClient, err := blockblob.NewClient(fmt.Sprintf("https://prdazureappoperator.blob.core.windows.net/state/k8sapp.%s.json", azapp.Name), azcred, nil)
	if err != nil {
		return err
	}
	if _, err := bbClient.Delete(context.TODO(), nil); err != nil {
		return err
	}
	return nil
}
