package az

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	k8sappv1alpha1 "github.com/rdalbuquerque/azure-operator/operator/api/v0alpha1"
)

type AzClient struct {
	cred *azidentity.ClientSecretCredential
}

func NewAzureClient() (*AzClient, error) {
	azcred, err := azidentity.NewClientSecretCredential(os.Getenv("ARM_TENANT_ID"), os.Getenv("ARM_CLIENT_ID"), os.Getenv("ARM_CLIENT_SECRET"), nil)
	if err != nil {
		return nil, err
	}
	return &AzClient{cred: azcred}, nil
}

func (az *AzClient) DeleteStateFile(azapp *k8sappv1alpha1.AzureApp) error {
	bbClient, _ := blockblob.NewClient(fmt.Sprintf("https://demooperator.blob.core.windows.net/demo-operator/k8sapp.%s.json", azapp.Name), az.cred, nil)
	if _, err := bbClient.Delete(context.Background(), nil); err != nil {
		return err
	}
	return nil
}

func (az *AzClient) SslCertificateExists(azkeyvault string) (bool, error) {
	kvUrl := fmt.Sprintf("https://%s.vault.azure.net/", azkeyvault)
	certClient, err := azcertificates.NewClient(kvUrl, az.cred, nil)
	if err != nil {
		return false, err
	}
	getResp, err := certClient.GetCertificate(context.TODO(), "ssl", "", nil)
	return getResp.ID != nil, IgnoreNotFound(err)
}

func IgnoreNotFound(err error) error {
	var responseError *azcore.ResponseError
	if errors.As(err, &responseError) {
		if err.(*azcore.ResponseError).ErrorCode == "CertificateNotFound" {
			return nil
		}
	}
	return err
}
