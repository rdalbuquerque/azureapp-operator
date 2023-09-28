package az

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azcertificates"
	"github.com/rdalbuquerque/azure-operator/operator/controllers/config"
)

type AzClient struct {
	cred *azidentity.ClientSecretCredential
}

func NewAzureClient() (*AzClient, error) {
	azcred, err := azidentity.NewClientSecretCredential(config.Config.ARMTenantID, config.Config.ARMClientID, config.Config.ARMClientSecret, nil)
	if err != nil {
		return nil, err
	}
	return &AzClient{cred: azcred}, nil
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
