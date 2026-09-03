//go:generate go run ../tools/gen-psp-rom-list/main.go

// Package pspdb maps PSP Game IDs to canonical game titles.
//
// SLATED FOR REMOVAL, to be replaced by Argosy Sigil. Do not build on it.
//
// It is 7,903 entries of generated Go, and save sync uses it twice: to name a
// PSP save group, and in extractPSPGameID to find where the Game ID ends in a
// directory name like "UCUS98653PROFILE00". The first has a PARAM.SFO fallback;
// the second has none, so removing this before a replacement exists would key
// those saves on the wrong ID.
package pspdb
