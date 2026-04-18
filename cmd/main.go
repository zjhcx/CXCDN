package main

import (
	"flag"
	"log"
	"os"

	"cxcdn/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	mode := flag.String("mode", "hertz", "server mode: hertz")
	cacheFile := flag.String("cache-file", "./cxcdn.cache", "path to disk cache file for persistence")
	noCache := flag.Bool("no-cache", false, "disable disk cache persistence")
	flag.Parse()

	// Allow override via environment variables
	if envAddr := os.Getenv("CXCDN_ADDR"); envAddr != "" {
		*addr = envAddr
	}
	if envMode := os.Getenv("CXCDN_MODE"); envMode != "" {
		*mode = envMode
	}
	if envCache := os.Getenv("CXCDN_CACHE_FILE"); envCache != "" {
		*cacheFile = envCache
	}
	if envNoCache := os.Getenv("CXCDN_NO_CACHE"); envNoCache == "true" {
		*noCache = true
	}

	// Disable cache if requested
	if *noCache {
		*cacheFile = ""
	}

	var err error
	switch *mode {
	default:
		err = server.RunHertz(*addr, *cacheFile)
	}

	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
