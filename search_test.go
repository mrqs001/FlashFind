package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestShlexKeepsQuotedPhrases(t *testing.T) {
	got := shlex(`alpha "beta gamma" delta`)
	want := []string{"alpha", "beta gamma", "delta"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCompileTokensRejectsInvalidRegex(t *testing.T) {
	if _, err := compileTokens("[", "REGEX"); err == nil {
		t.Fatal("expected invalid regex to fail")
	}
}

func TestScanFileANDMatchesTermsAcrossLines(t *testing.T) {
	path := writeTestFile(t, "first line has alpha\nsecond line has beta\n")
	set, err := compileTokens("alpha beta", "AND")
	if err != nil {
		t.Fatal(err)
	}

	hits := scanFile(context.Background(), path, set)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2", len(hits))
	}
}

func TestScanFileORMatchesAnyTerm(t *testing.T) {
	path := writeTestFile(t, "first line\nsecond beta line\n")
	set, err := compileTokens("alpha beta", "OR")
	if err != nil {
		t.Fatal(err)
	}

	hits := scanFile(context.Background(), path, set)
	if len(hits) != 1 || hits[0].line != 2 {
		t.Fatalf("got hits %#v, want one hit on line 2", hits)
	}
}

func TestRunSearchHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := make(chan searchResult)
	files, err := runSearch(ctx, t.TempDir(), &tokenSet{modeAND: true, tokens: []string{"alpha"}}, nil, results, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got err %v, want context.Canceled", err)
	}
	if files != 0 {
		t.Fatalf("got %d files, want 0", files)
	}
}

func TestRunSearchAppliesExcludePattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.log"), []byte("alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := make(chan searchResult, 2)
	closeAfterSearch := make(chan struct{})
	go func() {
		defer close(closeAfterSearch)
		files, err := runSearch(context.Background(), dir, &tokenSet{modeAND: true, tokens: []string{"alpha"}}, regexp.MustCompile(`\.log$`), results, nil)
		close(results)
		if err != nil {
			t.Errorf("runSearch returned error: %v", err)
		}
		if files != 1 {
			t.Errorf("got %d scanned files, want 1", files)
		}
	}()

	<-closeAfterSearch
	var got []searchResult
	for result := range results {
		got = append(got, result)
	}
	if len(got) != 1 || filepath.Base(got[0].filePath) != "keep.txt" {
		t.Fatalf("got results %#v, want keep.txt only", got)
	}
}

func writeTestFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
