package display

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/larsks/display1306/v2/display/fakedriver"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// Call represents a method call on the mock
type Call struct {
	Method string
	Args   []interface{}
}

// Enhanced FakeSSD1306 for testing with call tracking
type TrackedFakeSSD1306 struct {
	*fakedriver.FakeSSD1306
	Calls        []Call
	ErrorOnOpen  bool
	ErrorOnClose bool
	ErrorOnDraw  bool
}

func NewTrackedFakeSSD1306() *TrackedFakeSSD1306 {
	return &TrackedFakeSSD1306{
		FakeSSD1306: fakedriver.NewFakeSSD1306(),
		Calls:       make([]Call, 0),
	}
}

func (t *TrackedFakeSSD1306) Open() error {
	t.Calls = append(t.Calls, Call{Method: "Open", Args: nil})
	if t.ErrorOnOpen {
		return fmt.Errorf("mock open error")
	}
	return nil
}

func (t *TrackedFakeSSD1306) Close() error {
	t.Calls = append(t.Calls, Call{Method: "Close", Args: nil})
	if t.ErrorOnClose {
		return fmt.Errorf("mock close error")
	}
	return nil
}

func (t *TrackedFakeSSD1306) Bounds() image.Rectangle {
	t.Calls = append(t.Calls, Call{Method: "Bounds", Args: nil})
	return t.FakeSSD1306.Bounds()
}

func (t *TrackedFakeSSD1306) Draw(r image.Rectangle, src image.Image, sp image.Point) error {
	t.Calls = append(t.Calls, Call{Method: "Draw", Args: []interface{}{r, src, sp}})
	if t.ErrorOnDraw {
		return fmt.Errorf("mock draw error")
	}
	return nil
}

// Test helper functions
func (t *TrackedFakeSSD1306) WasCalled(method string) bool {
	for _, call := range t.Calls {
		if call.Method == method {
			return true
		}
	}
	return false
}

func (t *TrackedFakeSSD1306) CallCount(method string) int {
	count := 0
	for _, call := range t.Calls {
		if call.Method == method {
			count++
		}
	}
	return count
}

func (t *TrackedFakeSSD1306) LastDrawArgs() (image.Rectangle, image.Image, image.Point) {
	for i := len(t.Calls) - 1; i >= 0; i-- {
		if t.Calls[i].Method == "Draw" && len(t.Calls[i].Args) == 3 {
			return t.Calls[i].Args[0].(image.Rectangle),
				t.Calls[i].Args[1].(image.Image),
				t.Calls[i].Args[2].(image.Point)
		}
	}
	return image.Rectangle{}, nil, image.Point{}
}

// Test assertion helpers
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func assertError(t *testing.T, err error, expectedSubstr string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error but got none")
	} else if expectedSubstr != "" && !strings.Contains(err.Error(), expectedSubstr) {
		t.Errorf("Expected error to contain %q, got %q", expectedSubstr, err.Error())
	}
}

func assertMethodCalled(t *testing.T, mock *TrackedFakeSSD1306, method string) {
	t.Helper()
	if !mock.WasCalled(method) {
		t.Errorf("Expected %s to be called", method)
	}
}

func TestNewDisplay(t *testing.T) {
	tests := []struct {
		name    string
		busName string
		dev     SSD1306
		wantDev bool
	}{
		{
			name:    "with provided fake device",
			busName: "/dev/i2c-0",
			dev:     fakedriver.NewFakeSSD1306(),
			wantDev: true,
		},
		{
			name:    "with nil device creates real device",
			busName: "/dev/i2c-1",
			dev:     nil,
			wantDev: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, err := NewDisplay().WithBusName(tt.busName).WithDriver(tt.dev).Build()
			assertNoError(t, err)
			_ = display.Init()

			if display == nil {
				t.Fatal("NewDisplay returned nil")
			}

			if display.driver == nil && tt.wantDev {
				t.Error("Expected device to be set")
			}

			if display.lines != DEFAULT_MAX_LINES {
				t.Errorf("Expected lines to be %d, got %d", DEFAULT_MAX_LINES, display.lines)
			}

			if display.font == nil {
				t.Error("Expected font to be set")
			}

			if display.lineHeight <= 0 {
				t.Error("Expected lineHeight to be positive")
			}
		})
	}
}

