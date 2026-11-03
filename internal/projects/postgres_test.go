package projects

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/claytonseraphin/agent-observer/internal/db"
)

func newTestStore(t *testing.T) *PostgresStore {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		t.Skip("database url not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database connection failed %v", err)
	}
	t.Cleanup(pool.Close)

	err = db.Migrate(ctx, pool)
	if err != nil {
		t.Fatalf("database migration failed: %v", err)
	}

	return NewPostgresStore(pool)
}

func newTestProject(t *testing.T, store *PostgresStore) Project {
	t.Helper()

	newProjectID := NewProjectID()
	key := NewKey()
	keyHash := HashKey(key)
	keyHint := KeyHint(key)

	project := Project{
		ID:      newProjectID,
		Name:    "Project One " + newProjectID,
		KeyHash: keyHash,
		KeyHint: keyHint,
	}
	ctx := context.Background()
	t.Cleanup(func() {
		_, err := store.pool.Exec(ctx, "DELETE FROM projects WHERE id = $1", project.ID)
		if err != nil {
			t.Errorf("cleanup failed for %s: %v", project.ID, err)
		}
	})

	return project
}

func TestCreateProject(t *testing.T) {
	store := newTestStore(t)
	project := newTestProject(t, store)
	ctx := context.Background()

	created, err := store.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("error creating project, %v", err)
	}

	if created.ID != project.ID {
		t.Errorf("The id saved: %s, does not match the payload id: %s", created.ID, project.ID)
	}

	if created.Name != project.Name {
		t.Errorf("the name save: %s, does not match the payload name: %s", created.Name, project.Name)
	}

	if created.CreatedAt.IsZero() {
		t.Errorf("created_at was not set")
	}

	query := `SELECT count(*) FROM projects WHERE id = $1`
	var count int

	err = store.pool.QueryRow(
		ctx,
		query,
		project.ID,
	).Scan(
		&count,
	)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}
	if count != 1 {
		t.Errorf("project with ID %s was not found", project.ID)
	}
}

// Test adding agents in Create Project
func TestAddingAgentsInCreateProject(t *testing.T) {
	store := newTestStore(t)
	project := newTestProject(t, store)

	ctx := context.Background()

	news := Agent{
		Name: "news",
		URL:  "http://localhost:8001",
	}

	finance := Agent{
		Name: "finance",
		URL:  "http://localhost:8002",
	}

	project.Agents = append(project.Agents, news, finance)

	created, err := store.CreateProject(ctx, project)

	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}

	if created.CreatedAt.IsZero() {
		t.Errorf("created_at was not set")
	}

	query := `SELECT COUNT(*) FROM project_agents WHERE project_id = $1`
	var count int

	err = store.pool.QueryRow(
		ctx,
		query,
		project.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if count != 2 {
		t.Errorf("incorrect number of agents: %d", count)
	}
}

// Test Project name already taken
func TestProjectNameAlreadyTaken(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	projectOne := newTestProject(t, store)

	_, err := store.CreateProject(ctx, projectOne)
	if err != nil {
		t.Fatalf("error creating project one: %v", err)
	}

	projectTwoID := NewProjectID()
	key := NewKey()
	key_hash := HashKey(key)
	key_hint := KeyHint(key)
	projectTwo := Project{
		ID:      projectTwoID,
		Name:    strings.ToUpper(projectOne.Name),
		KeyHash: key_hash,
		KeyHint: key_hint,
	}

	t.Cleanup(func() {
		_, err := store.pool.Exec(ctx, "DELETE FROM projects WHERE id = $1", projectTwo.ID)
		if err != nil {
			t.Errorf("cleanup failed for %s: %v", projectTwo.ID, err)
		}
	})

	_, err = store.CreateProject(ctx, projectTwo)
	if !errors.Is(err, ErrNameTaken) {
		t.Errorf("CreateProject with a duplicate name: got error %v, want %v", err, ErrNameTaken)
	}
}

// Test Duplicate agent name
func TestDuplicateAgentName(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	project := newTestProject(t, store)

	agentOne := Agent{
		Name: "news",
		URL:  "http://localhost:8001",
	}

	agentTwo := Agent{
		Name: "news",
		URL:  "http://localhost:8002",
	}

	project.Agents = append(project.Agents, agentOne, agentTwo)

	_, err := store.CreateProject(ctx, project)
	if !errors.Is(err, ErrAgentExists) {
		t.Errorf("CreateProject with duplicate agent name: got error %v, want %v", err, ErrAgentExists)
	}

	var count int
	err = store.pool.QueryRow(
		ctx, `SELECT count(*) FROM projects WHERE id = $1`,
		project.ID,
	).Scan(&count)

	if err != nil {
		t.Fatalf("failed to query project: %v", err)
	}

	if count != 0 {
		t.Errorf("project %s was not rolled back: found %d rows, want 0", project.Name, count)
	}
}

// A project without a key (the default-project style): empty hash and hint are stored as NULL
func TestCreateProjectWithoutKey(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	projectOne := newTestProject(t, store)
	projectOne.KeyHash = ""
	projectOne.KeyHint = ""

	projectTwo := newTestProject(t, store)
	projectTwo.KeyHash = ""
	projectTwo.KeyHint = ""

	for _, project := range []Project{projectOne, projectTwo} {
		_, err := store.CreateProject(ctx, project)
		if err != nil {
			t.Fatalf("error creating project %s without a key: %v", project.ID, err)
		}

		var bothNull bool
		err = store.pool.QueryRow(
			ctx,
			`SELECT key_hash IS NULL AND key_hint IS NULL FROM projects WHERE id = $1`,
			project.ID,
		).Scan(&bothNull)
		if err != nil {
			t.Fatalf("failed to query project %s: %v", project.ID, err)
		}
		if !bothNull {
			t.Errorf("project %s: empty key_hash and key_hint should be stored as NULL", project.ID)
		}
	}
}
