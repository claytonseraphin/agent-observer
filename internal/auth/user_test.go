package auth

import (
	"testing"
)

func TestLocalUser(t *testing.T) {
	wantedUser := User{ID: "local", Name: "local user"}
	getLocalUser := LocalUser()

	if getLocalUser != wantedUser {
		t.Fatalf("incorrect local user id: got %q, want %q", getLocalUser, wantedUser)
	}
}
