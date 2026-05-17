package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func main() {
	cfg := loadCfg()

	a := app.NewWithID("FlashFind")
	a.Settings().SetTheme(theme.DarkTheme())

	w := a.NewWindow("FlashFind")
	w.Resize(fyne.NewSize(cfg.W, cfg.H))

	search := widget.NewEntry()
	search.SetPlaceHolder("Search terms, \"exact phrase\", or regex")

	exclude := widget.NewEntry()
	exclude.SetPlaceHolder("Optional exclude regex")

	folder := widget.NewEntry()
	folder.SetText(cfg.LastFolder)
	folder.SetPlaceHolder("Folder to scan")

	mode := widget.NewRadioGroup([]string{"AND", "OR", "REGEX"}, nil)
	mode.Horizontal = true
	mode.Required = true
	mode.SetSelected("AND")

	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		showFolderPicker(folder.Text, w, func(path string) {
			folder.SetText(path)
		})
	})
	browse.Importance = widget.LowImportance

	searchBtn := widget.NewButtonWithIcon("Search", theme.SearchIcon(), nil)
	searchBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), nil)
	clearBtn.Importance = widget.LowImportance
	clearBtn.Disable()

	field := func(label string, control fyne.CanvasObject) fyne.CanvasObject {
		lbl := widget.NewLabel(label)
		lbl.TextStyle = fyne.TextStyle{Bold: true}
		return container.NewVBox(lbl, control)
	}

	folderControl := container.NewBorder(nil, nil, nil, browse, folder)
	top := container.NewPadded(container.NewVBox(
		container.NewGridWithColumns(3,
			field("Search", search),
			field("Exclude", exclude),
			field("Folder", folderControl),
		),
		container.NewHBox(
			widget.NewLabel("Match"),
			mode,
			layout.NewSpacer(),
			clearBtn,
			searchBtn,
		),
		widget.NewSeparator(),
	))

	var paginatedData *paginatedResults
	paginationControls, resultsScroll, updatePaginationDisplay, clearPaginationDisplay := createPaginationControls(&paginatedData, w)

	statusT := widget.NewLabel("Ready")
	statusT.TextStyle = fyne.TextStyle{Bold: true}

	fileCountT := widget.NewLabel("0 files scanned")
	spin := widget.NewProgressBarInfinite()
	spin.Hide()

	btm := container.NewBorder(
		nil,
		nil,
		statusT,
		container.NewHBox(fileCountT, spin),
		paginationControls,
	)

	w.SetContent(container.NewBorder(top, btm, nil, nil, resultsScroll))

	var activeSearchID atomic.Int64
	var cancelSearch context.CancelFunc

	isCurrentSearch := func(id int64) bool {
		return activeSearchID.Load() == id
	}

	setSearchButton := func(searching bool) {
		if searching {
			searchBtn.SetText("Stop")
			searchBtn.SetIcon(theme.MediaStopIcon())
			searchBtn.Importance = widget.DangerImportance
			searchBtn.Refresh()
			spin.Show()
			return
		}

		searchBtn.SetText("Search")
		searchBtn.SetIcon(theme.SearchIcon())
		searchBtn.Importance = widget.HighImportance
		searchBtn.Refresh()
		spin.Hide()
	}

	stopActiveSearch := func(status string) {
		if cancelSearch != nil {
			cancelSearch()
			cancelSearch = nil
		}
		activeSearchID.Add(1)
		setSearchButton(false)
		statusT.SetText(status)
	}

	clearResults := func() {
		if cancelSearch != nil {
			stopActiveSearch("Stopped")
		}
		paginatedData = nil
		clearPaginationDisplay("No results")
		fileCountT.SetText("0 files scanned")
		statusT.SetText("Ready")
		clearBtn.Disable()
	}

	cancelForInputChange := func() {
		if cancelSearch == nil {
			return
		}

		stopActiveSearch("Stopped - search changed")
		paginatedData = nil
		clearPaginationDisplay("No results")
		fileCountT.SetText("0 files scanned")
	}

	applyResults := func(id int64, results []searchResult) {
		if !isCurrentSearch(id) {
			return
		}

		if paginatedData == nil {
			paginatedData = &paginatedResults{
				results:     results,
				currentPage: 0,
				pageSize:    defaultPageSize,
				totalCount:  len(results),
			}
		} else {
			paginatedData.results = results
			paginatedData.totalCount = len(results)
		}

		updatePaginationDisplay()
	}

	startSearch := func() {
		if strings.TrimSpace(folder.Text) == "" {
			dialog.ShowInformation("FlashFind", "Select a folder", w)
			return
		}

		query := strings.TrimSpace(search.Text)
		if query == "" {
			dialog.ShowInformation("FlashFind", "Enter search text", w)
			return
		}

		searchMode := mode.Selected
		set, err := compileTokens(query, searchMode)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		var excl *regexp.Regexp
		if strings.TrimSpace(exclude.Text) != "" {
			excl, err = regexp.Compile(exclude.Text)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
		}

		root := folder.Text
		cfg.LastFolder = root
		saveCfg(cfg)

		searchID := activeSearchID.Add(1)
		ctx, cancel := context.WithCancel(context.Background())
		cancelSearch = cancel

		paginatedData = nil
		clearPaginationDisplay("Searching...")
		clearBtn.Enable()
		fileCountT.SetText("0 files scanned")
		statusT.SetText("Scanning")
		setSearchButton(true)

		resultsChan := make(chan searchResult, 128)
		collectorDone := make(chan int, 1)

		var fileUpdateQueued atomic.Bool
		var latestFileCount atomic.Int64
		scheduleFileCount := func(files int) {
			latestFileCount.Store(int64(files))
			if !fileUpdateQueued.CompareAndSwap(false, true) {
				return
			}

			fyne.Do(func() {
				defer fileUpdateQueued.Store(false)
				if !isCurrentSearch(searchID) {
					return
				}
				fileCountT.SetText(fmt.Sprintf("%d files scanned", latestFileCount.Load()))
			})
		}

		var renderQueued atomic.Bool
		scheduleResults := func(results []searchResult) {
			if !renderQueued.CompareAndSwap(false, true) {
				return
			}

			snapshot := append([]searchResult(nil), results...)
			fyne.Do(func() {
				defer renderQueued.Store(false)
				if ctx.Err() != nil || !isCurrentSearch(searchID) {
					return
				}
				applyResults(searchID, snapshot)
				statusT.SetText(fmt.Sprintf("Scanning - %d results", len(snapshot)))
			})
		}

		go func() {
			allResults := make([]searchResult, 0, 64)
			pending := make([]searchResult, 0, 32)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			defer func() {
				collectorDone <- len(allResults)
			}()

			flush := func(force bool) {
				if len(pending) == 0 && !force {
					return
				}
				if len(pending) > 0 {
					allResults = append(allResults, pending...)
					pending = pending[:0]
				}
				if force {
					snapshot := append([]searchResult(nil), allResults...)
					fyne.Do(func() {
						if ctx.Err() != nil || !isCurrentSearch(searchID) {
							return
						}
						applyResults(searchID, snapshot)
					})
					return
				}
				scheduleResults(allResults)
			}

			for {
				select {
				case result, ok := <-resultsChan:
					if !ok {
						flush(true)
						return
					}
					pending = append(pending, result)
					if len(pending) >= 25 {
						flush(false)
					}
				case <-ticker.C:
					flush(false)
				case <-ctx.Done():
					return
				}
			}
		}()

		go func() {
			files, err := runSearch(ctx, root, set, excl, resultsChan, scheduleFileCount)
			close(resultsChan)
			totalResults := <-collectorDone

			if !isCurrentSearch(searchID) {
				return
			}

			fyne.Do(func() {
				if !isCurrentSearch(searchID) {
					return
				}

				cancelSearch = nil
				setSearchButton(false)
				fileCountT.SetText(fmt.Sprintf("%d files scanned", files))

				switch {
				case errors.Is(err, context.Canceled):
					statusT.SetText("Stopped")
				case err != nil:
					statusT.SetText("Search failed")
					dialog.ShowError(err, w)
				default:
					statusT.SetText(fmt.Sprintf("Ready - %d results", totalResults))
				}
			})
		}()
	}

	searchBtn.OnTapped = func() {
		if cancelSearch != nil {
			stopActiveSearch("Stopped")
			return
		}
		startSearch()
	}

	clearBtn.OnTapped = clearResults

	search.OnSubmitted = func(_ string) {
		if cancelSearch != nil {
			stopActiveSearch("Stopped")
			return
		}
		startSearch()
	}

	search.OnChanged = func(_ string) {
		cancelForInputChange()
	}

	exclude.OnChanged = func(_ string) {
		cancelForInputChange()
	}

	folder.OnChanged = func(newFolder string) {
		cfg.LastFolder = newFolder
		saveCfg(cfg)
		cancelForInputChange()
	}

	w.ShowAndRun()

	size := w.Canvas().Size()
	cfg.W, cfg.H = size.Width, size.Height
	cfg.LastFolder = folder.Text
	saveCfg(cfg)
}
