package auth

import (
	"context"
	"testing"
)

func TestUserRequestContext(t *testing.T) {
	ctx := context.Background()

	user := User{
		ID:   "usr_1234",
		Name: "user_test",
	}

	withUser := ContextWithUser(ctx, user)

	u, ok := UserFromContext(withUser)

	if !ok {
		t.Fatal("UserFromContext reported no user, want one")
	}

	if u != user {
		t.Errorf("incorrect user: got %q, want %q", u, user)
	}
}

func TestUserRequestContextBackGround(t *testing.T) {
	ctx := context.Background()
	user := User{}

	u, ok := UserFromContext(ctx)

	if ok {
		t.Fatalf("UserFromContext reported the wrong context %q", u)
	}

	if u != user {
		t.Fatalf("user should be empty, got %q", u)
	}
}
