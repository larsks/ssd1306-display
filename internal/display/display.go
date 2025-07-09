package display

import (
	"fmt"
	"image"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/devices/v3/ssd1306/image1bit"
	"periph.io/x/host/v3"
)

type (
	Display struct {
		busName    string
		bus        i2c.BusCloser
		dev        *ssd1306.Dev
		lines      uint
		bufferFile string
		buffer     []string
		font       *basicfont.Face
		lineHeight int
	}
)

// d := NewDisplay().WithBus("/dev/i2c-0").WithBufferFile("/tmp/display.txt")
func NewDisplay() *Display {
	f := basicfont.Face7x13
	lineHeight := f.Metrics().Height.Ceil()

	return &Display{
		busName:    "/dev/i2c-1",
		lines:      5,
		font:       f,
		lineHeight: lineHeight,
	}
}

func (d *Display) WithBus(busName string) *Display {
	d.busName = busName
	return d
}

func (d *Display) WithBufferFile(bufferFile string) *Display {
	d.bufferFile = bufferFile
	return d
}

func (d *Display) Close() error {
	return d.bus.Close()
}

func (d *Display) Clear() {
	for i := range d.buffer {
		d.buffer[i] = ""
	}
}

func (d *Display) PrintLine(line uint, text string) error {
	if int(line) > len(d.buffer) {
		return fmt.Errorf("request to draw on line %d but display only has %d lines", line, len(d.buffer))
	}

	d.buffer[line] = text
	return nil
}

func (d *Display) PrintLines(line uint, text []string) error {
	if int(line)+len(text) > len(d.buffer) {
		return fmt.Errorf("text would overflow display")
	}

	for i := range text {
		d.buffer[int(line)+i] = text[i]
	}

	return nil
}

func (d *Display) updateFromFile() error {
	if d.bufferFile == "" {
		return fmt.Errorf("bufferFile is undefined")
	}
	if data, err := os.ReadFile(d.bufferFile); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) > len(d.buffer) {
			lines = lines[0:len(d.buffer)]
		}
		d.buffer = lines
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *Display) Init() error {
	// Make sure periph is initialized.
	if _, err := host.Init(); err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}

	d.buffer = make([]string, d.lines)

	if d.bufferFile != "" {
		if err := d.updateFromFile(); err != nil {
			return fmt.Errorf("failed to initialized from buffer file: %w", err)
		}
	}

	// Use i2creg I²C bus registry to open the specified I²C bus.
	b, err := i2creg.Open(d.busName)
	if err != nil {
		return fmt.Errorf("failed to open i2c bus %s: %w", d.busName, err)
	}
	d.bus = b

	dev, err := ssd1306.NewI2C(b, &ssd1306.DefaultOpts)
	if err != nil {
		return fmt.Errorf("failed to initialize ssd1306: %w", err)
	}
	d.dev = dev

	return nil
}

func (d *Display) Update() error {
	img := image1bit.NewVerticalLSB(d.dev.Bounds())
	screen := font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{image1bit.On},
		Face: d.font,
	}

	for i, textLine := range d.buffer {
		screen.Dot = fixed.P(0, d.lineHeight*(1+i)-d.font.Descent)
		screen.DrawString(textLine)
	}
	if err := d.dev.Draw(d.dev.Bounds(), img, image.Point{}); err != nil {
		return fmt.Errorf("failed to draw on display: %w", err)
	}

	return nil
}
