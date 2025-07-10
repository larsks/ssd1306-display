package display

import (
	"fmt"
	"image"
	"os"
	"strings"
	"testing"
)

// Call represents a method call on the mock
type Call struct {
	Method string
	Args   []interface{}
}

// Enhanced FakeSSD1306 for testing with call tracking
type TrackedFakeSSD1306 struct {
	*FakeSSD1306
	Calls       []Call
	ErrorOnOpen bool
	ErrorOnClose bool
	ErrorOnDraw bool
}

func NewTrackedFakeSSD1306() *TrackedFakeSSD1306 {
	return &TrackedFakeSSD1306{
		FakeSSD1306: NewFakeSSD1306(),
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
			dev:     NewFakeSSD1306(),
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
			display := NewDisplay(tt.busName, tt.dev)
			
			if display == nil {
				t.Fatal("NewDisplay returned nil")
			}
			
			if display.dev == nil && tt.wantDev {
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

func TestDisplay_WithBufferFile(t *testing.T) {
	fake := NewFakeSSD1306()
	display := NewDisplay("/dev/i2c-0", fake)
	
	bufferFile := "/tmp/test_buffer.txt"
	result := display.WithBufferFile(bufferFile)
	
	if result != display {
		t.Error("WithBufferFile should return the same display instance")
	}
	
	if display.bufferFile != bufferFile {
		t.Errorf("Expected bufferFile to be %s, got %s", bufferFile, display.bufferFile)
	}
}

func TestDisplay_Init(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*TrackedFakeSSD1306)
		bufferFile  string
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
		{
			name: "init with buffer file",
			setupMock: func(mock *TrackedFakeSSD1306) {
				// No errors
			},
			bufferFile: "/tmp/test_init_buffer.txt",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewTrackedFakeSSD1306()
			tt.setupMock(mock)
			
			display := NewDisplay("/dev/i2c-0", mock)
			
			if tt.bufferFile != "" {
				display.WithBufferFile(tt.bufferFile)
				// Create a test buffer file
				testContent := "Line 1\nLine 2\nLine 3"
				if err := os.WriteFile(tt.bufferFile, []byte(testContent), 0644); err != nil {
					t.Fatalf("Failed to create test buffer file: %v", err)
				}
				defer os.Remove(tt.bufferFile)
			}
			
			err := display.Init()
			
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
			
			display := NewDisplay("/dev/i2c-0", mock)
			
			err := display.Close()
			
			if tt.wantError {
				assertError(t, err, "")
			} else {
				assertNoError(t, err)
			}
			
			assertMethodCalled(t, mock, "Close")
		})
	}
}

func TestDisplay_Clear(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display := NewDisplay("/dev/i2c-0", mock)
	
	// Initialize the display to set up the buffer
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}
	
	// Add some text to the buffer
	display.buffer[0] = "Test line 1"
	display.buffer[1] = "Test line 2"
	
	// Clear the display
	display.Clear()
	
	// Verify all buffer lines are empty
	for i, line := range display.buffer {
		if line != "" {
			t.Errorf("Expected buffer[%d] to be empty, got %q", i, line)
		}
	}
}

func TestDisplay_PrintLine(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display := NewDisplay("/dev/i2c-0", mock)
	
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
	display := NewDisplay("/dev/i2c-0", mock)
	
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
			display.Clear()
			
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
		bufferFile     string
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
			name: "update with buffer file",
			setupBuffer: func(d *Display) {
				d.buffer[0] = "File Line 1"
				d.buffer[1] = "File Line 2"
			},
			bufferFile:     "/tmp/test_update_buffer.txt",
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
			
			display := NewDisplay("/dev/i2c-0", mock)
			
			if tt.bufferFile != "" {
				display.WithBufferFile(tt.bufferFile)
				defer os.Remove(tt.bufferFile)
			}
			
			// Initialize and setup buffer
			if err := display.Init(); err != nil {
				t.Fatalf("Failed to initialize display: %v", err)
			}
			
			if tt.setupBuffer != nil {
				tt.setupBuffer(display)
			}
			
			err := display.Update()
			
			if tt.wantError {
				assertError(t, err, tt.errorSubstr)
			} else {
				assertNoError(t, err)
				
				// Verify buffer file was written if specified
				if tt.bufferFile != "" {
					if _, err := os.Stat(tt.bufferFile); os.IsNotExist(err) {
						t.Error("Expected buffer file to be created")
					} else {
						content, err := os.ReadFile(tt.bufferFile)
						if err != nil {
							t.Errorf("Failed to read buffer file: %v", err)
						} else {
							expectedContent := strings.Join(display.buffer, "\n")
							if string(content) != expectedContent {
								t.Errorf("Expected buffer file content %q, got %q", expectedContent, string(content))
							}
						}
					}
				}
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

func TestDisplay_UpdateFromFile(t *testing.T) {
	mock := NewTrackedFakeSSD1306()
	display := NewDisplay("/dev/i2c-0", mock)
	
	// Test with buffer file
	bufferFile := "/tmp/test_update_from_file.txt"
	display.WithBufferFile(bufferFile)
	defer os.Remove(bufferFile)
	
	// Create test content
	testContent := "File Line 1\nFile Line 2\nFile Line 3\nFile Line 4\nFile Line 5\nExtra Line"
	if err := os.WriteFile(bufferFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Initialize display
	if err := display.Init(); err != nil {
		t.Fatalf("Failed to initialize display: %v", err)
	}
	
	// Verify content was loaded correctly (should be truncated to display.lines)
	expectedLines := []string{"File Line 1", "File Line 2", "File Line 3", "File Line 4", "File Line 5"}
	for i, expected := range expectedLines {
		if i < len(display.buffer) && display.buffer[i] != expected {
			t.Errorf("Expected buffer[%d] to be %q, got %q", i, expected, display.buffer[i])
		}
	}
}

func TestDisplay_Integration(t *testing.T) {
	// Integration test that exercises the full workflow
	mock := NewTrackedFakeSSD1306()
	display := NewDisplay("/dev/i2c-0", mock)
	
	bufferFile := "/tmp/test_integration.txt"
	display.WithBufferFile(bufferFile)
	defer os.Remove(bufferFile)
	
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
	
	// Verify buffer file was written
	if _, err := os.Stat(bufferFile); os.IsNotExist(err) {
		t.Error("Expected buffer file to be created")
	}
	
	// Clear and verify
	display.Clear()
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