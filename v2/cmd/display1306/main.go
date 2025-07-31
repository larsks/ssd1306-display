package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/golang/freetype/truetype"
	"github.com/larsks/display1306/v2/display"
	"github.com/larsks/display1306/v2/display/fakedriver"
	"github.com/spf13/pflag"
)

type (
	Options struct {
		Device        string
		Line          uint
		Clear         bool
		DryRun        bool
		Font          string
		FontSize      float64
		Image         bool
		ImageInterval time.Duration
		Loop          bool
		Duration      time.Duration
		Wait          bool
	}
)

var (
	options Options
)

func processCommand(command string, d *display.Display) (bool, error) {
	if command[0] != '@' {
		return false, nil
	}

	parts := strings.Split(command[1:], "=")
	skip := false

	switch parts[0] {
	case "clear":
		if err := d.ClearScreen(); err != nil {
			return false, fmt.Errorf("failed to clear display: %v", err)
		}
	case "interval":
		skip = true
		if len(parts) != 2 {
			return false, fmt.Errorf("@interval requires a value: @interval=duration")
		}
		interval, err := time.ParseDuration(parts[1])
		if err != nil {
			return false, fmt.Errorf("invalid interval duration '%s': %v", parts[1], err)
		}
		options.ImageInterval = interval
		log.Printf("Updated image interval to %v", options.ImageInterval)
	case "pause":
		skip = true
		if len(parts) != 2 {
			return false, fmt.Errorf("@pause requires a value: @pause=duration")
		}
		pauseDuration, err := time.ParseDuration(parts[1])
		if err != nil {
			return false, fmt.Errorf("invalid pause duration '%s': %v", parts[1], err)
		}
		log.Printf("Pausing for %v", pauseDuration)
		time.Sleep(pauseDuration)
	default:
		return false, fmt.Errorf("unknown command: %s", command)
	}
	return skip, nil
}

func init() {
	pflag.StringVarP(&options.Device, "device", "d", "/dev/i2c-1", "path to i2c device")
	pflag.UintVarP(&options.Line, "line", "l", 1, "line number to start printing (1-based)")
	pflag.BoolVarP(&options.Clear, "clear", "k", false, "clear the display")
	pflag.BoolVarP(&options.DryRun, "dry-run", "n", false, "run without actual hardware")
	pflag.StringVarP(&options.Font, "font", "f", "", "path to truetype font file")
	pflag.Float64VarP(&options.FontSize, "font-size", "s", 13.0, "font size in points (ignored if --font not provided)")
	pflag.BoolVarP(&options.Image, "image", "i", false, "interpret non-option arguments as image filenames")
	pflag.DurationVar(&options.ImageInterval, "image-interval", 30*time.Millisecond, "interval between images")
	pflag.BoolVar(&options.Loop, "loop", false, "loop through images continuously")
	pflag.BoolVar(&options.Wait, "wait", false, "pause and require ctrl-c to exit")
	pflag.DurationVar(&options.Duration, "duration", 0, "maximum duration to run loop (0 for unlimited)")
}

func main() {
	pflag.Parse()
	args := pflag.Args()

	// Validate arguments
	if options.Image && len(args) == 0 {
		log.Fatalf("--image requires at least one image filename as argument")
	}

	pflag.Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "loop":
			if !options.Image {
				log.Fatalf("--loop can only be used with --image")
			}
		case "image-interval":
			if !options.Image {
				log.Fatalf("--image-interval can only be used with --image")
			}
		case "font-size", "font":
			if options.Image {
				log.Fatalf("--font and --font-size cannot be used with --image")
			}
		case "wait":
			if !options.DryRun {
				log.Fatalf("--wait can only be used with --dry-run")
			}
		}
	})

	// Get text to display
	// This has to happen before calling d.Init(), otherwise we get errors
	// reading from stdin.
	var lines []string
	if !options.Image {
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
	}

	var driver display.SSD1306
	var fakeDriver *fakedriver.FakeSSD1306
	if options.DryRun {
		fakeDriver = fakedriver.NewFakeSSD1306()
		fakeDriver.SetWaitMode(true)
		driver = fakeDriver
	}

	// Initialize display
	builder := display.NewDisplay().
		WithBusName(options.Device).
		WithDriver(driver)

	if options.Font != "" {
		fontData, err := os.ReadFile(options.Font)
		if err != nil {
			log.Fatalf("failed to read font file: %v", err)
		}

		tf, err := truetype.Parse(fontData)
		if err != nil {
			log.Fatalf("failed to parse font: %v", err)
		}

		fontFace := truetype.NewFace(tf, &truetype.Options{
			Size: options.FontSize,
			DPI:  72,
		})

		builder = builder.WithFont(fontFace)
	}

	d, err := builder.Build()
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close() //nolint:errcheck

	if err := d.Init(); err != nil {
		log.Fatal(err)
	}

	if options.Clear {
		d.ClearScreen() //nolint:errcheck
	}

	// If using fake driver in wait mode, wait for start signal
	if fakeDriver != nil && fakeDriver.IsWaitMode() {
		log.Println("Waiting for start button click in browser...")
		fakeDriver.WaitForStart()
		log.Println("Start button clicked, beginning rendering...")
	}

	if options.Image {
		// Display images in sequence
		var startTime time.Time
		if options.Loop && options.Duration > 0 {
			startTime = time.Now()
		}

	outer:
		for {
			for _, imagePath := range args {
				if imagePath[0] == '@' {
					skip, err := processCommand(imagePath, d)
					if err != nil {
						log.Fatalf("failed to process command '%s': %v", imagePath, err)
					}
					if skip {
						continue
					}
				} else {
					if err := d.ShowImageFromFile(imagePath); err != nil {
						log.Fatalf("failed to display image %s: %v", imagePath, err)
					}
				}

				if len(args) > 1 {
					time.Sleep(options.ImageInterval)
				}

				// Check duration limit if looping
				if options.Loop && options.Duration > 0 && time.Since(startTime) >= options.Duration {
					break outer
				}
			}
			if !options.Loop {
				break
			}
		}
	} else {
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

	if options.DryRun && options.Wait {
		log.Printf("paused; press CTRL-C to exit")
		syscall.Pause() //nolint:errcheck
	}

	// If using fake driver in blocking mode, wait for interrupt
	if fakeDriver != nil {
		// Explicitly close the display to shut down the HTTP server
		d.Close() //nolint:errcheck
	}
}
