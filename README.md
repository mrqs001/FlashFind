# FlashFind

FlashFind is a small cross-platform desktop search app built with Go and Fyne. It scans a folder, streams matching files into a paginated result list, and keeps long searches cancellable.

## Features

- AND, OR, and REGEX search modes
- Optional exclude regex
- Cancellable background search
- Progressive, paginated results
- Native OS folder picker where available, with a Fyne fallback
- Click a result to reveal it in the file manager
- Right-click result actions for copying file names and paths
- Persistent window size and last selected folder

## Requirements

- Go 1.24 or newer
- A desktop environment supported by Fyne
- Linux users may need standard Fyne build dependencies for OpenGL/GLFW
- Optional Linux native folder picker tools: `zenity`, `yad`, `kdialog`, or `qarma`

If none of the native folder picker tools are available, FlashFind falls back to Fyne's built-in folder dialog.

## Build

```bash
make build
```

For a smaller release binary:

```bash
make release
```

Without `make`:

```bash
go build -o flashfind
```

## Run

```bash
make run
```

Or after building:

```bash
./flashfind
```

## Usage

1. Select a folder.
2. Enter search terms.
3. Choose `AND`, `OR`, or `REGEX`.
4. Optionally add an exclude regex.
5. Click `Search` or press Enter.
6. Click `Stop` to cancel an active search.
7. Use `Clear` to reset the result list.

Results are paginated. Click a result to reveal it in your file manager, or right-click it for copy options.

## Search Behavior

- `AND`: all terms must exist somewhere in the file.
- `OR`: any term can match a line.
- `REGEX`: regular expression matching is case-insensitive.
- Files larger than 50 MB are skipped.
- Up to 10,000 lines are scanned per file.
- Up to 100 hits are retained per file.

Quoted phrases are supported, for example:

```text
"exact phrase" anotherTerm
```

## Configuration

FlashFind stores user settings in:

```text
~/.flashfind.json
```

The config currently stores window size and the last selected folder.

## Development

```bash
make fmt
make test
make vet
```

The source is split by responsibility:

- `main.go`: app wiring and UI state
- `search.go`: token parsing and file scanning
- `search_runner.go`: cancellable directory walking and concurrent execution
- `pagination.go`: result display and pagination controls
- `folder_picker.go`: native folder picker integration
- `file_open.go`: cross-platform file manager integration
- `config.go`: persistent settings