func TestDisplay_WithFont(t *testing.T) {
	tests := []struct {
		name         string
		font         font.Face
		expectHeight int
	}{
		{
			name:         "with basicfont Face7x13",
			font:         basicfont.Face7x13,
			expectHeight: basicfont.Face7x13.Metrics().Height.Ceil(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display := NewDisplay().WithFont(tt.font)

			if display.font != tt.font {
				t.Error("Expected custom font to be set")
			}

			if display.lineHeight != tt.expectHeight {
				t.Errorf("Expected lineHeight to be %d, got %d", tt.expectHeight, display.lineHeight)
			}
		})
	}
}

func TestDisplay_Build_WithDefaultFont(t *testing.T) {
	display := NewDisplay()
	built, err := display.Build()
	assertNoError(t, err)

	// Should have default font set
	if built.font == nil {
		t.Error("Expected default font to be set")
	}

	expectedHeight := basicfont.Face7x13.Metrics().Height.Ceil()
	if built.lineHeight != expectedHeight {
		t.Errorf("Expected lineHeight to be %d, got %d", expectedHeight, built.lineHeight)
	}
}

func TestDisplay_Build_WithCustomFont(t *testing.T) {
	customFont := basicfont.Face7x13
	display := NewDisplay().WithFont(customFont)
	built, err := display.Build()
	assertNoError(t, err)

	// Should preserve custom font
	if built.font != customFont {
		t.Error("Expected custom font to be preserved")
	}

	expectedHeight := customFont.Metrics().Height.Ceil()
	if built.lineHeight != expectedHeight {
		t.Errorf("Expected lineHeight to be %d, got %d", expectedHeight, built.lineHeight)
	}
}

func TestDisplay_Init(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*TrackedFakeSSD1306)
		wantError   bool
		errorSubstr string
	}{
		{
			name: "successful init",
			setupMock: func(mock *TrackedFakeSSD1306) {
				// No errors
			},
			wantError: false,
		},
		{
			name: "device open error",
			setupMock: func(mock *TrackedFakeSSD1306) {
				mock.ErrorOnOpen = true
			},
			wantError:   true,
			errorSubstr: "failed to initialize device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewTrackedFakeSSD1306()
			tt.setupMock(mock)

			display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
			assertNoError(t, err)

			err = display.Init()

			if tt.wantError {
				assertError(t, err, tt.errorSubstr)
			} else {
				assertNoError(t, err)
				assertMethodCalled(t, mock, "Open")

				if len(display.buffer) != int(display.lines) {
					t.Errorf("Expected buffer length to be %d, got %d", display.lines, len(display.buffer))
				}
			}
		})
	}
}

func TestDisplay_Close(t *testing.T) {
	tests := []struct {
		name        string
		shouldError bool
		wantError   bool
	}{
		{
			name:        "successful close",
			shouldError: false,
			wantError:   false,
		},
		{
			name:        "close with error",
			shouldError: true,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewTrackedFakeSSD1306()
			mock.ErrorOnClose = tt.shouldError

			display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
			assertNoError(t, err)
			err = display.Init()
			assertNoError(t, err)

			err = display.Close()

			if tt.wantError {
				assertError(t, err, "")
			} else {
				assertNoError(t, err)
			}

			assertMethodCalled(t, mock, "Close")
		})
	}
}

func TestDisplay_ClearLines(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	// Initialize the display to set up the buffer
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	// Add some text to the buffer
	display.buffer[0] = "Test line 1"
	display.buffer[1] = "Test line 2"

	// Clear the display
	display.ClearLines() //nolint:errcheck

	// Verify all buffer lines are empty
	for i, line := range display.buffer {
		if line != "" {
			t.Errorf("Expected buffer[%d] to be empty, got %q", i, line)
		}
	}
}

