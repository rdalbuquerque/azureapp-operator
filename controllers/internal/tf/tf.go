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
	"github.com/go-logr/logr"
	"github.com/hashicorp/terraform-exec/tfexec"
	k8sappv0alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
)

type TfClient struct {
	*tfexec.Terraform
	logr.Logger
}

func NewTerraformClient(tfExePath, tfBaseDir string, azapp *k8sappv0alpha1.AzureApp, logr logr.Logger) (*TfClient, error) {
	workdir := fmt.Sprintf("%s/%s", tfBaseDir, azapp.Name)
	os.Mkdir(workdir, os.FileMode(0666))
	tf, err := tfexec.NewTerraform(workdir, tfExePath)
	if err := renderTerraformMain(azapp, tfBaseDir); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &TfClient{
		Terraform: tf,
		Logger:    logr.WithValues("phase", "terraform"),
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
	backendInfo.ResourceGroup = "tf-remote"
	backendInfo.StorageAccount = "rdaremotestate1"
	backendInfo.Container = "state"
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

func (tf *TfClient) InitTerraform() error {
	tf.Logger.Info("Initiating terraform", "workdir", tf.WorkingDir())
	return tf.Init(context.Background())
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

func (tf *TfClient) CheckForAzureChanges() (*bool, error) {
	if err := os.Chdir(tf.WorkingDir()); err != nil {
		return nil, err
	}
	var err error
	changed := new(bool)
	*changed, err = tf.Plan(context.Background())
	if err != nil {
		return nil, err
	}
	return changed, nil
}

func (tf *TfClient) ReconcileAzureResources() error {
	if err := os.Chdir(tf.WorkingDir()); err != nil {
		return err
	}
	return tf.Apply(context.Background())
}

func (tf *TfClient) DestroyAzureResources(azapp *k8sappv0alpha1.AzureApp) error {
	if err := os.Chdir(tf.WorkingDir()); err != nil {
		return err
	}
	if err := tf.Destroy(context.Background()); err == nil {
		if err := tf.deleteStateFile(azapp); err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func (tf *TfClient) deleteStateFile(azapp *k8sappv0alpha1.AzureApp) error {
	azcred, _ := azidentity.NewClientSecretCredential(os.Getenv("ARM_TENANT_ID"), os.Getenv("ARM_CLIENT_ID"), os.Getenv("ARM_CLIENT_SECRET"), nil)
	bbClient, _ := blockblob.NewClient(fmt.Sprintf("https://rdaremotestate1.blob.core.windows.net/state/k8sapp.%s.json", azapp.Name), azcred, nil)
	if _, err := bbClient.Delete(context.Background(), nil); err != nil {
		return err
	}
	return nil
}
