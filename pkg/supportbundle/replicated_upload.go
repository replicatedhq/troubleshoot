package supportbundle

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultReplicatedAppEndpoint = "https://replicated.app"
	// replicatedSDKChartLabelPrefix is the prefix for the helm.sh/chart label on SDK secrets.
	// The full label value is "replicated-{VERSION}" (e.g., "replicated-1.18.2").
	replicatedSDKChartLabelPrefix = "replicated-"
	// integrationLicenseIDKey is the secret key for the license ID set by the SDK chart.
	integrationLicenseIDKey = "integration-license-id"
	// replicatedConfigKey is the secret key for the full SDK config YAML.
	replicatedConfigKey   = "config.yaml"
	s3UploadTimeout       = 30 * time.Minute
	apiRequestTimeout     = 30 * time.Second
	maxErrorResponseBytes = 1024 * 1024 // 1 MB
)

// ReplicatedConfig represents the config.yaml stored in the SDK secret.
// The license field can be either a YAML map (nested object) or a YAML string,
// so we use interface{} and re-marshal to extract the license ID.
type ReplicatedConfig struct {
	License               interface{} `json:"license" yaml:"license"`
	ChannelID             string      `json:"channelID" yaml:"channelID"`
	ReplicatedAppEndpoint string      `json:"replicatedAppEndpoint" yaml:"replicatedAppEndpoint"`
}

// replicatedLicenseSpec is the inner structure containing the license ID.
type replicatedLicenseSpec struct {
	Spec struct {
		LicenseID string `json:"licenseID" yaml:"licenseID"`
	} `json:"spec" yaml:"spec"`
}

// ReplicatedUploadCredentials holds the values discovered from the in-cluster SDK secret.
type ReplicatedUploadCredentials struct {
	LicenseID string
	ChannelID string
	Endpoint  string
}

// supportBundleUploadURLResponse is the response from the presigned URL endpoint.
type supportBundleUploadURLResponse struct {
	BundleID  string `json:"bundle_id"`
	UploadURL string `json:"upload_url"`
}

// markUploadedRequest is the payload sent to mark an upload as complete.
type markUploadedRequest struct {
	ChannelID string `json:"channel_id"`
}

// markUploadedResponse is the response after marking a bundle as uploaded.
type markUploadedResponse struct {
	Slug string `json:"slug"`
}

// DiscoverReplicatedCredentials discovers the Replicated SDK secret and extracts
// the license ID, channel ID, and endpoint needed for upload.
//
// Discovery strategy:
//  1. If secretName is provided, look up that specific secret.
//  2. Otherwise, discover the SDK secret by listing secrets with the label
//     "helm.sh/chart" prefixed with "replicated-" (the SDK Helm chart).
//
// The secret name follows the SDK chart naming convention: {APP_NAME}-sdk
// (e.g., "k8laude-sdk" for an app named "k8laude").
//
// License ID extraction:
//  1. Try the "integration-license-id" key first (direct string value).
//  2. Fall back to parsing "config.yaml" and extracting license.spec.licenseID.
//
// Requires RBAC: the service account must have `get` and `list` permissions
// on `secrets` in the target namespace.
func DiscoverReplicatedCredentials(ctx context.Context, restConfig *rest.Config, namespace string, secretName string) (*ReplicatedUploadCredentials, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "create kubernetes clientset")
	}

	var secretData map[string][]byte
	var resolvedName string

	if secretName != "" {
		// Explicit secret name provided — look it up directly
		secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "get secret %s/%s", namespace, secretName)
		}
		secretData = secret.Data
		resolvedName = secretName
	} else {
		// Discover the SDK secret by label
		secrets, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=Helm",
		})
		if err != nil {
			return nil, errors.Wrapf(err, "list secrets in namespace %s", namespace)
		}

		for _, s := range secrets.Items {
			chartLabel := s.Labels["helm.sh/chart"]
			if strings.HasPrefix(chartLabel, replicatedSDKChartLabelPrefix) {
				secretData = s.Data
				resolvedName = s.Name
				break
			}
		}

		if secretData == nil {
			return nil, fmt.Errorf("no Replicated SDK secret found in namespace %q (looked for helm.sh/chart label with prefix %q). "+
				"If the SDK is installed in a different namespace, use --sdk-namespace to specify it", namespace, replicatedSDKChartLabelPrefix)
		}
	}

	// Extract license ID: try integration-license-id key first, fall back to config.yaml
	licenseID, err := extractLicenseID(secretData, resolvedName, namespace)
	if err != nil {
		return nil, err
	}

	// Extract channel ID and endpoint from config.yaml
	channelID, endpoint, err := extractConfigFields(secretData)
	if err != nil {
		return nil, err
	}

	if endpoint == "" {
		endpoint = defaultReplicatedAppEndpoint
	}

	if err := validateEndpoint(endpoint); err != nil {
		return nil, errors.Wrap(err, "invalid replicated app endpoint")
	}

	return &ReplicatedUploadCredentials{
		LicenseID: licenseID,
		ChannelID: channelID,
		Endpoint:  strings.TrimRight(endpoint, "/"),
	}, nil
}

