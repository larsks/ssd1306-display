package display

import (
	"fmt"
	"image"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/host/v3"
)

type (
	SSD1306 interface {
		Open() error
		Close() error
		Bounds() image.Rectangle
		Draw(r image.Rectangle, src image.Image, sp image.Point) error
	}

	RealSSD1306 struct {
		busName string
		bus     i2c.BusCloser
		dev     *ssd1306.Dev
	}

	FakeSSD1306 struct {
	}
)

func NewRealSSD1306(busName string) *RealSSD1306 {
	return &RealSSD1306{
		busName: busName,
	}
}

func NewFakeSSD1306() *FakeSSD1306 {
	return &FakeSSD1306{}
}

func (d *FakeSSD1306) Open() error {
	return nil
}

func (d *FakeSSD1306) Close() error {
	return nil
}

func (d *FakeSSD1306) Bounds() image.Rectangle {
	return image.Rect(0, 0, 100, 100)
}

func (d *FakeSSD1306) Draw(r image.Rectangle, src image.Image, sp image.Point) error {
	return nil
}

func (d *RealSSD1306) Open() error {
	// Make sure periph is initialized.
	if _, err := host.Init(); err != nil {
		return fmt.Errorf("failed to initialize display: %w", err)
	}

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

func (d *RealSSD1306) Close() error {
	return d.bus.Close()
}

func (d *RealSSD1306) Bounds() image.Rectangle {
	return d.dev.Bounds()
}

func (d *RealSSD1306) Draw(r image.Rectangle, src image.Image, sp image.Point) error {
	return d.dev.Draw(r, src, sp)
}
