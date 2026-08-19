package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestAccountsStoreDisplayOrderPersistsWithoutChangingPositions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	store := newAccountsStore(path)
	store.Accounts = []account{
		{ID: 10, Name: "first", Token: "token-1"},
		{ID: 20, Name: "second", Token: "token-2"},
		{ID: 30, Name: "third", Token: "token-3"},
	}
	store.LastUsed = 1
	if err := store.SetDisplayOrder([]int{30, 10, 20}); err != nil {
		t.Fatalf("SetDisplayOrder: %v", err)
	}

	loaded := newAccountsStore(path)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	list := loaded.List()
	gotStable := []int{list[0].StableID, list[1].StableID, list[2].StableID}
	gotPositions := []int{list[0].Num, list[1].Num, list[2].Num}
	if !reflect.DeepEqual(gotStable, []int{30, 10, 20}) {
		t.Fatalf("stable display order = %v", gotStable)
	}
	if !reflect.DeepEqual(gotPositions, []int{3, 1, 2}) {
		t.Fatalf("operational positions changed: %v", gotPositions)
	}
	token, _, _, _, ok := loaded.GetAccountByIndex(1)
	if !ok || token != "token-1" {
		t.Fatalf("position 1 token changed: ok=%v token=%q", ok, token)
	}
}

func TestAccountsStoreDisplayOrderValidationAndAdd(t *testing.T) {
	store := newAccountsStore(filepath.Join(t.TempDir(), "accounts.json"))
	store.Accounts = []account{{ID: 4}, {ID: 7}}
	store.DisplayOrder = []int{4, 7}
	if err := store.SetDisplayOrder([]int{4, 4}); err == nil {
		t.Fatal("duplicate stable IDs should be rejected")
	}
	if err := store.SetDisplayOrder([]int{4}); err == nil {
		t.Fatal("incomplete order should be rejected")
	}
	if _, err := store.Add("", "token", ""); err != nil {
		t.Fatalf("Add: %v", err)
	}
	list := store.List()
	if got := list[len(list)-1].StableID; got != 8 {
		t.Fatalf("new stable ID should be appended, got %d", got)
	}
}
