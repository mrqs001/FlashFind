package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const defaultPageSize = 10

// clickableFileResult creates a clickable widget for a file result
type clickableFileResult struct {
	widget.Card
	filePath string
	onTapped func(string)
	window   fyne.Window
}

func newClickableFileResult(fileName, filePath string, showPath bool, hits []hit, window fyne.Window) *clickableFileResult {
	cfr := &clickableFileResult{
		filePath: filePath,
		window:   window,
	}

	var content []fyne.CanvasObject

	titleLabel := widget.NewLabel(fileName)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	matchLabel := widget.NewLabel(fmt.Sprintf("%d matches", len(hits)))
	matchLabel.Alignment = fyne.TextAlignTrailing

	header := container.NewHBox(
		widget.NewIcon(theme.FileTextIcon()),
		titleLabel,
		layout.NewSpacer(),
		matchLabel,
	)
	content = append(content, header)

	if showPath {
		relPath := filePath
		if len(relPath) > 80 {
			relPath = "..." + relPath[len(relPath)-77:]
		}
		pathLabel := widget.NewLabel(relPath)
		pathLabel.TextStyle = fyne.TextStyle{Italic: true}
		content = append(content, pathLabel)
	}

	maxHitsToShow := 3
	hitsToShow := hits
	if len(hitsToShow) > maxHitsToShow {
		hitsToShow = hitsToShow[:maxHitsToShow]
	}

	for _, hit := range hitsToShow {
		cleanLine := strings.TrimSpace(hit.text)
		if len(cleanLine) > 240 {
			cleanLine = cleanLine[:240] + "..."
		}

		hitLabel := widget.NewLabel(fmt.Sprintf("Line %d: %s", hit.line, cleanLine))
		hitLabel.Wrapping = fyne.TextWrapWord
		content = append(content, hitLabel)
	}

	// Show truncation notice if there are more hits
	if len(hits) > maxHitsToShow {
		truncLabel := widget.NewLabel(fmt.Sprintf("... and %d more matches", len(hits)-maxHitsToShow))
		truncLabel.TextStyle = fyne.TextStyle{Italic: true}
		content = append(content, truncLabel)
	}

	// Create the card with all content
	cfr.Card = *widget.NewCard("", "", container.NewVBox(content...))
	cfr.ExtendBaseWidget(cfr)

	return cfr
}

func (cfr *clickableFileResult) Tapped(ev *fyne.PointEvent) {
	if cfr.onTapped != nil {
		cfr.onTapped(cfr.filePath)
	}
}

func (cfr *clickableFileResult) DoubleTapped(ev *fyne.PointEvent) {
	cfr.Tapped(ev)
}

func (cfr *clickableFileResult) TappedSecondary(ev *fyne.PointEvent) {
	cfr.showContextMenu(ev)
}

func (cfr *clickableFileResult) showContextMenu(ev *fyne.PointEvent) {
	fileName := filepath.Base(cfr.filePath)
	folderPath := filepath.Dir(cfr.filePath)

	copyFileNameItem := fyne.NewMenuItem("Copy File Name", func() {
		cfr.window.Clipboard().SetContent(fileName)
	})
	copyFileNameItem.Icon = theme.ContentCopyIcon()

	copyFilePathItem := fyne.NewMenuItem("Copy File Path", func() {
		cfr.window.Clipboard().SetContent(cfr.filePath)
	})
	copyFilePathItem.Icon = theme.ContentCopyIcon()

	copyFolderPathItem := fyne.NewMenuItem("Copy Folder Path", func() {
		cfr.window.Clipboard().SetContent(folderPath)
	})
	copyFolderPathItem.Icon = theme.FolderIcon()

	openFolderItem := fyne.NewMenuItem("Open Folder", func() {
		if cfr.onTapped != nil {
			cfr.onTapped(cfr.filePath)
		}
	})
	openFolderItem.Icon = theme.FolderOpenIcon()

	menu := fyne.NewMenu("",
		copyFileNameItem,
		copyFilePathItem,
		copyFolderPathItem,
		fyne.NewMenuItemSeparator(),
		openFolderItem,
	)

	widget.ShowPopUpMenuAtPosition(menu, cfr.window.Canvas(), ev.AbsolutePosition)
}

type paginatedResults struct {
	results     []searchResult
	currentPage int
	pageSize    int
	totalCount  int
}

func (pr *paginatedResults) getCurrentPageResults() []searchResult {
	start := pr.currentPage * pr.pageSize
	end := start + pr.pageSize
	if end > len(pr.results) {
		end = len(pr.results)
	}
	if start >= len(pr.results) {
		return []searchResult{}
	}
	return pr.results[start:end]
}

