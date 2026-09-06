package storage

import (
	"encoding/json"
	"testing"
)

func TestChats_saveAndLoad(t *testing.T) {
	n := openTest(t)
	msg, err := json.Marshal(map[string]string{"role": "user", "content": "привет"})
	if err != nil {
		t.Fatal(err)
	}

	store := ChatStore{
		ActiveID: "c1",
		Chats: []Chat{{
			ID:       "c1",
			Title:    "привет",
			Messages: []json.RawMessage{msg},
		}},
	}
	if err := n.SaveChats(store); err != nil {
		t.Fatal(err)
	}

	got, err := n.LoadChats()
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveID != "c1" || len(got.Chats) != 1 || got.Chats[0].Title != "привет" {
		t.Fatalf("store = %+v", got)
	}
	if string(got.Chats[0].Messages[0]) == "" {
		t.Fatal("message dropped")
	}
}

func TestChats_loadMissing(t *testing.T) {
	n := openTest(t)
	got, err := n.LoadChats()
	if err != nil || len(got.Chats) != 0 {
		t.Fatalf("empty = %+v, err = %v", got, err)
	}
}

func TestChats_hiddenFromNotesList(t *testing.T) {
	n := openTest(t)
	if err := n.SaveChats(ChatStore{
		ActiveID: "c1",
		Chats:    []Chat{{ID: "c1", Title: "x"}},
	}); err != nil {
		t.Fatal(err)
	}
	list, err := n.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("chats leaked into notes: %+v", list)
	}
}

func TestChats_overwrite(t *testing.T) {
	n := openTest(t)
	first := ChatStore{ActiveID: "a", Chats: []Chat{{ID: "a", Title: "один"}}}
	second := ChatStore{ActiveID: "b", Chats: []Chat{{ID: "b", Title: "два"}}}
	if err := n.SaveChats(first); err != nil {
		t.Fatal(err)
	}
	if err := n.SaveChats(second); err != nil {
		t.Fatal(err)
	}
	got, err := n.LoadChats()
	if err != nil || got.ActiveID != "b" || len(got.Chats) != 1 || got.Chats[0].Title != "два" {
		t.Fatalf("store = %+v, err = %v", got, err)
	}
}

func TestChats_rejectBadID(t *testing.T) {
	n := openTest(t)
	err := n.SaveChats(ChatStore{Chats: []Chat{{ID: "../x", Title: "x"}}})
	if err != ErrInvalidChats {
		t.Fatalf("err = %v", err)
	}
}
