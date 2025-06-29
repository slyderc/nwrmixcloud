package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nowwaveradio/mixcloud-updater/internal/config"
	"github.com/nowwaveradio/mixcloud-updater/internal/cue"
	"github.com/nowwaveradio/mixcloud-updater/internal/filter"
	"github.com/nowwaveradio/mixcloud-updater/internal/mixcloud"
)

var (
	cueFile   = flag.String("cue-file", "", "Path to the CUE file to parse")
	configFile = flag.String("config", "config.toml", "Path to the configuration file")
	showName   = flag.String("show-name", "", "Name of the show to update on Mixcloud")
	dryRun     = flag.Bool("dry-run", false, "Print what would be done without making changes")
	help       = flag.Bool("help", false, "Show help information")
)

func main() {
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Validate required arguments
	if *cueFile == "" {
		log.Fatal("Error: -cue-file is required")
	}

	if *showName == "" {
		log.Fatal("Error: -show-name is required")
	}

	fmt.Printf("Mixcloud Updater v1.0\n")
	fmt.Printf("CUE File: %s\n", *cueFile)
	fmt.Printf("Show Name: %s\n", *showName)
	fmt.Printf("Config: %s\n", *configFile)
	fmt.Printf("Dry Run: %t\n", *dryRun)

	// AIDEV-TODO: Load configuration from file using config.LoadConfig()
	_ = config.Config{}

	// AIDEV-TODO: Parse CUE file using cue.ParseCueFile()
	_ = cue.CueSheet{}

	// AIDEV-TODO: Initialize content filter using filter.NewFilter()
	_ = filter.Filter{}

	// AIDEV-TODO: Initialize Mixcloud client using mixcloud.NewClient()
	_ = mixcloud.Client{}

	// AIDEV-TODO: Process show update workflow (parse -> filter -> format -> upload)
	fmt.Println("Processing... (placeholder)")

	fmt.Println("Done!")
}