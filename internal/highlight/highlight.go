package highlight

import "strings"

// ColorizeLine adds tview dynamic color tags based on common keywords.
func ColorizeLine(line string) string {
	if strings.Contains(line, "ERROR") || strings.Contains(line, "FATAL") {
		return "[red]" + line + "[-]"
	}
	if strings.Contains(line, "WARN") || strings.Contains(line, "WARNING") {
		return "[orange]" + line + "[-]"
	}
	if strings.Contains(line, "INFO") {
		return "[lightblue]" + line + "[-]"
	}
	return line
}
