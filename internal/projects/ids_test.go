package projects

import (
	"regexp"
	"testing"
)

func TestNewProjectID(t *testing.T) {
	pattern := regexp.MustCompile(`^prj_[a-z0-9]{12}$`)
	newProject := NewProjectID()

	if !pattern.MatchString(newProject) {
		t.Errorf("NewProjectID() = %q, want prj_ followed by 12 characters", newProject)
	}
}

func TestNewProjectsDifferentIDs(t *testing.T) {
	projectOne := NewProjectID()
	projectTwo := NewProjectID()

	if projectOne == projectTwo {
		t.Errorf("NewProjectID() returned the same ID twice: %q", projectOne)
	}
}
