package supportbundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

type VandoorUploadResponse struct {
	BundleID string `json:"bundleId"`
	URL      string `json:"url"`
}

// UploadToVandoor uploads a support bundle to vandoor using the 3-step process:
// 1. Get presigned URL from vandoor API
// 2. Upload file to S3 using presigned URL
// 3. Mark bundle as uploaded in vandoor API
func UploadToVandoor(bundlePath, endpoint, token, appID string) error {
	// Step 1: Get presigned URL
	uploadResp, err := getVandoorUploadURL(endpoint, token)
	if err != nil {
		return errors.Wrap(err, "get upload URL")
	}

	// Step 2: Upload file to S3
	if err := uploadFileToS3(bundlePath, uploadResp.URL); err != nil {
		return errors.Wrap(err, "upload to S3")
	}

	// Step 3: Mark as uploaded
	return markVandoorBundleUploaded(endpoint, token, uploadResp.BundleID, appID)
}

// getVandoorUploadURL calls GET /v3/supportbundle/upload-url
func getVandoorUploadURL(endpoint, token string) (*VandoorUploadResponse, error) {
	req, err := http.NewRequest("GET", endpoint+"/v3/supportbundle/upload-url", nil)
	if err != nil {
		return nil, errors.Wrap(err, "create request")
	}

	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed: HTTP %d", resp.StatusCode)
	}

	var uploadResp VandoorUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, errors.Wrap(err, "decode response")
	}

	return &uploadResp, nil
}

// uploadFileToS3 uploads the bundle file to the presigned S3 URL
func uploadFileToS3(bundlePath, s3URL string) error {
	file, err := os.Open(bundlePath)
	if err != nil {
		return errors.Wrap(err, "open file")
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return errors.Wrap(err, "stat file")
	}

	req, err := http.NewRequest("PUT", s3URL, file)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	req.Header.Set("Content-Type", "application/gzip")
	req.ContentLength = stat.Size()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("S3 upload failed: HTTP %d", resp.StatusCode)
	}

	return nil
}

// markVandoorBundleUploaded calls POST /v3/supportbundle/{bundleId}/uploaded
func markVandoorBundleUploaded(endpoint, token, bundleID, appID string) error {
	body := map[string]string{
		"app_id": appID,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return errors.Wrap(err, "marshal body")
	}

	url := endpoint + "/v3/supportbundle/" + bundleID + "/uploaded"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("mark uploaded failed: HTTP %d", resp.StatusCode)
	}

	return nil
}