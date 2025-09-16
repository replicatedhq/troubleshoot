package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/credentials"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func LoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the vendor portal and save your API token",
		Long: `Launches a browser-based login to authenticate with the vendor portal.

After successful login, the API token is stored locally and used automatically by
support-bundle commands. The TROUBLESHOOT_TOKEN environment variable still overrides
the stored token when set (useful for CI).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			portalEndpoint := v.GetString("endpoint")
			if portalEndpoint == "" {
				portalEndpoint = "https://vendor.replicated.com"
			}

			if env := strings.TrimSpace(os.Getenv("TROUBLESHOOT_TOKEN")); env != "" {
				fmt.Fprintf(os.Stderr, "Warning: TROUBLESHOOT_TOKEN is set and will override saved credentials when running commands.\n")
			}

			// Start a local callback server on a random port
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return errors.Wrap(err, "start local callback server")
			}
			defer listener.Close()

			redirectURI := fmt.Sprintf("http://%s/callback", listener.Addr().String())
			nonce, err := generateNonce()
			if err != nil {
				return err
			}

			// Build login URL
			u, err := url.Parse(portalEndpoint)
			if err != nil {
				return errors.Wrap(err, "parse endpoint")
			}
			if !strings.HasPrefix(u.Scheme, "http") {
				return errors.Errorf("invalid endpoint: %s", portalEndpoint)
			}
			u.Path = strings.TrimRight(u.Path, "/") + "/cli-login"
			q := u.Query()
			q.Set("redirect_uri", redirectURI)
			q.Set("nonce", nonce)
			q.Set("cli", "support-bundle")
			// Also include state for portals that prefer it
			q.Set("state", nonce)
			u.RawQuery = q.Encode()

			// Channel to receive token or error from callback handler
			tokenCh := make(chan string, 1)
			errCh := make(chan error, 1)

			srv := &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/callback" {
						http.NotFound(w, r)
						return
					}
					rNonce := r.URL.Query().Get("nonce")
					if rNonce == "" {
						rNonce = r.URL.Query().Get("state")
					}
					// Use callback nonce if present, else fall back to our original
					exchangeNonce := rNonce
					if exchangeNonce == "" {
						exchangeNonce = nonce
					}
					exchange := r.URL.Query().Get("exchange")
					if exchange == "" {
						http.Error(w, "Missing exchange URL", http.StatusBadRequest)
						errCh <- errors.New("missing exchange URL")
						return
					}

					token, exErr := exchangeForToken(exchange, exchangeNonce)
					if exErr != nil {
						http.Error(w, "Login failed: "+exErr.Error(), http.StatusBadGateway)
						errCh <- exErr
						return
					}

					fmt.Fprint(w, "Login successful. You may close this tab and return to the terminal.")
					tokenCh <- token
				}),
			}

			go func() {
				_ = srv.Serve(listener)
			}()
			defer func() {
				// shutdown server after we return
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
			}()

			// Open browser
			if err := openBrowser(u.String()); err != nil {
				fmt.Fprintf(os.Stderr, "Please open this URL in your browser to continue:\n%s\n", u.String())
			} else {
				fmt.Fprintf(os.Stderr, "Opening browser for authentication... If it doesn't open, visit:\n%s\n", u.String())
			}

			// Wait for token or error with timeout
			select {
			case token := <-tokenCh:
				if err := credentials.SetCurrentCredentials(token); err != nil {
					return errors.Wrap(err, "save credentials")
				}
				fmt.Fprintf(os.Stderr, "Login complete. Credentials saved.\n")
				return nil
			case err := <-errCh:
				return err
			case <-time.After(5 * time.Minute):
				return errors.New("login timed out waiting for browser callback")
			}
		},
	}

	cmd.Flags().String("endpoint", "https://vendor.replicated.com", "vendor portal endpoint for login")
	return cmd
}

func LogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove saved credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := credentials.RemoveCurrentCredentials(); err != nil {
				return errors.Wrap(err, "remove credentials")
			}
			fmt.Fprintf(os.Stderr, "Logged out. Saved credentials removed.\n")
			return nil
		},
	}
	return cmd
}

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func exchangeForToken(exchangeURL, nonce string) (string, error) {
	req, err := http.NewRequest("GET", exchangeURL, nil)
	if err != nil {
		return "", err
	}
	// Provide nonce via header and query param for compatibility
	q := req.URL.Query()
	if q.Get("nonce") == "" {
		q.Set("nonce", nonce)
	}
	if q.Get("state") == "" {
		q.Set("state", nonce)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-State", nonce)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.Errorf("exchange failed: %s", resp.Status)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if strings.TrimSpace(body.Token) == "" {
		return "", errors.New("empty token from exchange")
	}
	return body.Token, nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		// linux and others
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", url).Start()
		}
		return errors.New("cannot find a way to open the browser automatically")
	}
}
