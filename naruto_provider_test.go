package main

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	name            string
	characters      []CanonicalCharacter
	clans           []CanonicalClan
	characterErr    error
	clanErr         error
	characterCalled int
	clanCalled      int
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) SearchCharacters(ctx context.Context, req SearchRequest) ([]CanonicalCharacter, error) {
	f.characterCalled++
	return f.characters, f.characterErr
}

func (f *fakeProvider) SearchClans(ctx context.Context, req SearchRequest) ([]CanonicalClan, error) {
	f.clanCalled++
	return f.clans, f.clanErr
}

func TestProviderRegistry_SearchCharactersFirst_FallbackOrder(t *testing.T) {
	p1 := &fakeProvider{name: "p1", characterErr: errors.New("down")}
	p2 := &fakeProvider{name: "p2", characters: []CanonicalCharacter{{ID: "1", Name: "Naruto"}}}
	p3 := &fakeProvider{name: "p3", characters: []CanonicalCharacter{{ID: "2", Name: "Sasuke"}}}

	reg := NewProviderRegistry(p1, p2, p3)
	got, source, err := reg.SearchCharactersFirst(context.Background(), SearchRequest{Query: "naruto", Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != "p2" {
		t.Fatalf("expected source p2, got %q", source)
	}
	if len(got) != 1 || got[0].Name != "Naruto" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if p1.characterCalled != 1 || p2.characterCalled != 1 || p3.characterCalled != 0 {
		t.Fatalf("unexpected call counts p1=%d p2=%d p3=%d", p1.characterCalled, p2.characterCalled, p3.characterCalled)
	}
}

func TestProviderRegistry_SearchCharactersFirst_AllFail(t *testing.T) {
	p1 := &fakeProvider{name: "p1", characterErr: errors.New("timeout")}
	p2 := &fakeProvider{name: "p2", characterErr: errors.New("500")}
	reg := NewProviderRegistry(p1, p2)

	got, source, err := reg.SearchCharactersFirst(context.Background(), SearchRequest{Query: "kakashi", Limit: 2})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got != nil || source != "" {
		t.Fatalf("expected nil result and empty source, got=%v source=%q", got, source)
	}
}

func TestProviderRegistry_SearchClansFirst_ReturnsFirstNonEmpty(t *testing.T) {
	p1 := &fakeProvider{name: "p1", clans: nil}
	p2 := &fakeProvider{name: "p2", clans: []CanonicalClan{{ID: "45", Name: "Uchiha"}}}
	reg := NewProviderRegistry(p1, p2)

	got, source, err := reg.SearchClansFirst(context.Background(), SearchRequest{Query: "uchiha", Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != "p2" {
		t.Fatalf("expected source p2, got %q", source)
	}
	if len(got) != 1 || got[0].Name != "Uchiha" {
		t.Fatalf("unexpected clan result: %+v", got)
	}
}