func (pr *paginatedResults) getTotalPages() int {
	if pr.pageSize == 0 {
		return 0
	}
	return (len(pr.results) + pr.pageSize - 1) / pr.pageSize
}

func (pr *paginatedResults) hasNextPage() bool {
	return pr.currentPage < pr.getTotalPages()-1
}

func (pr *paginatedResults) hasPrevPage() bool {
	return pr.currentPage > 0
}

func createPaginationControls(paginatedData **paginatedResults, window fyne.Window) (*fyne.Container, *container.Scroll, func(), func(string)) {
	pageInfo := widget.NewLabel("No results")
	prevBtn := widget.NewButtonWithIcon("Previous", theme.NavigateBackIcon(), nil)
	nextBtn := widget.NewButtonWithIcon("Next", theme.NavigateNextIcon(), nil)

	// Page size selector
	pageSizeSelect := widget.NewSelect([]string{"10", "20", "50", "100"}, nil)
	pageSizeSelect.SetSelected(strconv.Itoa(defaultPageSize))

	// Show/Hide paths toggle
	showPathsCheck := widget.NewCheck("Show paths", nil)
	showPathsCheck.SetChecked(false) // Default to hidden

	// Create a container for clickable results
	resultsContainer := container.NewVBox()
	resultsScroll := container.NewScroll(resultsContainer)
	resultsScroll.SetMinSize(fyne.NewSize(800, 400))

	prevBtn.Disable()
	nextBtn.Disable()

	clearDisplay := func(message string) {
		pageInfo.SetText("No results")
		prevBtn.Disable()
		nextBtn.Disable()
		resultsContainer.Objects = nil
		if message != "" {
			emptyLabel := widget.NewLabel(message)
			emptyLabel.Alignment = fyne.TextAlignCenter
			resultsContainer.Add(container.NewCenter(emptyLabel))
		}
		resultsContainer.Refresh()
	}

	clearDisplay("No results")

	// Function to update pagination display with better performance
	updatePaginationDisplay := func() {
		if *paginatedData == nil || len((*paginatedData).results) == 0 {
			clearDisplay("No results")
			return
		}

		totalPages := (*paginatedData).getTotalPages()
		totalResults := len((*paginatedData).results)
		pageInfo.SetText(fmt.Sprintf("Page %d of %d (%d total results)",
			(*paginatedData).currentPage+1, totalPages, totalResults))

		if (*paginatedData).hasPrevPage() {
			prevBtn.Enable()
		} else {
			prevBtn.Disable()
		}

		if (*paginatedData).hasNextPage() {
			nextBtn.Enable()
		} else {
			nextBtn.Disable()
		}

		// Display current page results with clickable widgets
		currentResults := (*paginatedData).getCurrentPageResults()

		// Clear the results container
		resultsContainer.Objects = nil

		if len(currentResults) == 0 {
			noResultsLabel := widget.NewLabel("No results to display")
			resultsContainer.Add(noResultsLabel)
		} else {
			for _, result := range currentResults {
				fileName := filepath.Base(result.filePath)

				// Create clickable file result widget
				clickableResult := newClickableFileResult(
					fileName,
					result.filePath,
					showPathsCheck.Checked,
					result.hits,
					window,
				)

				clickableResult.onTapped = func(filePath string) {
					if err := openFileOrFolder(filePath, false); err != nil {
						dialog.ShowError(err, window)
					}
				}

				resultsContainer.Add(clickableResult)
			}
		}

		resultsContainer.Refresh()
	}

	// Pagination button handlers
	prevBtn.OnTapped = func() {
		if *paginatedData != nil && (*paginatedData).hasPrevPage() {
			(*paginatedData).currentPage--
			updatePaginationDisplay()
		}
	}

	nextBtn.OnTapped = func() {
		if *paginatedData != nil && (*paginatedData).hasNextPage() {
			(*paginatedData).currentPage++
			updatePaginationDisplay()
		}
	}

	// Configure page size selector callback
	pageSizeSelect.OnChanged = func(value string) {
		if *paginatedData != nil {
			if pageSize, err := strconv.Atoi(value); err == nil {
				(*paginatedData).pageSize = pageSize
				(*paginatedData).currentPage = 0 // Reset to first page
				updatePaginationDisplay()
			}
		}
	}

	// Configure show paths checkbox callback
	showPathsCheck.OnChanged = func(checked bool) {
		updatePaginationDisplay()
	}

	// Pagination controls container
	paginationControls := container.NewHBox(
		prevBtn,
		pageInfo,
		nextBtn,
		widget.NewSeparator(),
		widget.NewLabel("Per page:"),
		pageSizeSelect,
		widget.NewSeparator(),
		showPathsCheck,
	)

	return paginationControls, resultsScroll, updatePaginationDisplay, clearDisplay
}
