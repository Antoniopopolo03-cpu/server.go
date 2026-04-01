package main

type SearchRequest struct {
	Query string
	Limit int
}

type CanonicalCharacter struct {
	ID             string
	Name           string
	Clan           string
	Affiliation    []string
	Sex            string
	Birthdate      string
	Classification []string
	RankPartI      string
	RankPartII     string
	RankGaiden     string
	DebutAnime     string
	DebutManga     string
	ImageURL       string
	Jutsu          []string
	Source         string
}

type CanonicalClan struct {
	ID             string
	Name           string
	CharacterCount int
	Source         string
}
