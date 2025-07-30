package main

import (
	"bufio"
	"log"
	"os"

	"github.com/larsks/display1306/display"
	"github.com/spf13/pflag"
)

type (
	Options struct {
		Device     string
		Line       uint
		BufferFile string
		Clear      bool
		DryRun     bool
	}
)

var (
	options Options
)

func init() {
	pflag.StringVarP(&options.Device, "device", "d", "/dev/i2c-1", "path to i2c device")
	pflag.UintVarP(&options.Line, "line", "l", 1, "line number to start printing (1-based)")
	pflag.StringVarP(&options.BufferFile, "buffer", "b", "", "path to buffer file for persistent display state")
	pflag.BoolVarP(&options.Clear, "clear", "k", false, "clear the display and buffer")
	pflag.BoolVarP(&options.DryRun, "dry-run", "n", false, "run without actual hardware")
}

func main() {
	pflag.Parse()
	args := pflag.Args()

	// Get text to display
	// This has to happen before calling d.Init(), otherwise we get errors
	// reading from stdin.
	var lines []string
	if len(args) > 0 {
		lines = args
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("error reading stdin: %v", err)
		}
	}

	var driver display.SSD1306
	if options.DryRun {
		driver = display.NewFakeSSD1306()
	}

	// Initialize display
	d := display.NewDisplay().
		WithBufferFile(options.BufferFile).
		WithBusName(options.Device).
		WithDriver(driver)
	defer d.Close() //nolint:errcheck

	if err := d.Init(); err != nil {
		log.Fatal(err)
	}

	if options.Clear {
		d.Clear()
	}

	// Update display with new text
	if len(lines) > 0 {
		if err := d.PrintLines(options.Line-1, lines); err != nil {
			log.Fatalf("failed to print lines: %v", err)
		}
	}

	// Update the display
	if err := d.Update(); err != nil {
		log.Fatal(err)
	}
}
