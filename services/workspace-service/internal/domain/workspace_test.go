package domain

import "testing"

func TestIsValidRole(t *testing.T) {
	valid := []string{"owner", "admin", "member", "guest"}
	for _, r := range valid {
		if !IsValidRole(r) {
			t.Errorf("IsValidRole(%q) = false, want true", r)
		}
	}

	invalid := []string{"", "superadmin", "root", "OWNER", "Admin"}
	for _, r := range invalid {
		if IsValidRole(r) {
			t.Errorf("IsValidRole(%q) = true, want false", r)
		}
	}
}

func TestCanManageMembers(t *testing.T) {
	canManage := []string{"owner", "admin"}
	for _, r := range canManage {
		if !CanManageMembers(r) {
			t.Errorf("CanManageMembers(%q) = false, want true", r)
		}
	}

	cannotManage := []string{"member", "guest", "", "unknown"}
	for _, r := range cannotManage {
		if CanManageMembers(r) {
			t.Errorf("CanManageMembers(%q) = true, want false", r)
		}
	}
}
