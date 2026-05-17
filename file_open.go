package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// openFileOrFolder opens a file or reveals it in the system file manager.
func openFileOrFolder(filePath string, openFile bool) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		if openFile {
			cmd = exec.Command("cmd", "/c", "start", "", filePath)
		} else {
			cmd = exec.Command("explorer", "/select,", filePath)
		}
	case "darwin":
		if openFile {
			cmd = exec.Command("open", filePath)
		} else {
			cmd = exec.Command("open", "-R", filePath)
		}
	case "linux":
		if openFile {
			cmd = exec.Command("xdg-open", filePath)
		} else {
			folderPath := filepath.Dir(filePath)

			if _, err := exec.LookPath("nautilus"); err == nil {
				cmd = exec.Command("nautilus", "--select", filePath)
			} else if _, err := exec.LookPath("dolphin"); err == nil {
				cmd = exec.Command("dolphin", "--select", filePath)
			} else if _, err := exec.LookPath("thunar"); err == nil {
				cmd = exec.Command("thunar", folderPath)
			} else if _, err := exec.LookPath("pcmanfm"); err == nil {
				cmd = exec.Command("pcmanfm", folderPath)
			} else {
				cmd = exec.Command("xdg-open", folderPath)
			}
		}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return cmd.Start()
}
