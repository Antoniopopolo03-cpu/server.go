package main

import (
	"context"
	"fmt"
	"strings"
)

type NarutoProvider interface {
	Name() string
	SearchCharacters(ctx context.Context, req SearchRequest) ([]CanonicalCharacter, error)
	SearchClans(ctx context.Context, req SearchRequest) ([]CanonicalClan, error)
}

type ProviderRegistry struct {
	providers []NarutoProvider
}

func NewProviderRegistry(providers ...NarutoProvider) *ProviderRegistry {
	return &ProviderRegistry{providers: providers}
}

func (r *ProviderRegistry) SearchCharactersFirst(ctx context.Context, req SearchRequest) ([]CanonicalCharacter, string, error) {
	var errs []string
	for _, p := range r.providers {
		list, err := p.SearchCharacters(ctx, req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		if len(list) > 0 {
			return list, p.Name(), nil
		}
	}
	if len(errs) > 0 {
		return nil, "", fmt.Errorf("all providers failed: %s", strings.Join(errs, "; "))
	}
	return nil, "", nil
}

func (r *ProviderRegistry) SearchClansFirst(ctx context.Context, req SearchRequest) ([]CanonicalClan, string, error) {
	var errs []string
	for _, p := range r.providers {
		list, err := p.SearchClans(ctx, req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		if len(list) > 0 {
			return list, p.Name(), nil
		}
	}
	if len(errs) > 0 {
		return nil, "", fmt.Errorf("all providers failed: %s", strings.Join(errs, "; "))
	}
	return nil, "", nil
}
