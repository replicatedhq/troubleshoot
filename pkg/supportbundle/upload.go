package supportbundle

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// UploadToReplicatedApp uploads a support bundle directly to replicated.app
// using the app slug as the upload path
func UploadToReplicatedApp(bundlePath, licenseID, appSlug string) error {
	// Open the bundle file
	file, err := os.Open(bundlePath)
	if err != nil {
		return errors.Wrap(err, "failed to open bundle file")
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return errors.Wrap(err, "failed to stat file")
	}

	// Build the upload URL using the app slug
	uploadURL := fmt.Sprintf("https://replicated.app/supportbundle/upload/%s", appSlug)

	// Create the request
	req, err := http.NewRequest("POST", uploadURL, file)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	// Set headers
	req.Header.Set("Authorization", licenseID)
	req.Header.Set("Content-Type", "application/gzip")
	req.ContentLength = stat.Size()

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to upload bundle")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}

	return nil
}

// UploadBundleAutoDetect uploads a support bundle with automatic license detection
func UploadBundleAutoDetect(bundlePath string, providedLicenseID string) error {
	licenseID := providedLicenseID
	var appSlug string

	// Try to extract license from bundle if not provided
	if licenseID == "" {
		extractedLicense, extractedAppSlug, err := ExtractLicenseFromBundle(bundlePath)
		if err != nil {
			return errors.Wrap(err, "failed to extract license from bundle")
		}
		if extractedLicense == "" {
			return errors.New("could not find license ID in bundle. Please provide --license-id")
		}
		licenseID = extractedLicense
		appSlug = extractedAppSlug
	}

	// Upload the bundle
	fmt.Printf("Uploading support bundle to replicated.app...\n")
	if err := UploadToReplicatedApp(bundlePath, licenseID, appSlug); err != nil {
		return errors.Wrap(err, "failed to upload bundle")
	}

	fmt.Printf("Successfully uploaded support bundle\n")
	return nil
}

