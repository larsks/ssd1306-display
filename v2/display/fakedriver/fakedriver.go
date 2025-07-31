package fakedriver

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"periph.io/x/devices/v3/ssd1306/image1bit"
)

//go:embed display.html
var displayTemplate embed.FS

type FakeSSD1306 struct {
	bounds        image.Rectangle
	mutex         sync.Mutex
	buffer        *image.RGBA
	server        *http.Server
	listenAddress string
	port          uint
	clients       map[chan string]bool
	waitMode      bool
	startChan     chan bool
	started       bool
}

func getEnvWithDefault(name, defval string) string {
	val := os.Getenv(name)
	if val == "" {
		return defval
	}
	return val
}

func NewFakeSSD1306() *FakeSSD1306 {
	listenAddress := getEnvWithDefault("FAKESSD1306_LISTEN_ADDRESS", "127.0.0.1")
	portStr := getEnvWithDefault("FAKESSD1306_PORT", "8080")

	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		port = 8080
		log.Printf("invalid port %s: using default port %d", portStr, port)
	}

	return &FakeSSD1306{
		bounds:        image.Rect(0, 0, 128, 64),
		listenAddress: listenAddress,
		port:          uint(port),
		clients:       make(map[chan string]bool),
		startChan:     make(chan bool, 1),
	}
}

func (f *FakeSSD1306) WithPort(port uint) *FakeSSD1306 {
	f.port = port
	return f
}

func (f *FakeSSD1306) WithListenAddress(addr string) *FakeSSD1306 {
	f.listenAddress = addr
	return f
}

func (d *FakeSSD1306) SetWaitMode(waitMode bool) {
	d.waitMode = waitMode
}

func (d *FakeSSD1306) IsWaitMode() bool {
	return d.waitMode
}

func (d *FakeSSD1306) WaitForStart() {
	if d.waitMode && !d.started {
		<-d.startChan
		d.started = true
	}
}

func (d *FakeSSD1306) Open() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Initialize the buffer
	d.buffer = image.NewRGBA(d.bounds)

	// Fill with black (OLED background)
	for y := d.bounds.Min.Y; y < d.bounds.Max.Y; y++ {
		for x := d.bounds.Min.X; x < d.bounds.Max.X; x++ {
			d.buffer.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.handleDisplay)
	mux.HandleFunc("/events", d.handleSSE)
	mux.HandleFunc("/start", d.handleStart)

	d.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", d.listenAddress, d.port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("SSD1306 Display Simulator running at http://localhost:%d", d.port)
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

func (d *FakeSSD1306) Close() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.server != nil {
		// Clear clients map without closing channels to avoid panic
		d.clients = make(map[chan string]bool)

		// Force close the server immediately - don't wait for graceful shutdown
		err := d.server.Close()
		d.server = nil
		return err
	}
	return nil
}

func (d *FakeSSD1306) Bounds() image.Rectangle {
	return d.bounds
}

func (d *FakeSSD1306) Draw(r image.Rectangle, src image.Image, sp image.Point) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.buffer == nil {
		return fmt.Errorf("display not initialized")
	}

	// Convert the source image to our display buffer
	// The src is typically a 1-bit image from the display library
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			srcX := sp.X + (x - r.Min.X)
			srcY := sp.Y + (y - r.Min.Y)

			if srcX >= src.Bounds().Min.X && srcX < src.Bounds().Max.X &&
				srcY >= src.Bounds().Min.Y && srcY < src.Bounds().Max.Y {

				srcColor := src.At(srcX, srcY)

				// Convert 1-bit color to RGB
				var displayColor color.RGBA
				if srcColor == image1bit.On {
					// White pixel (LED on)
					displayColor = color.RGBA{255, 255, 255, 255}
				} else {
					// Black pixel (LED off)
					displayColor = color.RGBA{0, 0, 0, 255}
				}

				d.buffer.Set(x, y, displayColor)
			}
		}
	}

	// Notify all connected clients of the update
	d.notifyClients()

	return nil
}

func (d *FakeSSD1306) notifyClients() {
	// Convert buffer to base64 PNG for SSE
	var buf bytes.Buffer
	if err := png.Encode(&buf, d.buffer); err != nil {
		return
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Send to all connected clients
	for client := range d.clients {
		select {
		case client <- "image:" + b64:
		default:
			// Client channel is full or closed, remove it
			close(client)
			delete(d.clients, client)
		}
	}
}

func (d *FakeSSD1306) notifyStatus() {
	status := "waiting"
	if d.started {
		status = "started"
	}

	// Send status to all connected clients
	for client := range d.clients {
		select {
		case client <- "status:" + status:
		default:
			// Client channel is full or closed, remove it
			close(client)
			delete(d.clients, client)
		}
	}
}

func (d *FakeSSD1306) handleDisplay(w http.ResponseWriter, r *http.Request) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Convert buffer to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, d.buffer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Encode to base64
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Load and parse HTML template
	tmplContent, err := displayTemplate.ReadFile("display.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("display").Parse(string(tmplContent))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		ImageData string
	}{
		ImageData: b64,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("failed to render template: %v", err)
	}
}

func (d *FakeSSD1306) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a new client channel
	clientChan := make(chan string, 10)

	d.mutex.Lock()
	d.clients[clientChan] = true
	d.mutex.Unlock()

	// Send initial status and image
	d.mutex.Lock()
	// Send initial status
	status := "waiting"
	if d.started {
		status = "started"
	}
	clientChan <- "status:" + status

	// Send initial image
	var buf bytes.Buffer
	if d.buffer != nil {
		if err := png.Encode(&buf, d.buffer); err == nil {
			b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
			clientChan <- "image:" + b64
		}
	}
	d.mutex.Unlock()

	// Handle client disconnection
	defer func() {
		d.mutex.Lock()
		delete(d.clients, clientChan)
		d.mutex.Unlock()
		close(clientChan)
	}()

	// Stream updates to client
	for {
		select {
		case data, ok := <-clientChan:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (d *FakeSSD1306) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.started {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte("Already started")) //nolint:errcheck
		return
	}

	// Signal that start button was clicked
	select {
	case d.startChan <- true:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Started")) //nolint:errcheck
		// Notify all clients of status change
		go d.notifyStatus()
	default:
		// Channel is full
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte("Start signal already sent")) //nolint:errcheck
	}
}
