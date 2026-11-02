package administration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseRequestCanonicalizesLogins(t *testing.T) {
	req, err := ParseRequest([]byte(`{"kind":"invite","logins":[" @Octo-Cat ","octo-cat","@MONA_shortcode","mona_SHORTCODE","hubot"]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Octo-Cat", "MONA_shortcode", "hubot"}
	if strings.Join(req.Logins, ",") != strings.Join(want, ",") || req.Role != RoleMember || req.TeamSlug != "" {
		t.Fatalf("request = %+v, want logins %v in input order with member role", req, want)
	}
	again, err := ParseRequest([]byte(`{"logins":["octo-cat","mona_shortcode","HUBOT"],"role":"member","kind":"invite"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(req.Fingerprint(), again.Fingerprint()) {
		t.Fatal("case and an explicit member role changed the fingerprint of the same request")
	}
	reordered, _ := ParseRequest([]byte(`{"kind":"invite","logins":["hubot","octo-cat","mona_shortcode"]}`))
	team, _ := ParseRequest([]byte(`{"kind":"team_membership","team_slug":"Core","logins":["octo-cat","mona_shortcode","hubot"]}`))
	for name, other := range map[string]Request{"order": reordered, "kind": team} {
		if bytes.Equal(req.Fingerprint(), other.Fingerprint()) {
			t.Fatalf("a different %s has the same fingerprint", name)
		}
	}
	if team.TeamSlug != "core" {
		t.Fatalf("team slug = %q, want lowercase", team.TeamSlug)
	}
}

func TestParseRequestRejectsUnsafeInput(t *testing.T) {
	fifty := make([]string, 0, MaxTargets+1)
	for i := range MaxTargets {
		fifty = append(fifty, fmt.Sprintf("user%d", i))
	}
	encode := func(logins []string) string {
		b, _ := json.Marshal(logins)
		return string(b)
	}
	// Duplicates do not count toward the limit.
	if _, err := ParseRequest([]byte(`{"kind":"invite","logins":` + encode(append(fifty, "USER0", "@user1")) + `}`)); err != nil {
		t.Fatalf("50 unique logins with duplicates rejected: %v", err)
	}
	cases := []struct {
		name, body, code string
	}{
		{"empty body", ``, CodeInvalidRequest},
		{"not an object", `[]`, CodeInvalidRequest},
		{"two objects", `{"kind":"invite","logins":["a"]}{}`, CodeInvalidRequest},
		{"unknown field", `{"kind":"invite","logins":["a"],"teams":[1]}`, CodeInvalidRequest},
		{"unknown kind", `{"kind":"remove","logins":["a"]}`, CodeInvalidRequest},
		{"no logins", `{"kind":"invite","logins":[]}`, CodeInvalidRequest},
		{"null logins", `{"kind":"invite"}`, CodeInvalidRequest},
		{"blank login", `{"kind":"invite","logins":["  "]}`, CodeInvalidLogin},
		{"path segment", `{"kind":"invite","logins":["a/../b"]}`, CodeInvalidLogin},
		{"query", `{"kind":"invite","logins":["a?role=admin"]}`, CodeInvalidLogin},
		{"encoded", `{"kind":"invite","logins":["a%2Fb"]}`, CodeInvalidLogin},
		{"leading hyphen", `{"kind":"invite","logins":["-a"]}`, CodeInvalidLogin},
		{"too long", `{"kind":"invite","logins":["` + strings.Repeat("a", 101) + `"]}`, CodeInvalidLogin},
		{"51 unique", `{"kind":"invite","logins":` + encode(append(fifty, "extra")) + `}`, CodeTooManyLogins},
		{"invite as owner", `{"kind":"invite","logins":["a"],"role":"admin"}`, CodeInvalidRequest},
		{"invite with team", `{"kind":"invite","logins":["a"],"team_slug":"core"}`, CodeInvalidRequest},
		{"team without slug", `{"kind":"team_membership","logins":["a"]}`, CodeInvalidRequest},
		{"team maintainer", `{"kind":"team_membership","logins":["a"],"team_slug":"core","role":"admin"}`, CodeInvalidRequest},
		{"team slug path", `{"kind":"team_membership","logins":["a"],"team_slug":"../members"}`, CodeInvalidRequest},
		{"role without role", `{"kind":"organization_role","logins":["a"]}`, CodeInvalidRequest},
		{"role with team", `{"kind":"organization_role","logins":["a"],"role":"member","team_slug":"core"}`, CodeInvalidRequest},
		{"billing role", `{"kind":"organization_role","logins":["a"],"role":"billing_manager"}`, CodeInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRequest([]byte(tc.body))
			var inErr *InputError
			if !errors.As(err, &inErr) || inErr.Code != tc.code {
				t.Fatalf("error = %v, want code %s", err, tc.code)
			}
		})
	}
}

func TestParseIdempotencyKey(t *testing.T) {
	if key, ok := ParseIdempotencyKey("6F9619FF-8B86-D011-B42D-00C04FC964FF"); !ok || key != "6f9619ff-8b86-d011-b42d-00c04fc964ff" {
		t.Fatalf("key = %q %v", key, ok)
	}
	for _, bad := range []string{"", "6f9619ff8b86d011b42d00c04fc964ff", "6f9619ff-8b86-d011-b42d-00c04fc964fg", "{6f9619ff-8b86-d011-b42d-00c04fc964ff}"} {
		if _, ok := ParseIdempotencyKey(bad); ok {
			t.Errorf("accepted %q", bad)
		}
	}
}