func TestDisplay_PrintLine(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	// Initialize the display
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	tests := []struct {
		name      string
		line      uint
		text      string
		wantError bool
	}{
		{
			name:      "valid line",
			line:      0,
			text:      "Hello World",
			wantError: false,
		},
		{
			name:      "last valid line",
			line:      DEFAULT_MAX_LINES - 1,
			text:      "Last line",
			wantError: false,
		},
		{
			name:      "line out of bounds",
			line:      DEFAULT_MAX_LINES,
			text:      "Should fail",
			wantError: true,
		},
		{
			name:      "line far out of bounds",
			line:      100,
			text:      "Should fail",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := display.PrintLine(tt.line, tt.text)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				if int(tt.line) < len(display.buffer) && display.buffer[tt.line] != tt.text {
					t.Errorf("Expected buffer[%d] to be %q, got %q", tt.line, tt.text, display.buffer[tt.line])
				}
			}
		})
	}
}

func TestDisplay_PrintLines(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	// Initialize the display
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	tests := []struct {
		name      string
		line      uint
		text      []string
		wantError bool
	}{
		{
			name:      "valid lines",
			line:      0,
			text:      []string{"Line 1", "Line 2", "Line 3"},
			wantError: false,
		},
		{
			name:      "single line",
			line:      2,
			text:      []string{"Single line"},
			wantError: false,
		},
		{
			name:      "fills remaining space",
			line:      3,
			text:      []string{"Line 4", "Line 5"},
			wantError: false,
		},
		{
			name:      "overflow",
			line:      0,
			text:      []string{"1", "2", "3", "4", "5", "6"}, // 6 lines but only 5 available
			wantError: true,
		},
		{
			name:      "overflow from middle",
			line:      3,
			text:      []string{"Line 4", "Line 5", "Line 6"}, // 3 lines starting at line 3 = 6 total
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear buffer before each test
			display.ClearLines() //nolint:errcheck

			err := display.PrintLines(tt.line, tt.text)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Verify the text was written to the correct positions
				for i, expectedText := range tt.text {
					bufferIndex := int(tt.line) + i
					if bufferIndex < len(display.buffer) {
						if display.buffer[bufferIndex] != expectedText {
							t.Errorf("Expected buffer[%d] to be %q, got %q", bufferIndex, expectedText, display.buffer[bufferIndex])
						}
					}
				}
			}
		})
	}
}

func TestDisplay_Update(t *testing.T) {
	tests := []struct {
		name           string
		setupBuffer    func(*Display)
		mockShouldErr  bool
		wantError      bool
		errorSubstr    string
		wantDrawCalled bool
	}{
		{
			name: "successful update",
			setupBuffer: func(d *Display) {
				d.buffer[0] = "Line 1"
				d.buffer[1] = "Line 2"
			},
			wantDrawCalled: true,
			wantError:      false,
		},
		{
			name: "draw error",
			setupBuffer: func(d *Display) {
				d.buffer[0] = "Test"
			},
			mockShouldErr:  true,
			wantError:      true,
			errorSubstr:    "failed to draw on display",
			wantDrawCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewTrackedFakeSSD1306()
			mock.ErrorOnDraw = tt.mockShouldErr

			display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
			assertNoError(t, err)

			// Initialize and setup buffer
			if err := display.Init(); err != nil {
				t.Fatalf("Failed to initialize display: %v", err)
			}

			if tt.setupBuffer != nil {
				tt.setupBuffer(display)
			}

			err = display.Update()

			if tt.wantError {
				assertError(t, err, tt.errorSubstr)
			} else {
				assertNoError(t, err)
			}

			if tt.wantDrawCalled {
				assertMethodCalled(t, mock, "Draw")
				expectedBounds := mock.Bounds()
				drawRect, _, _ := mock.LastDrawArgs()
				if drawRect != expectedBounds {
					t.Errorf("Expected draw rect to be %v, got %v", expectedBounds, drawRect)
				}
			}
		})
	}
}

