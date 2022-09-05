package supportbundle

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
)

func uploadSupportBundle(r *troubleshootv1beta2.ResultRequest, archivePath string) error {
	contentType := getExpectedContentType(r.URI)
	if contentType != "" && contentType != "application/tar+gzip" {
		return fmt.Errorf("cannot upload content type %s", contentType)
	}

	for {
		f, err := os.Open(archivePath)
		if err != nil {
			return errors.Wrap(err, "open file")
		}
		defer f.Close()

		fileStat, err := f.Stat()
		if err != nil {
			return errors.Wrap(err, "stat file")
		}

		req, err := http.NewRequest(r.Method, r.URI, f)
		if err != nil {
			return errors.Wrap(err, "create request")
		}
		req.ContentLength = fileStat.Size()
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		httpClient := httputil.GetHttpClient()
		resp, err := httpClient.Do(req)
		if err != nil {
			if shouldRetryRequest(err) {
				continue
			}
			return errors.Wrap(err, "execute request")
		}

		if resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}

		break
	}

	// send redaction report
	if r.RedactURI != "" {
		type PutSupportBundleRedactions struct {
			Redactions redact.RedactionList `json:"redactions"`
		}

		redactBytes, err := json.Marshal(PutSupportBundleRedactions{Redactions: redact.GetRedactionList()})
		if err != nil {
			return errors.Wrap(err, "get redaction report")
		}

		for {
			req, err := http.NewRequest("PUT", r.RedactURI, bytes.NewReader(redactBytes))
			if err != nil {
				return errors.Wrap(err, "create redaction report request")
			}
			req.ContentLength = int64(len(redactBytes))

			httpClient := httputil.GetHttpClient()
			resp, err := httpClient.Do(req)
			if err != nil {
				if shouldRetryRequest(err) {
					continue
				}
				return errors.Wrap(err, "execute redaction request")
			}

			if resp.StatusCode >= 300 {
				return fmt.Errorf("unexpected redaction status code %d", resp.StatusCode)
			}

			break
		}
	}

	return nil
}

func callbackSupportBundleAPI(r *troubleshootv1beta2.ResultRequest, archivePath string) error {
	for {
		req, err := http.NewRequest(r.Method, r.URI, nil)
		if err != nil {
			return errors.Wrap(err, "create request")
		}

		httpClient := httputil.GetHttpClient()
		resp, err := httpClient.Do(req)
		if err != nil {
			if shouldRetryRequest(err) {
				continue
			}
			return errors.Wrap(err, "execute request")
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}

		break
	}
	return nil
}

func getExpectedContentType(uploadURL string) string {
	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("Content-Type")
}

func shouldRetryRequest(err error) bool {
	if strings.Contains(err.Error(), "x509") && canTryInsecure() {
		httputil.AddTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
		return true
	}
	return false
}

func canTryInsecure() bool {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return false
	}
	prompt := promptui.Prompt{
		Label:     "Connection appears to be insecure. Would you like to attempt to create a support bundle anyway?",
		IsConfirm: true,
	}

	_, err := prompt.Run()
	return err == nil
}
