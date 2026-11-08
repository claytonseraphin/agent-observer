package auth

// import (
// 	"errors"
// )

type Permission uint8

const (
	Read Permission = 1 << iota
	Write
	Delete
	Manage
)

type Role Permission

const (
	Viewer Role = Role(Read)
	Editor Role = Role(Read) | Role(Write)
	Owner  Role = Role(Read) | Role(Write) | Role(Delete) | Role(Manage)
)

func (r Role) Can(p Permission) bool {
	return Permission(r)&p == p
}

func RoleFor(user User, projectID string) Role {
	return Owner
}
