package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var githubKeys = []string{
	"GITHUB_APP_ID", "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_PRIVATE_KEY",
	"GITHUB_WEBHOOK_SECRET", "GITHUB_OAUTH_CALLBACK_URL", "GITHUB_APP_SLUG",
}

// loadGitHub returns nil when no GitHub variable is set. Partial configuration is an error.
func loadGitHub(get func(string, string) string) (*GitHub, []error) {
	var set, missing []string
	for _, k := range githubKeys {
		if get(k, "") != "" {
			set = append(set, k)
		} else {
			missing = append(missing, k)
		}
	}
	if len(set) == 0 {
		return nil, nil
	}
	if len(missing) > 0 {
		return nil, []error{fmt.Errorf("GitHub App configuration is partial; missing %s", strings.Join(missing, ", "))}
	}
	var errs []error
	appID, err := strconv.ParseInt(get("GITHUB_APP_ID", ""), 10, 64)
	if err != nil || appID <= 0 {
		errs = append(errs, errors.New("GITHUB_APP_ID must be a positive integer"))
	}
	pk, err := parsePrivateKey(get("GITHUB_PRIVATE_KEY", ""))
	if err != nil {
		errs = append(errs, fmt.Errorf("GITHUB_PRIVATE_KEY: %w", err))
	}
	timeout, err := time.ParseDuration(get("GITHUB_REQUEST_TIMEOUT", "20s"))
	if err != nil || timeout <= 0 {
		errs = append(errs, errors.New("GITHUB_REQUEST_TIMEOUT must be a positive duration"))
	}
	web := strings.TrimRight(get("GITHUB_WEB_BASE_URL", "https://github.com"), "/")
	return &GitHub{
		AppID:          appID,
		ClientID:       get("GITHUB_CLIENT_ID", ""),
		ClientSecret:   get("GITHUB_CLIENT_SECRET", ""),
		PrivateKey:     pk,
		WebhookSecret:  []byte(get("GITHUB_WEBHOOK_SECRET", "")),
		APIBaseURL:     strings.TrimRight(get("GITHUB_API_BASE_URL", "https://api.github.com"), "/"),
		WebBaseURL:     web,
		OAuthCallback:  get("GITHUB_OAUTH_CALLBACK_URL", ""),
		InstallURL:     web + "/apps/" + get("GITHUB_APP_SLUG", "") + "/installations/new",
		RequestTimeout: timeout,
	}, errs
}

// parsePrivateKey accepts PEM text, PEM with literal "\n" escapes, or base64-encoded PEM.
func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	text := strings.ReplaceAll(raw, `\n`, "\n")
	if !strings.Contains(text, "-----BEGIN") {
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
		if err != nil {
			return nil, errors.New("not PEM and not base64-encoded PEM")
		}
		text = string(dec)
	}
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("unsupported private key encoding")
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return rk, nil
}
