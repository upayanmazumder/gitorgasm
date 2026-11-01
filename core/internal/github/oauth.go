package github

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
)

// AuthorizeURL builds the web application flow URL with state and a PKCE S256 challenge.
func (c *Client) AuthorizeURL(state, codeChallenge string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.OAuthCallback)
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	q.Set("allow_signup", "false")
	return c.cfg.WebBaseURL + "/login/oauth/authorize?" + q.Encode()
}

// InstallURL is where users install the App on an organization.
func (c *Client) InstallURL() string { return c.cfg.InstallURL }

// ExchangeCode calls POST https://github.com/login/oauth/access_token for a user token.
func (c *Client) ExchangeCode(ctx context.Context, code, codeVerifier string) (*models.UserToken, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", c.cfg.OAuthCallback)
	form.Set("code_verifier", codeVerifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.WebBaseURL+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var out struct {
		models.UserToken
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if _, err := c.do(ctx, "oauth/access-token", "oauth", req, &out); err != nil {
		return nil, err
	}
	// GitHub reports OAuth failures with HTTP 200 and an error field.
	if out.Error != "" || out.AccessToken == "" {
		return nil, &APIError{Operation: "oauth/access-token", Status: http.StatusOK, Kind: KindInvalid, Message: out.Error}
	}
	if out.ExpiresIn > 0 {
		out.ExpiresAt = c.now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return &out.UserToken, nil
}
