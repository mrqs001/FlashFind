package main

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"regexp"
	"strings"
)

type tokenSet struct {
	modeAND bool
	tokens  []string
	re      *regexp.Regexp
}

type hit struct {
	line int
	text string
}

type searchResult struct {
	filePath string
	hits     []hit
}

func compileTokens(raw, mode string) (*tokenSet, error) {
	if mode == "REGEX" {
		re, err := regexp.Compile("(?i)" + raw)
		return &tokenSet{re: re}, err
	}

	// Parse tokens with proper quote handling
	parts := shlex(raw)
	if len(parts) == 0 {
		return &tokenSet{tokens: []string{}}, nil
	}

	// Convert to lowercase for case-insensitive matching
	tokens := make([]string, len(parts))
	for i, part := range parts {
		tokens[i] = strings.ToLower(part)
	}

	if mode == "OR" {
		// Create alternation pattern, escaping each token
		escaped := make([]string, len(tokens))
		for i, token := range tokens {
			escaped[i] = regexp.QuoteMeta(token)
		}
		pattern := "(?i)(" + strings.Join(escaped, "|") + ")"
		re, err := regexp.Compile(pattern)
		return &tokenSet{re: re}, err
	}

	if mode == "AND" {
		return &tokenSet{modeAND: true, tokens: tokens}, nil
	}

	return &tokenSet{modeAND: true, tokens: tokens}, nil
}

func shlex(src string) []string {
	var res []string
	var buf bytes.Buffer
	quoted := false

	for _, r := range src {
		switch r {
		case '"':
			quoted = !quoted
		case ' ', '\t', '\n', '\r':
			if quoted {
				buf.WriteRune(r)
			} else if buf.Len() > 0 {
				res = append(res, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}

	if buf.Len() > 0 {
		res = append(res, buf.String())
	}

	return res
}

func scanFile(ctx context.Context, path string, set *tokenSet) []hit {
	if ctx.Err() != nil {
		return nil
	}

	// Check file size first to avoid reading huge files into memory
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	// Skip files larger than 50MB to avoid memory issues and improve responsiveness
	const maxFileSize = 50 * 1024 * 1024
	if info.Size() > maxFileSize {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	// For AND mode, we need to check if all terms exist anywhere in the file
	if set.modeAND {
		return scanFileAllTerms(ctx, file, set)
	}

	// For OR and REGEX modes, continue with line-by-line scanning
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	out := make([]hit, 0, 10)
	n := 0

	const maxLines = 10000
	const maxHits = 100

	for sc.Scan() && n < maxLines && len(out) < maxHits {
		n++
		originalLine := sc.Text()
		line := strings.ToLower(originalLine)

		if n%128 == 0 && ctx.Err() != nil {
			return nil
		}

		matched := set.re.MatchString(line)

		if matched {
			if len(originalLine) > 500 {
				originalLine = originalLine[:500] + "..."
			}
			out = append(out, hit{n, originalLine})
		}
	}
	return out
}

// scanFileAllTerms searches line-by-line for AND mode so cancellation remains responsive.
func scanFileAllTerms(ctx context.Context, file *os.File, set *tokenSet) []hit {
	if len(set.tokens) == 0 {
		return nil
	}

	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	out := make([]hit, 0, 10)
	found := make([]bool, len(set.tokens))
	foundCount := 0
	n := 0

	const maxLines = 10000
	const maxHits = 100

	for sc.Scan() && n < maxLines {
		n++
		originalLine := sc.Text()
		line := strings.ToLower(originalLine)

		if n%128 == 0 && ctx.Err() != nil {
			return nil
		}

		// Check if this line contains any of the search terms
		lineHasMatch := false
		for i, token := range set.tokens {
			if strings.Contains(line, token) {
				lineHasMatch = true
				if !found[i] {
					found[i] = true
					foundCount++
				}
			}
		}

		if lineHasMatch && len(out) < maxHits {
			if len(originalLine) > 500 {
				originalLine = originalLine[:500] + "..."
			}
			out = append(out, hit{n, originalLine})
		}
	}

	if foundCount != len(set.tokens) {
		return nil
	}

	return out
}
