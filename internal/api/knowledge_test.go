package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newKnowledgeMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux, hist := newTestMux(t) // from runyx_test.go: wires RegisterRunyx already
	s := &Server{History: hist}
	s.RegisterKnowledge(mux, &RunyxExtras{History: hist})
	return mux
}

func TestProjectAndMemoryOverHTTP(t *testing.T) {
	mux := newKnowledgeMux(t)

	rec := doJSON(t, mux, "POST", "/api/runyx/projects", map[string]string{
		"Name": "Mon projet", "SystemPrompt": "Sois concis.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: status %d, body %s", rec.Code, rec.Body.String())
	}
	var proj struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &proj)

	rec = doJSON(t, mux, "GET", "/api/runyx/projects", nil)
	var projects []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &projects)
	if len(projects) != 1 {
		t.Fatalf("projects = %v", projects)
	}

	rec = doJSON(t, mux, "POST", "/api/runyx/memory", map[string]string{
		"ProjectID": proj.ID, "Fact": "The user likes coffee.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add memory: status %d, body %s", rec.Code, rec.Body.String())
	}
	var mem struct{ ID string }
	json.Unmarshal(rec.Body.Bytes(), &mem)

	rec = doJSON(t, mux, "GET", "/api/runyx/memory?project_id="+proj.ID, nil)
	var facts []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &facts)
	if len(facts) != 1 {
		t.Fatalf("memory facts = %v", facts)
	}

	req := httptest.NewRequest("DELETE", "/api/runyx/memory/"+mem.ID, nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("delete memory: status %d", rec2.Code)
	}

	rec = doJSON(t, mux, "GET", "/api/runyx/memory?project_id="+proj.ID, nil)
	json.Unmarshal(rec.Body.Bytes(), &facts)
	if len(facts) != 0 {
		t.Errorf("expected memory gone after delete, got %v", facts)
	}
}

func TestWebSearchOverHTTPRealNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	mux := newKnowledgeMux(t)
	var rec *httptest.ResponseRecorder
	for attempt := 0; attempt < 4; attempt++ {
		rec = doJSON(t, mux, "POST", "/api/runyx/websearch", map[string]any{"Query": "golang programming", "MaxResults": 3})
		if rec.Code == http.StatusOK {
			var results []map[string]any
			json.Unmarshal(rec.Body.Bytes(), &results)
			if len(results) > 0 {
				return
			}
		}
	}
	t.Fatalf("websearch: status %d, body %s", rec.Code, rec.Body.String())
}

func TestContextGaugeOverHTTP(t *testing.T) {
	mux := newKnowledgeMux(t)
	rec := doJSON(t, mux, "POST", "/api/runyx/context/gauge", map[string]any{
		"Messages":     []map[string]string{{"Role": "user", "Content": "bonjour"}},
		"BudgetTokens": 1000,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("gauge: status %d, body %s", rec.Code, rec.Body.String())
	}
	var gauge map[string]any
	json.Unmarshal(rec.Body.Bytes(), &gauge)
	if gauge["NeedsCompress"] != false {
		t.Errorf("gauge = %v, expected NeedsCompress=false for a tiny conversation", gauge)
	}
}
