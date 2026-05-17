package main

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
)

var errSearchCancelled = errors.New("search cancelled")

func runSearch(ctx context.Context, root string, set *tokenSet, exclude *regexp.Regexp, results chan<- searchResult, progress func(int)) (int, error) {
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	pool := make(chan struct{}, workers)
	var wg sync.WaitGroup
	files := 0

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return errSearchCancelled
		}
		if err != nil || d.IsDir() {
			return nil
		}
		if exclude != nil && exclude.MatchString(path) {
			return nil
		}

		files++
		if progress != nil && files%100 == 0 {
			progress(files)
		}

		select {
		case <-ctx.Done():
			return errSearchCancelled
		case pool <- struct{}{}:
		}

		wg.Add(1)
		go func(p string) {
			defer func() {
				<-pool
				wg.Done()
			}()

			hits := scanFile(ctx, p, set)
			if len(hits) == 0 {
				return
			}

			select {
			case <-ctx.Done():
				return
			case results <- searchResult{filePath: p, hits: hits}:
			}
		}(path)

		return nil
	})

	wg.Wait()

	if ctx.Err() != nil || errors.Is(walkErr, errSearchCancelled) {
		return files, context.Canceled
	}

	return files, walkErr
}
