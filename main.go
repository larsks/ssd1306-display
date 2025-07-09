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

func main() {
	var device string
	pflag.StringVarP(&device, "device", "d", "/dev/i2c-1", "path to i2c device")
	pflag.Parse()
	args := pflag.Args()

	// Get text to display
	var text string
	if len(args) > 0 {
		text = strings.Join(args, " ")
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("error reading stdin: %v", err)
		}
		text = strings.Join(lines, "\n")
	}

	fmt.Printf("got text: %s\n", text)

	// Make sure periph is initialized.
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	// Use i2creg I²C bus registry to open the specified I²C bus.
	b, err := i2creg.Open(device)
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
	
	for i, line := range lines {
		drawer.Dot = fixed.P(0, lineHeight*(i+1)-f.Descent)
		drawer.DrawString(line)
	}
	if err := dev.Draw(dev.Bounds(), img, image.Point{}); err != nil {
		log.Fatal(err)
	}
}
