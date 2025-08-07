package main

import (
	"fmt"
	"log"
	"os"

	"hyperbyte-logs/internal/tlog"
	"hyperbyte-logs/internal/ui"
)

// Implementation moved to internal packages `tlog` and `ui`.

var defaultLogPaths = []string{
	"/var/log/syslog",
	"/var/log/messages",
	"/var/log/kern.log",
	"/var/log/dmesg",
	"/var/log/auth.log",
	"/var/log/system.log",
	"/var/log/daemon.log",
}

func main() {
	files := os.Args[1:]
	if len(files) == 0 {
		fmt.Println("No files specified, using default system logs...")
		for _, f := range defaultLogPaths {
			if _, err := os.Stat(f); err == nil {
				files = append(files, f)
			}
		}
		if len(files) == 0 {
			fmt.Println("No readable default log files found.")
			os.Exit(1)
		}
	}

	tailers := make([]*tlog.Tailer, len(files))
	for i, file := range files {
		t, err := tlog.NewTailer(file)
		if err != nil {
			log.Fatalf("Failed to tail %s: %v", file, err)
		}
		tailers[i] = t
	}

	application := ui.NewAppUI(tailers)
	if err := application.Run(); err != nil {
		log.Fatalf("UI error: %v", err)
	}
}
