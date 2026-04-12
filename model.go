package main

type DeckName string

type Card struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
