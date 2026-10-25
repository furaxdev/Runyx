package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDatabaseFileIsNotWorldReadable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	info, err := os.Stat(filepath.Join(dir, "runyx.db"))
	if err != nil {
		t.Fatalf("stat runyx.db: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("runyx.db mode = %v, expected no group/other permission bits (it holds every conversation the user has had)", info.Mode().Perm())
	}
}

func TestConversationAndMessages(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	conv, err := s.NewConversation("Test", "qwen2.5:0.5b", "")
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}

	if _, err := s.AddMessage(conv.ID, "", "main", "user", "Hello"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if _, err := s.AddMessage(conv.ID, "", "main", "assistant", "Hi there!"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	msgs, err := s.Messages(conv.ID, "main")
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Content != "Hello" || msgs[1].Content != "Hi there!" {
		t.Errorf("unexpected message order/content: %+v", msgs)
	}
}

func TestForkCreatesNewBranch(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()
	conv, _ := s.NewConversation("Test", "m", "")

	m1, _ := s.AddMessage(conv.ID, "", "main", "user", "Quelle est la capitale de la France ?")
	s.AddMessage(conv.ID, m1.ID, "main", "assistant", "Paris")

	if err := s.Fork(conv.ID, m1.ID, "edit-1", "Quelle est la capitale de l'Allemagne ?"); err != nil {
		t.Fatalf("Fork: %v", err)
	}

	branches, err := s.Branches(conv.ID)
	if err != nil || len(branches) != 2 {
		t.Fatalf("Branches: got %v, err=%v", branches, err)
	}

	forked, err := s.Messages(conv.ID, "edit-1")
	if err != nil {
		t.Fatalf("Messages(edit-1): %v", err)
	}
	if len(forked) != 1 || forked[0].Content != "Quelle est la capitale de l'Allemagne ?" {
		t.Errorf("forked branch content = %+v", forked)
	}
}

func TestSearchFullText(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()
	conv, _ := s.NewConversation("Test", "m", "")
	s.AddMessage(conv.ID, "", "main", "user", "Comment faire une tarte Tatin ?")
	s.AddMessage(conv.ID, "", "main", "user", "Quelle heure est-il ?")

	results, err := s.Search("Tatin", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || !strings.Contains(results[0].Content, "Tatin") {
		t.Errorf("Search results = %+v", results)
	}
}

func TestPinAndExport(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()
	conv, _ := s.NewConversation("Pinned convo", "m", "")
	if err := s.Pin(conv.ID, true); err != nil {
		t.Fatalf("Pin: %v", err)
	}
	s.AddMessage(conv.ID, "", "main", "user", "hello")
	s.AddMessage(conv.ID, "", "main", "assistant", "world")

	md, err := s.ExportMarkdown(conv.ID, "main")
	if err != nil {
		t.Fatalf("ExportMarkdown: %v", err)
	}
	if !strings.Contains(md, "hello") || !strings.Contains(md, "world") {
		t.Errorf("markdown export missing content: %q", md)
	}

	js, err := s.ExportJSON(conv.ID, "main")
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	if !strings.Contains(string(js), "hello") {
		t.Errorf("json export missing content: %s", js)
	}

	convos, err := s.ListConversations()
	if err != nil || len(convos) != 1 || !convos[0].Pinned {
		t.Errorf("ListConversations: %+v, err=%v", convos, err)
	}
}

func TestRename(t *testing.T) {
	s, _ := Open(t.TempDir())
	defer s.Close()
	conv, _ := s.NewConversation("Original", "m", "")

	if err := s.Rename(conv.ID, "Renamed"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	convos, err := s.ListConversations()
	if err != nil || len(convos) != 1 || convos[0].Title != "Renamed" {
		t.Errorf("ListConversations: %+v, err=%v", convos, err)
	}
}
