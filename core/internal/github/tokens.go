package github

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type installationToken struct {
	value     string
	expiresAt time.Time
}

// appJWT backdates iat by 60 seconds for clock drift and keeps exp within GitHub's ten-minute limit.
func (c *Client) appJWT() (string, error) {
	now := c.now()
	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(c.cfg.AppID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(c.cfg.PrivateKey)
}

// installationToken caches POST /app/installations/{installation_id}/access_tokens (apps/create-installation-access-token), refreshing five minutes before expiry.
func (c *Client) installationToken(ctx context.Context, id int64) (string, error) {
	c.mu.Lock()
	t, ok := c.tokens[id]
	c.mu.Unlock()
	if ok && c.now().Before(t.expiresAt.Add(-5*time.Minute)) {
		return t.value, nil
	}
	jwtToken, err := c.appJWT()
	if err != nil {
		return "", err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/app/installations/"+strconv.FormatInt(id, 10)+"/access_tokens", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if _, err := c.do(ctx, "apps/create-installation-access-token", "app", req, &out); err != nil {
		return "", err
	}
	c.mu.Lock()
	c.tokens[id] = installationToken{value: out.Token, expiresAt: out.ExpiresAt}
	c.mu.Unlock()
	return out.Token, nil
}

// ForgetInstallation drops a cached token, for example after the installation is deleted.
func (c *Client) ForgetInstallation(id int64) {
	c.mu.Lock()
	delete(c.tokens, id)
	c.mu.Unlock()
}
