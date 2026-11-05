package main

import "testing"

func mustLoad(t *testing.T, path string) Scene {
	t.Helper()
	s, err := LoadScene(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustStyle(t *testing.T, path string) StyleSample {
	t.Helper()
	s, err := loadStyle(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func factText(s Scene, id string) string {
	f, _ := s.FactByID(id)
	return f.Text
}
