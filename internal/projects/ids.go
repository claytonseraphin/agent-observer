package projects

func NewProjectID() string {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"

	return randomString(12, charset, "prj_")
}
