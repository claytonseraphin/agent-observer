package auth

type User struct {
	ID   string
	Name string
}

func LocalUser() User {
	return User{ID: "local", Name: "local user"}
}