func TestDisplay_MethodsFailWithoutInit(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	// Test that all methods fail when Init() hasn't been called
	tests := []struct {
		name        string
		operation   func() error
		errorSubstr string
	}{
		{
			name: "Clear fails without init",
			operation: func() error {
				return display.ClearLines()
			},
			errorSubstr: "driver has not been initialized",
		},
		{
			name: "PrintLine fails without init",
			operation: func() error {
				return display.PrintLine(0, "test")
			},
			errorSubstr: "driver has not been initialized",
		},
		{
			name: "PrintLines fails without init",
			operation: func() error {
				return display.PrintLines(0, []string{"test"})
			},
			errorSubstr: "driver has not been initialized",
		},
		{
			name: "Update fails without init",
			operation: func() error {
				return display.Update()
			},
			errorSubstr: "driver has not been initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			assertError(t, err, tt.errorSubstr)
		})
	}

	// Verify that no driver methods were called
	if len(mock.Calls) > 0 {
		t.Errorf("Expected no driver methods to be called, but got: %v", mock.Calls)
	}
}

func TestDisplay_Integration(t *testing.T) {
	// Integration test that exercises the full workflow
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	// Initialize
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Add some text
	if err := display.PrintLine(0, "Hello World"); err != nil {
		t.Fatalf("Failed to print line: %v", err)
	}

	if err := display.PrintLines(1, []string{"Line 2", "Line 3"}); err != nil {
		t.Fatalf("Failed to print lines: %v", err)
	}

	// Update display
	if err := display.Update(); err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Verify all expected calls were made
	assertMethodCalled(t, mock, "Open")
	assertMethodCalled(t, mock, "Draw")

	if mock.CallCount("Draw") != 1 {
		t.Errorf("Expected Draw to be called once, got %d times", mock.CallCount("Draw"))
	}

	// Clear and verify
	display.ClearLines() //nolint:errcheck
	for i, line := range display.buffer {
		if line != "" {
			t.Errorf("Expected buffer[%d] to be empty after clear, got %q", i, line)
		}
	}

	// Close
	if err := display.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	assertMethodCalled(t, mock, "Close")
}

func TestDisplay_FontIntegration(t *testing.T) {
	// Integration test that verifies font handling throughout the workflow
	mock := NewTrackedFakeSSD1306()

	// Test with custom font
	customFont := basicfont.Face7x13
	display, err := NewDisplay().
		WithBusName("/dev/i2c-0").
		WithDriver(mock).
		WithFont(customFont).
		Build()
	assertNoError(t, err)

	// Verify font is set correctly
	if display.font != customFont {
		t.Error("Expected custom font to be preserved")
	}

	expectedHeight := customFont.Metrics().Height.Ceil()
	if display.lineHeight != expectedHeight {
		t.Errorf("Expected lineHeight to be %d, got %d", expectedHeight, display.lineHeight)
	}

	// Initialize and use the display
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	if err := display.PrintLine(0, "Font Test"); err != nil {
		t.Fatalf("Failed to print line: %v", err)
	}

	if err := display.Update(); err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Verify draw was called
	assertMethodCalled(t, mock, "Draw")
}

// TestImage is a helper type for creating test images
type TestImage struct {
	bounds image.Rectangle
	pixels map[image.Point]color.Color
}

func NewTestImage(width, height int) *TestImage {
	return &TestImage{
		bounds: image.Rect(0, 0, width, height),
		pixels: make(map[image.Point]color.Color),
	}
}

func (t *TestImage) ColorModel() color.Model {
	return color.RGBAModel
}

func (t *TestImage) Bounds() image.Rectangle {
	return t.bounds
}

func (t *TestImage) At(x, y int) color.Color {
	if c, ok := t.pixels[image.Point{x, y}]; ok {
		return c
	}
	return color.RGBA{0, 0, 0, 255} // Default to black
}

func (t *TestImage) Set(x, y int, c color.Color) {
	t.pixels[image.Point{x, y}] = c
}

func TestDisplay_ShowImage_SmallImage(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	// Create a small test image (smaller than display bounds)
	displayBounds := mock.Bounds()
	smallImage := NewTestImage(displayBounds.Dx()/2, displayBounds.Dy()/2)

	// Set some white pixels
	smallImage.Set(0, 0, color.RGBA{255, 255, 255, 255})
	smallImage.Set(1, 1, color.RGBA{255, 255, 255, 255})

	err = display.ShowImage(smallImage)
	assertNoError(t, err)
	assertMethodCalled(t, mock, "Draw")
}

