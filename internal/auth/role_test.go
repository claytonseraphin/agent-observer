package auth

import (
	"testing"
)

func TestUserRoleAndWrongPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       Role
		permission Permission
		want       bool
	}{
		{"write permission for viewer", Viewer, Write, false},
		{"delete permission for viewer", Viewer, Delete, false},
		{"manage permission for viewer", Viewer, Manage, false},
		{"delete permission for editor", Editor, Delete, false},
		{"manage permission for editor", Editor, Manage, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.role.Can(tc.permission)
			if got != tc.want {
				t.Errorf("assigning wrong permission %d to role %d", tc.permission, tc.role)
			}
		})
	}
}

func TestUserRoleRightPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       Role
		permission Permission
		want       bool
	}{
		{"read permission for viewer", Viewer, Read, true},
		{"read permission for editor", Editor, Read, true},
		{"write permission for editor", Editor, Write, true},
		{"read permission for owner", Owner, Read, true},
		{"write permission for owner", Owner, Write, true},
		{"delete permission for owner", Owner, Delete, true},
		{"manage permission for owner", Owner, Manage, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.role.Can(tc.permission)
			if got != tc.want {
				t.Errorf("incorrect output: got %t, want %t", got, tc.want)
			}
		})
	}
}

func TestUserWithNoRole(t *testing.T) {
	tests := []struct {
		name       string
		role       Role
		permission Permission
		want       bool
	}{
		{"read permission for role zero", 0, Read, false},
		{"write permission for role zero", 0, Write, false},
		{"delete permission for role zero", 0, Delete, false},
		{"manage permission for role zero", 0, Manage, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.role.Can(tc.permission)
			if got != tc.want {
				t.Errorf("incorrect output: got %t, want %t", got, tc.want)
			}
		})
	}

}

func TestRoleFor(t *testing.T) {
	user := LocalUser()
	projectID := "prj_1234abc"

	roleFor := RoleFor(user, projectID)

	if roleFor != Owner {
		t.Errorf("wrong role: got %v, want %v", roleFor, Owner)
	}
}
