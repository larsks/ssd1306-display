package display

import (
	"fmt"
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"periph.io/x/devices/v3/ssd1306/image1bit"
)

const (
	DEFAULT_MAX_LINES uint = 5
)

type (
	Display struct {
		busName     string
		driver      SSD1306
		lines       uint
		buffer      []string
		font        font.Face
		lineHeight  int
		initialized bool
	}
)

func NewDisplay() *Display {
	return &Display{
		lines: DEFAULT_MAX_LINES,
	}
}

func (d *Display) WithLines(lines uint) *Display {
	d.lines = lines
	return d
}

func (d *Display) WithBusName(busName string) *Display {
	d.busName = busName
	return d
}

func (d *Display) WithDriver(driver SSD1306) *Display {
	d.driver = driver
	return d
}

func (d *Display) WithFont(f font.Face) *Display {
	d.font = f
	d.lineHeight = f.Metrics().Height.Ceil()
	return d
}

func (d *Display) Build() (*Display, error) {
	if d.font == nil {
		f := basicfont.Face7x13
		lineHeight := f.Metrics().Height.Ceil()
		d.font = f
		d.lineHeight = lineHeight
	}
	return d, nil
}

func (d *Display) Init() error {
	d.buffer = make([]string, d.lines)

	if d.driver == nil {
		d.driver = NewRealSSD1306(d.busName)
	}

	if err := d.driver.Open(); err != nil {
		return fmt.Errorf("failed to initialize device: %w", err)
	}

	d.initialized = true

	return nil
}

func (d *Display) Close() error {
	if d.initialized {
		return d.driver.Close()
	}
	return nil
}

func (d *Display) Clear() error {
	if !d.initialized {
		return fmt.Errorf("driver has not been initialized")
	}
	for i := range d.buffer {
		d.buffer[i] = ""
	}
	return nil
}

func (d *Display) PrintLine(line uint, text string) error {
	if !d.initialized {
		return fmt.Errorf("driver has not been initialized")
	}

	if int(line) >= len(d.buffer) {
		return fmt.Errorf("request to draw on line %d but display only has %d lines", line, len(d.buffer))
	}

	d.buffer[line] = text
	return nil
}

func (d *Display) PrintLines(line uint, text []string) error {
	if !d.initialized {
		return fmt.Errorf("driver has not been initialized")
	}

	if int(line)+len(text) > int(d.lines) {
		return fmt.Errorf("text requires more than %d lines", len(d.buffer))
	}

	for i := range text {
		d.buffer[int(line)+i] = text[i]
	}

	return nil
}

func (d *Display) Update() error {
	if !d.initialized {
		return fmt.Errorf("driver has not been initialized")
	}

	img := image1bit.NewVerticalLSB(d.driver.Bounds())
	screen := font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{image1bit.On},
		Face: d.font,
	}

	for i, textLine := range d.buffer {
		screen.Dot = fixed.P(0, d.lineHeight*(1+i)) //-d.font.Descent)
		screen.DrawString(textLine)
	}
	if err := d.driver.Draw(d.driver.Bounds(), img, image.Point{}); err != nil {
		return fmt.Errorf("failed to draw on display: %w", err)
	}

	return nil
}
