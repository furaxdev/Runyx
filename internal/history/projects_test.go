package history

import "testing"

func TestProjectCRUD(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	p, err := s.NewProject("Mon projet", "Tu es un assistant utile.", "/tmp/files")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	got, ok, err := s.GetProject(p.ID)
	if err != nil || !ok {
		t.Fatalf("GetProject: ok=%v err=%v", ok, err)
	}
	if got.Name != "Mon projet" {
		t.Errorf("Name = %q", got.Name)
	}

	if err := s.UpdateProjectPrompt(p.ID, "Nouveau prompt"); err != nil {
		t.Fatalf("UpdateProjectPrompt: %v", err)
	}
	got, _, _ = s.GetProject(p.ID)
	if got.SystemPrompt != "Nouveau prompt" {
		t.Errorf("SystemPrompt = %q", got.SystemPrompt)
	}

	list, err := s.ListProjects()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListProjects: %v, err=%v", list, err)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()
	_, ok, err := s.GetProject("does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for missing project")
	}
}

func TestUserMemoryVisibleAndDeletable(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()

	m, err := s.AddMemory("", "The user prefers short answers.")
	if err != nil {
		t.Fatalf("AddMemory: %v", err)
	}

	facts, err := s.ListMemory("")
	if err != nil || len(facts) != 1 || facts[0].Fact != m.Fact {
		t.Fatalf("ListMemory: %v, err=%v", facts, err)
	}

	if err := s.DeleteMemory(m.ID); err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}
	facts, _ = s.ListMemory("")
	if len(facts) != 0 {
		t.Errorf("expected memory to be gone after delete, got %v", facts)
	}
}

func TestMemoryScopedByProject(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()
	s.AddMemory("project-a", "fact A")
	s.AddMemory("project-b", "fact B")
	s.AddMemory("", "global fact")

	factsA, _ := s.ListMemory("project-a")
	if len(factsA) != 1 || factsA[0].Fact != "fact A" {
		t.Errorf("project-a facts = %v", factsA)
	}
	factsGlobal, _ := s.ListMemory("")
	if len(factsGlobal) != 1 || factsGlobal[0].Fact != "global fact" {
		t.Errorf("global facts = %v", factsGlobal)
	}
}
