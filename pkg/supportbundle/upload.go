package supportbundle

import (
	"fmt"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

// UploadToReplicatedApp uploads a support bundle directly to replicated.app
// using the app slug as the upload path
func UploadToReplicatedApp(bundlePath, licenseID, appSlug, uploadDomain string) error {
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

	// Use custom domain if provided, otherwise default to replicated.app
	domain := uploadDomain
	if domain == "" {
		domain = "replicated.app"
	}

	// Build the upload URL using the app slug
	uploadURL := fmt.Sprintf("https://%s/supportbundle/upload/%s", domain, appSlug)

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

// UploadBundleAutoDetect uploads a support bundle with automatic license and app slug detection
func UploadBundleAutoDetect(bundlePath string, providedLicenseID, providedAppSlug, uploadDomain string) error {
	licenseID := providedLicenseID

	// Always extract from bundle to get app slug (and license if not provided)
	extractedLicense, extractedAppSlug, err := ExtractLicenseFromBundle(bundlePath)
	if err != nil {
		return errors.Wrap(err, "failed to extract data from bundle")
	}

	// Use provided license ID if given, otherwise use extracted one
	if licenseID == "" {
		if extractedLicense == "" {
			return errors.New("could not find license ID in bundle. Please provide --license-id")
		}
		licenseID = extractedLicense
	}

	// Use provided app slug if given, otherwise use extracted one
	appSlug := providedAppSlug
	if appSlug == "" {
		if extractedAppSlug == "" {
			return errors.New("could not determine app slug from bundle. Please provide --app-slug")
		}
		appSlug = extractedAppSlug
	}

	// Determine target domain for upload message
	targetDomain := uploadDomain
	if targetDomain == "" {
		targetDomain = "replicated.app"
	}

	// Upload the bundle
	fmt.Printf("Uploading support bundle to %s...\n", targetDomain)
	if err := UploadToReplicatedApp(bundlePath, licenseID, appSlug, uploadDomain); err != nil {
		return errors.Wrap(err, "failed to upload bundle")
	}

	fmt.Printf("Successfully uploaded support bundle\n")
	return nil
}
