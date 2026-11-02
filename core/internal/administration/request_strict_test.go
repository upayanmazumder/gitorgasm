package administration

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRequestRejectsAmbiguousJSON(t *testing.T) {
	for _, body := range []string{
		`null`,
		`{"kind":"invite","kind":"organization_role","role":"admin","logins":["a"]}`,
		`{"kind":"invite","logins":["a"],"role":null}`,
		`{"kind":"invite","logins":[null]}`,
		`{"kind":"invite","logins":["@@a"]}`,
		`{"kind":"team_membership","logins":["a"],"team_slug":"%2f"}`,
		`{"kind":"invite","logins":["a"],"padding":"` + strings.Repeat("x", 16<<10) + `"}`,
	} {
		if _, err := ParseRequest([]byte(body)); err == nil {
			t.Fatalf("accepted ambiguous or unsafe JSON: %.160s", body)
		}
	}
}

func TestInputErrorDoesNotEchoUnsafeInput(t *testing.T) {
	_, err := ParseRequest([]byte(`{"kind":"invite","logins":["private?token=secret"]}`))
	var input *InputError
	if !errors.As(err, &input) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestParseRequestRejectsInapplicableOrEmptyExplicitFields(t *testing.T) {
	for _, body := range []string{
		`{"kind":"invite","logins":["a"],"team_slug":""}`,
		`{"kind":"organization_role","logins":["a"],"role":"member","team_slug":""}`,
		`{"kind":"invite","logins":["a"],"role":""}`,
		`{"kind":"team_membership","logins":["a"],"team_slug":"core","role":""}`,
	} {
		if _, err := ParseRequest([]byte(body)); err == nil {
			t.Fatalf("accepted invalid field combination: %s", body)
		}
	}
}
