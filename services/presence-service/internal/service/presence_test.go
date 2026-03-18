package service

import "testing"

// ── key helper tests ──────────────────────────────────────────────────────────

func TestOnlineKey(t *testing.T) {
	got := onlineKey("user-1", "ws-1")
	want := "presence:online:ws-1:user-1"
	if got != want {
		t.Errorf("onlineKey: got %q, want %q", got, want)
	}
}

func TestAwayKey(t *testing.T) {
	got := awayKey("user-1", "ws-1")
	want := "presence:away:ws-1:user-1"
	if got != want {
		t.Errorf("awayKey: got %q, want %q", got, want)
	}
}

func TestUserIDFromKey(t *testing.T) {
	cases := []struct {
		key    string
		prefix string
		want   string
	}{
		{"presence:online:ws-1:user-abc", "presence:online:ws-1:", "user-abc"},
		{"presence:away:ws-2:user-xyz", "presence:away:ws-2:", "user-xyz"},
		{"too-short", "presence:online:ws-1:", ""},
		{"presence:online:ws-1:", "presence:online:ws-1:", ""},
	}
	for _, c := range cases {
		got := userIDFromKey(c.key, c.prefix)
		if got != c.want {
			t.Errorf("userIDFromKey(%q, %q) = %q, want %q", c.key, c.prefix, got, c.want)
		}
	}
}

// ── TTL constants ─────────────────────────────────────────────────────────────

func TestPresenceTTLConstants(t *testing.T) {
	if onlineTTL <= 0 {
		t.Error("onlineTTL must be positive")
	}
	if awayTTL <= 0 {
		t.Error("awayTTL must be positive")
	}
	if awayTTL <= onlineTTL {
		t.Error("awayTTL should be greater than onlineTTL")
	}
}
