package display

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/bmp"
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

func (d *Display) ClearLines() error {
	if !d.initialized {
		return fmt.Errorf("driver has not been initialized")
	}
	for i := range d.buffer {
		d.buffer[i] = ""
	}
	return nil
}

func (d *Display) ClearScreen() error {
	img := image1bit.NewVerticalLSB(d.driver.Bounds())
	if err := d.driver.Draw(d.driver.Bounds(), img, image.Point{}); err != nil {
		return fmt.Errorf("failed to draw on display: %w", err)
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
		screen.Dot = fixed.P(0, d.lineHeight*(1+i)-d.font.Metrics().Descent.Round())
		screen.DrawString(textLine)
	}
	if err := d.driver.Draw(d.driver.Bounds(), img, image.Point{}); err != nil {
		return fmt.Errorf("failed to draw on display: %w", err)
	}

	return nil
}

func (d *Display) SetFont(f font.Face) {
	d.font = f
	d.lineHeight = f.Metrics().Height.Ceil()
}

func (d *Display) ShowImage(img image.Image) error {
	if !d.initialized {
		return fmt.Errorf("driver has not been initialized")
	}

	bounds := d.driver.Bounds()
	displayImg := image1bit.NewVerticalLSB(bounds)

	imgBounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			srcX := imgBounds.Min.X + x
			srcY := imgBounds.Min.Y + y
			if srcX < imgBounds.Max.X && srcY < imgBounds.Max.Y {
				c := img.At(srcX, srcY)
				gray := color.GrayModel.Convert(c).(color.Gray)
				if gray.Y > 128 {
					displayImg.Set(x, y, image1bit.On)
				} else {
					displayImg.Set(x, y, image1bit.Off)
				}
			}
		}
	}

	if err := d.driver.Draw(bounds, displayImg, image.Point{}); err != nil {
		return fmt.Errorf("failed to draw image on display: %w", err)
	}

	return nil
}

func (d *Display) ShowImageFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close() //nolint:errcheck

	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	return d.ShowImage(img)
}