func TestDisplay_ShowImage_LargeImageCropped(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	// Create a large test image (larger than display bounds)
	displayBounds := mock.Bounds()
	largeImage := NewTestImage(displayBounds.Dx()*2, displayBounds.Dy()*2)

	// Set different colored pixels in different quadrants
	// Upper left (should be visible after cropping)
	largeImage.Set(0, 0, color.RGBA{255, 255, 255, 255}) // White
	largeImage.Set(1, 0, color.RGBA{255, 255, 255, 255}) // White

	// Upper right (should be cropped out)
	largeImage.Set(displayBounds.Dx()+10, 0, color.RGBA{255, 0, 0, 255}) // Red

	// Lower left (should be cropped out)
	largeImage.Set(0, displayBounds.Dy()+10, color.RGBA{0, 255, 0, 255}) // Green

	// This should not return an error anymore - large images should be cropped
	err = display.ShowImage(largeImage)
	assertNoError(t, err)
	assertMethodCalled(t, mock, "Draw")
}

func TestDisplay_ShowImage_ExactSizeImage(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	// Create an image exactly the size of the display
	displayBounds := mock.Bounds()
	exactImage := NewTestImage(displayBounds.Dx(), displayBounds.Dy())

	// Fill with a pattern
	for y := 0; y < displayBounds.Dy(); y++ {
		for x := 0; x < displayBounds.Dx(); x++ {
			if (x+y)%2 == 0 {
				exactImage.Set(x, y, color.RGBA{255, 255, 255, 255}) // White
			}
		}
	}

	err = display.ShowImage(exactImage)
	assertNoError(t, err)
	assertMethodCalled(t, mock, "Draw")
}

func TestDisplay_ShowImage_WithoutInit(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	// Don't initialize the display
	testImage := NewTestImage(10, 10)

	err = display.ShowImage(testImage)
	assertError(t, err, "driver has not been initialized")

	// Verify no draw was called
	if mock.WasCalled("Draw") {
		t.Error("Expected Draw not to be called when display not initialized")
	}
}

func TestDisplay_ShowImage_DrawError(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	mock.ErrorOnDraw = true

	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	testImage := NewTestImage(10, 10)

	err = display.ShowImage(testImage)
	assertError(t, err, "failed to draw image on display")
	assertMethodCalled(t, mock, "Draw")
}

func TestDisplay_ShowImageFromFile_Integration(t *testing.T) {
	// This test just verifies that ShowImageFromFile calls ShowImage
	// We can't easily test file operations without creating temporary files
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	// Test with non-existent file should return appropriate error
	err = display.ShowImageFromFile("/nonexistent/file.png")
	assertError(t, err, "failed to open image file")

	// Verify no draw was called due to file error
	if mock.WasCalled("Draw") {
		t.Error("Expected Draw not to be called when file cannot be opened")
	}
}

func TestDisplay_SetFont(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display, err := NewDisplay().WithBusName("/dev/i2c-0").WithDriver(mock).Build()
	assertNoError(t, err)

	// Initialize the display
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}

	// Set a new font (using the same font for simplicity, but this demonstrates the method works)
	newFont := basicfont.Face7x13
	display.SetFont(newFont)

	// Verify the font was changed
	if display.font != newFont {
		t.Error("Expected font to be updated")
	}

	// Verify line height was recalculated
	expectedHeight := newFont.Metrics().Height.Ceil()
	if display.lineHeight != expectedHeight {
		t.Errorf("Expected lineHeight to be %d, got %d", expectedHeight, display.lineHeight)
	}

	// Verify we can still use the display normally after changing font
	if err := display.PrintLine(0, "Test with new font"); err != nil {
		t.Fatalf("Failed to print line after font change: %v", err)
	}

	if err := display.Update(); err != nil {
		t.Fatalf("Failed to update after font change: %v", err)
	}

	assertMethodCalled(t, mock, "Draw")
}
