package app

import (
	"log"
	"os"
)

func Main() {
	log.SetFlags(0)

	if len(os.Args) == 1 {
		if isTerminal(os.Stdin) {
			runBridge(nil)
			return
		}
		runMCPServer()
		return
	}

	switch os.Args[1] {
	case "serve":
		runMCPServer()
	case "bridge":
		runBridge(os.Args[2:])
	case "setup":
		setup(os.Args[2:])
	case "xiaozhi-store-url":
		xiaoZhiStoreURL(os.Args[2:])
	case "linear-store-api-key":
		linearStoreAPIKey(os.Args[2:])
	case "resolve":
		resolve(os.Args[2:])
	case "start":
		startIssueWorkCommand(os.Args[2:])
	case "finish":
		finishIssueWork(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
