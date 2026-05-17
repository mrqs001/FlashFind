package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

func showFolderPicker(current string, parent fyne.Window, picked func(string)) {
	if showNativeFolderPicker(current, picked) {
		return
	}

	dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, parent)
			return
		}
		if u != nil {
			picked(u.Path())
		}
	}, parent)
}

func showNativeFolderPicker(current string, picked func(string)) bool {
	cmd, ok := nativeFolderPickerCommand(current)
	if !ok {
		return false
	}

	go func() {
		out, err := cmd.Output()
		if err != nil && current != "" {
			if retry, ok := nativeFolderPickerCommand(""); ok {
				out, err = retry.Output()
			}
		}
		if err != nil {
			return
		}

		path := strings.TrimSpace(string(out))
		if path == "" {
			return
		}

		fyne.Do(func() {
			picked(path)
		})
	}()

	return true
}

func nativeFolderPickerCommand(current string) (*exec.Cmd, bool) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("osascript"); err != nil {
			return nil, false
		}
		script := `POSIX path of (choose folder with prompt "Select folder")`
		if current != "" {
			script = fmt.Sprintf(`POSIX path of (choose folder with prompt "Select folder" default location POSIX file "%s")`, appleScriptQuote(current))
		}
		return exec.Command("osascript", "-e", script), true
	case "linux":
		if path, err := exec.LookPath("zenity"); err == nil {
			args := []string{"--file-selection", "--directory", "--title", "Select folder"}
			if current != "" {
				args = append(args, "--filename", current+"/")
			}
			return exec.Command(path, args...), true
		}
		if path, err := exec.LookPath("yad"); err == nil {
			args := []string{"--file-selection", "--directory", "--title", "Select folder"}
			if current != "" {
				args = append(args, "--filename", current+"/")
			}
			return exec.Command(path, args...), true
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			args := []string{"--getexistingdirectory"}
			if current != "" {
				args = append(args, current)
			}
			args = append(args, "Select folder")
			return exec.Command(path, args...), true
		}
		if path, err := exec.LookPath("qarma"); err == nil {
			args := []string{"--file-selection", "--directory", "--title", "Select folder"}
			if current != "" {
				args = append(args, "--filename", current+"/")
			}
			return exec.Command(path, args...), true
		}
	case "windows":
		if path, err := exec.LookPath("powershell.exe"); err == nil {
			return exec.Command(path, "-NoProfile", "-STA", "-Command", windowsFolderPickerScript(current)), true
		}
		if path, err := exec.LookPath("powershell"); err == nil {
			return exec.Command(path, "-NoProfile", "-STA", "-Command", windowsFolderPickerScript(current)), true
		}
	}

	return nil, false
}

func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

func windowsFolderPickerScript(current string) string {
	selectedPath := psSingleQuote(current)
	return fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = 'Select folder'
$dialog.ShowNewFolderButton = $true
if (%[1]s -ne '') { $dialog.SelectedPath = %[1]s }
if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  Write-Output $dialog.SelectedPath
}
`, selectedPath)
}

func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
