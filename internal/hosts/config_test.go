package hosts

import "testing"

func TestAppendLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mustAppend(t, Node{Name: "alpha", Address: "10.0.0.1", User: "root"})

	nodes, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "alpha" || nodes[0].Address != "10.0.0.1" {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}
}

func TestUpdateReplacesFieldsAndAllowsRename(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mustAppend(t, Node{Name: "alpha", Address: "10.0.0.1"})
	mustAppend(t, Node{Name: "beta", Address: "10.0.0.2"})

	updated := Node{Name: "alpha2", Address: "10.0.0.9", User: "misael", Port: 2222, OS: "linux"}
	if err := Update("alpha", updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	nodes, _ := Load()
	got := findByName(nodes, "alpha2")
	if got == nil {
		t.Fatalf("renamed node not found: %+v", nodes)
	}
	if got.Address != "10.0.0.9" || got.User != "misael" || got.Port != 2222 || got.OS != "linux" {
		t.Fatalf("fields not updated: %+v", got)
	}
	if findByName(nodes, "alpha") != nil {
		t.Fatalf("old name still present after rename")
	}
	if findByName(nodes, "beta") == nil {
		t.Fatalf("unrelated node lost")
	}
}

func TestUpdateNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mustAppend(t, Node{Name: "alpha", Address: "10.0.0.1"})

	if err := Update("ghost", Node{Name: "ghost", Address: "x"}); err == nil {
		t.Fatalf("expected error updating a missing host")
	}
}

func TestDeleteRemovesEntry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mustAppend(t, Node{Name: "alpha", Address: "10.0.0.1"})
	mustAppend(t, Node{Name: "beta", Address: "10.0.0.2"})

	if err := Delete("alpha"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	nodes, _ := Load()
	if len(nodes) != 1 || nodes[0].Name != "beta" {
		t.Fatalf("unexpected nodes after delete: %+v", nodes)
	}
}

func TestDeleteNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mustAppend(t, Node{Name: "alpha", Address: "10.0.0.1"})

	if err := Delete("ghost"); err == nil {
		t.Fatalf("expected error deleting a missing host")
	}
}

func mustAppend(t *testing.T, n Node) {
	t.Helper()
	if err := Append(n); err != nil {
		t.Fatalf("Append(%s): %v", n.Name, err)
	}
}

func findByName(nodes []Node, name string) *Node {
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
	}
	return nil
}
