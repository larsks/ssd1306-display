package main

import (
	"bufio"
	"fmt"
	"image"
	"log"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/devices/v3/ssd1306/image1bit"
	"periph.io/x/host/v3"
)

type (
	Options struct {
		Device     string
		Line       uint
		BufferFile string
		Clear      bool
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
}

func main() {
	pflag.Parse()
	args := pflag.Args()

	// Handle clear option - remove buffer file if it exists
	if options.Clear && options.BufferFile != "" {
		if err := os.Remove(options.BufferFile); err != nil && !os.IsNotExist(err) {
			log.Fatalf("failed to remove buffer file: %v", err)
		}
	}

	// Get text to display
	var newText string
	if len(args) > 0 {
		newText = strings.Join(args, " ")
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("error reading stdin: %v", err)
		}
		newText = strings.Join(lines, "\n")
	}

	// Read existing buffer if specified (unless we're clearing)
	var displayLines []string
	if options.BufferFile != "" && !options.Clear {
		if data, err := os.ReadFile(options.BufferFile); err == nil {
			displayLines = strings.Split(string(data), "\n")
		} else if !os.IsNotExist(err) {
			log.Fatalf("failed to read buffer file: %v", err)
		}
	}

	// Update lines in memory
	if newText != "" || options.Clear {
		newLines := strings.Split(newText, "\n")
		for i, newLine := range newLines {
			targetLine := options.Line + uint(i) - 1 // Convert to 0-based index
			// Expand displayLines if necessary
			for uint(len(displayLines)) <= targetLine {
				displayLines = append(displayLines, "")
			}
			displayLines[targetLine] = newLine
		}
	}

	// Write back to buffer if specified
	if options.BufferFile != "" {
		bufferContent := strings.Join(displayLines, "\n")
		if err := os.WriteFile(options.BufferFile, []byte(bufferContent), 0644); err != nil {
			log.Fatalf("failed to write buffer file: %v", err)
		}
	}

	// Prepare final text for display
	text := strings.Join(displayLines, "\n")
	fmt.Printf("got text: %s\n", text)

	// Make sure periph is initialized.
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	// Use i2creg I²C bus registry to open the specified I²C bus.
	b, err := i2creg.Open(options.Device)
	if err != nil {
		log.Fatalf("failed to open i2c bus: %v", err)
	}
	defer b.Close()

	dev, err := ssd1306.NewI2C(b, &ssd1306.DefaultOpts)
	if err != nil {
		log.Fatalf("failed to initialize ssd1306: %v", err)
	}

	// Draw on it.
	img := image1bit.NewVerticalLSB(dev.Bounds())
	f := basicfont.Face7x13
	drawer := font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{image1bit.On},
		Face: f,
	}

	// Split text by newlines and draw each line separately
	lines := strings.Split(text, "\n")
	lineHeight := f.Metrics().Height.Ceil()

	for i, textLine := range lines {
		drawer.Dot = fixed.P(0, lineHeight*(1+i)-f.Descent)
		drawer.DrawString(textLine)
	}
	if err := dev.Draw(dev.Bounds(), img, image.Point{}); err != nil {
		log.Fatal(err)
	}
}
