package name

import "testing"

func TestToGo(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"id", "ID"},             // built-in acronym
		{"aws_nodes", "AWSNode"}, // built-in acronym + singularize
		{"users", "User"},        // plain singularize
		{"plugin_versions", "PluginVersion"},
	}
	for _, tt := range tests {
		if got := ToGo(tt.in); got != tt.want {
			t.Errorf("ToGo(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRegisterAcronyms(t *testing.T) {
	// Without registration, "dns" is not an acronym and gets singularized to "Dn".
	if got := ToGo("dns_zones"); got != "DnZone" {
		t.Fatalf("precondition: ToGo(\"dns_zones\") = %q, want %q", got, "DnZone")
	}

	// Mixed-case keys are matched case-insensitively; the value is emitted verbatim.
	RegisterAcronyms(map[string]string{"DNS": "DNS", "oauth": "OAuth"})

	tests := []struct {
		in   string
		want string
	}{
		{"dns_zones", "DNSZone"},       // user acronym, not singularized
		{"oauth_tokens", "OAuthToken"}, // verbatim mixed-case Go form
		{"id", "ID"},                   // built-in still works after merge
	}
	for _, tt := range tests {
		if got := ToGo(tt.in); got != tt.want {
			t.Errorf("ToGo(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
