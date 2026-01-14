package azure_data_lake

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake"
	azfilesystem "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/filesystem"
	"github.com/thanos-io/objstore/exthttp"
)

// DirDelim is the delimiter used to model a directory structure in an object store bucket.
const DirDelim = "/"

func getDataLakeGen2FilesystemClient(conf Config, wrapRoundtripper func(http.RoundTripper) http.RoundTripper) (*azfilesystem.Client, error) {
	var rt http.RoundTripper
	rt, err := exthttp.DefaultTransport(conf.HTTPConfig)
	if err != nil {
		return nil, err
	}
	if conf.HTTPConfig.Transport != nil {
		rt = conf.HTTPConfig.Transport
	}
	if wrapRoundtripper != nil {
		rt = wrapRoundtripper(rt)
	}

	opt := &azfilesystem.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    conf.PipelineConfig.MaxTries,
				TryTimeout:    time.Duration(conf.PipelineConfig.TryTimeout),
				RetryDelay:    time.Duration(conf.PipelineConfig.RetryDelay),
				MaxRetryDelay: time.Duration(conf.PipelineConfig.MaxRetryDelay),
			},
			Telemetry: policy.TelemetryOptions{
				ApplicationID: "Thanos",
			},
			Transport: &http.Client{Transport: rt},
		},
	}

	fileSystemURL := fmt.Sprintf("https://%s.dfs.core.windows.net/%s", conf.StorageAccountName, conf.ContainerName)

	if conf.StorageConnectionString != "" {
		return azfilesystem.NewClientFromConnectionString(conf.StorageConnectionString, conf.ContainerName, opt)
	}

	if conf.StorageAccountKey != "" {
		creds, err := azdatalake.NewSharedKeyCredential(conf.StorageAccountName, conf.StorageAccountKey)
		if err != nil {
			return nil, err
		}

		return azfilesystem.NewClientWithSharedKeyCredential(fileSystemURL, creds, opt)
	}

	cred, err := getTokenCredential(conf)
	if err != nil {
		return nil, err
	}

	return azfilesystem.NewClient(fileSystemURL, cred, opt)
}

func getTokenCredential(conf Config) (azcore.TokenCredential, error) {
	if conf.ClientSecret != "" && conf.AzTenantID != "" && conf.ClientID != "" {
		return azidentity.NewClientSecretCredential(conf.AzTenantID, conf.ClientID, conf.ClientSecret, &azidentity.ClientSecretCredentialOptions{})
	}

	if conf.UserAssignedID == "" {
		return azidentity.NewDefaultAzureCredential(nil)
	}

	msiOpt := &azidentity.ManagedIdentityCredentialOptions{}
	msiOpt.ID = azidentity.ClientID(conf.UserAssignedID)
	return azidentity.NewManagedIdentityCredential(msiOpt)
}
