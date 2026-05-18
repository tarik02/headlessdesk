package desktop

// Status describes the current remote desktop session and framebuffer state.
type Status struct {
	Protocol  string `json:"protocol"`
	Connected bool   `json:"connected"`
	Active    bool   `json:"active"`
	Ready     bool   `json:"ready"`
	State     string `json:"state"`
	Version   string `json:"version"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Error     string `json:"error,omitempty"`
}

// Backend is the protocol-neutral control surface exposed to REST and MCP APIs.
type Backend interface {
	Status() Status
	ScreenshotPNG() ([]byte, error)
	SendKey(name string, down bool, repeat bool) error
	SendKeyScancode(scancode uint32, down bool, repeat bool) error
	TypeText(text string) error
	MoveMouse(x int, y int) error
	SendMouseButton(button string, x int, y int, down bool) error
	SendMouseWheel(x int, y int, delta int, horizontal bool) error
}

// Session is a long-lived remote desktop connection.
type Session interface {
	Backend
	Done() <-chan struct{}
	Err() error
	Close() error
}