// SDKSecretMatch represents a discovered Replicated SDK secret with its
// resolved credentials and location metadata for display purposes.
type SDKSecretMatch struct {
	SecretName string
	Namespace  string
	Creds      *ReplicatedUploadCredentials
}

// FindAllSDKCredentials searches all namespaces for Replicated SDK secrets
// and returns all valid matches. This handles the case where the SDK is
// installed in a namespace different from where troubleshoot is running,
// or where multiple SDK installations exist.
//
// Requires RBAC: the service account must have `list` on secrets cluster-wide.
func FindAllSDKCredentials(ctx context.Context, restConfig *rest.Config) ([]SDKSecretMatch, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "create kubernetes clientset")
	}

	secrets, err := clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=Helm",
	})
	if err != nil {
		return nil, errors.Wrap(err, "list secrets across all namespaces")
	}

	var matches []SDKSecretMatch
	for _, s := range secrets.Items {
		chartLabel := s.Labels["helm.sh/chart"]
		if !strings.HasPrefix(chartLabel, replicatedSDKChartLabelPrefix) {
			continue
		}

		licenseID, err := extractLicenseID(s.Data, s.Name, s.Namespace)
		if err != nil {
			continue
		}

		channelID, endpoint, err := extractConfigFields(s.Data)
		if err != nil {
			continue
		}

		if endpoint == "" {
			endpoint = defaultReplicatedAppEndpoint
		}

		if err := validateEndpoint(endpoint); err != nil {
			continue
		}

		matches = append(matches, SDKSecretMatch{
			SecretName: s.Name,
			Namespace:  s.Namespace,
			Creds: &ReplicatedUploadCredentials{
				LicenseID: licenseID,
				ChannelID: channelID,
				Endpoint:  strings.TrimRight(endpoint, "/"),
			},
		})
	}

	return matches, nil
}

// extractLicenseID tries the integration-license-id key first, then falls back
// to parsing config.yaml for the license ID.
func extractLicenseID(data map[string][]byte, secretName, namespace string) (string, error) {
	// Try the direct integration key first
	if licenseBytes, ok := data[integrationLicenseIDKey]; ok {
		licenseID := strings.TrimSpace(string(licenseBytes))
		if licenseID != "" {
			return licenseID, nil
		}
	}

	// Fall back to config.yaml
	configData, ok := data[replicatedConfigKey]
	if !ok {
		return "", fmt.Errorf("secret %s/%s contains neither %q nor %q key",
			namespace, secretName, integrationLicenseIDKey, replicatedConfigKey)
	}

	var config ReplicatedConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return "", errors.Wrap(err, "unmarshal replicated config")
	}

	if config.License == nil {
		return "", fmt.Errorf("replicated config in secret %s/%s does not contain a license", namespace, secretName)
	}

	// The license field may be a YAML map or string. Re-marshal to YAML bytes
	// and parse into our spec struct to extract the license ID.
	licenseBytes, err := yaml.Marshal(config.License)
	if err != nil {
		return "", errors.Wrapf(err, "marshal license field in secret %s/%s", namespace, secretName)
	}

	var spec replicatedLicenseSpec
	if err := yaml.Unmarshal(licenseBytes, &spec); err != nil {
		return "", errors.Wrapf(err, "unmarshal license spec in secret %s/%s", namespace, secretName)
	}

	licenseID := spec.Spec.LicenseID
	if licenseID == "" {
		return "", fmt.Errorf("license ID is empty in secret %s/%s", namespace, secretName)
	}

	return licenseID, nil
}

// extractConfigFields parses config.yaml to get channelID and endpoint.
// Returns empty strings (not errors) if config.yaml is missing, since these
// fields have defaults.
func extractConfigFields(data map[string][]byte) (channelID string, endpoint string, err error) {
	configData, ok := data[replicatedConfigKey]
	if !ok {
		return "", "", nil
	}

	var config ReplicatedConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return "", "", errors.Wrap(err, "unmarshal replicated config for channel/endpoint")
	}

	return config.ChannelID, config.ReplicatedAppEndpoint, nil
}

// GetPresignedUploadURL calls the Replicated API to obtain a presigned S3 URL for upload.
func GetPresignedUploadURL(creds *ReplicatedUploadCredentials) (*supportBundleUploadURLResponse, error) {
	reqURL := fmt.Sprintf("%s/v3/supportbundle/upload-url", creds.Endpoint)

	req, err := http.NewRequest("POST", reqURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "create presigned URL request")
	}

	setBasicAuth(req, creds.LicenseID)

	client := &http.Client{Timeout: apiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request presigned upload URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseBytes))
		return nil, fmt.Errorf("failed to get presigned URL: status %d: %s", resp.StatusCode, string(body))
	}

	var uploadURLResp supportBundleUploadURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadURLResp); err != nil {
		return nil, errors.Wrap(err, "decode presigned URL response")
	}

	if uploadURLResp.UploadURL == "" {
		return nil, fmt.Errorf("presigned URL response did not contain an upload URL")
	}

	if err := validatePresignedURL(uploadURLResp.UploadURL); err != nil {
		return nil, errors.Wrap(err, "invalid presigned upload URL")
	}

	return &uploadURLResp, nil
}

// UploadToS3 uploads the support bundle archive to S3 using the presigned URL.
func UploadToS3(presignedURL string, archivePath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return errors.Wrap(err, "open archive for upload")
	}
	defer f.Close()

	fileStat, err := f.Stat()
	if err != nil {
		return errors.Wrap(err, "stat archive file")
	}

	req, err := http.NewRequest("PUT", presignedURL, f)
	if err != nil {
		return errors.Wrap(err, "create S3 upload request")
	}
	req.ContentLength = fileStat.Size()
	req.Header.Set("Content-Type", "application/tar+gzip")

	client := &http.Client{Timeout: s3UploadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "upload to S3")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseBytes))
		return fmt.Errorf("S3 upload failed: status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// MarkSupportBundleUploaded notifies the Replicated API that the upload is complete.
func MarkSupportBundleUploaded(creds *ReplicatedUploadCredentials, bundleID string) (string, error) {
	reqURL := fmt.Sprintf("%s/v3/supportbundle/%s/uploaded", creds.Endpoint, url.PathEscape(bundleID))

	payload, err := json.Marshal(markUploadedRequest{ChannelID: creds.ChannelID})
	if err != nil {
		return "", errors.Wrap(err, "marshal uploaded request")
	}

	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(payload))
	if err != nil {
		return "", errors.Wrap(err, "create mark-uploaded request")
	}

	req.Header.Set("Content-Type", "application/json")
	setBasicAuth(req, creds.LicenseID)

	client := &http.Client{Timeout: apiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "mark support bundle uploaded")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorResponseBytes))
		return "", fmt.Errorf("mark uploaded failed: status %d: %s", resp.StatusCode, string(body))
	}

	var uploaded markUploadedResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploaded); err != nil {
		return "", errors.Wrap(err, "decode mark-uploaded response")
	}

	return uploaded.Slug, nil
}

// UploadSupportBundleToReplicated performs the full 3-step presigned URL upload flow:
// 1. Get a presigned S3 URL from the Replicated API
// 2. Upload the archive directly to S3
// 3. Notify the Replicated API that the upload is complete
func UploadSupportBundleToReplicated(creds *ReplicatedUploadCredentials, archivePath string) (string, error) {
	// Step 1: Get presigned upload URL
	uploadURL, err := GetPresignedUploadURL(creds)
	if err != nil {
		return "", errors.Wrap(err, "get presigned upload URL")
	}

	// Step 2: Upload to S3
	if err := UploadToS3(uploadURL.UploadURL, archivePath); err != nil {
		return "", errors.Wrap(err, "upload bundle to S3")
	}

	// Step 3: Mark as uploaded
	slug, err := MarkSupportBundleUploaded(creds, uploadURL.BundleID)
	if err != nil {
		return "", errors.Wrap(err, "mark bundle as uploaded")
	}

	return slug, nil
}

// validateEndpoint ensures the endpoint uses HTTPS to protect Basic Auth credentials.
func validateEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("endpoint must use HTTPS (got %q)", parsed.Scheme)
	}
	return nil
}

// validatePresignedURL ensures the presigned URL uses HTTPS to prevent SSRF via HTTP.
func validatePresignedURL(presignedURL string) error {
	parsed, err := url.Parse(presignedURL)
	if err != nil {
		return fmt.Errorf("failed to parse presigned URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("presigned URL must use HTTPS (got %q)", parsed.Scheme)
	}
	return nil
}

func setBasicAuth(req *http.Request, licenseID string) {
	auth := base64.StdEncoding.EncodeToString([]byte(licenseID + ":" + licenseID))
	req.Header.Set("Authorization", "Basic "+auth)
}
