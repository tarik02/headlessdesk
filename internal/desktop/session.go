package desktop

import (
	"errors"
	"image"
	"sync"
)

var ErrCroppedOutputUnavailable = errors.New("cropped output is unavailable")

// Status describes the current remote desktop session and framebuffer state.
type Status struct {
	Protocol       string   `json:"protocol"`
	Connected      bool     `json:"connected"`
	Active         bool     `json:"active"`
	Ready          bool     `json:"ready"`
	State          string   `json:"state"`
	Version        string   `json:"version"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Error          string   `json:"error,omitempty"`
	InputBackend   string   `json:"input_backend,omitempty"`
	OutputBackend  string   `json:"output_backend,omitempty"`
	InputProtocol  string   `json:"input_protocol,omitempty"`
	OutputProtocol string   `json:"output_protocol,omitempty"`
	InputReady     bool     `json:"input_ready"`
	OutputReady    bool     `json:"output_ready"`
	InputWidth     int      `json:"input_width,omitempty"`
	InputHeight    int      `json:"input_height,omitempty"`
	OutputWidth    int      `json:"output_width,omitempty"`
	OutputHeight   int      `json:"output_height,omitempty"`
	InputError     string   `json:"input_error,omitempty"`
	OutputError    string   `json:"output_error,omitempty"`
	InputRegions   []Region `json:"input_regions,omitempty"`
	OutputRegions  []Region `json:"output_regions,omitempty"`
	Regions        []Region `json:"regions,omitempty"`
}

type Crop struct {
	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`
	W *int `json:"w,omitempty"`
	H *int `json:"h,omitempty"`
}

type Region struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Resource interface {
	Done() <-chan struct{}
	Err() error
	Close() error
}

type Component interface {
	Resource
	Status() Status
}

type OutputBackend interface {
	Component
	Screenshot() (image.Image, error)
}

type CroppedOutputBackend interface {
	ScreenshotCrop(crop Crop) (image.Image, error)
}

type InputBackend interface {
	Component
	SendKey(name string, down bool, repeat bool) error
	SendKeyScancode(scancode uint32, down bool, repeat bool) error
	TypeText(text string) error
	MoveMouse(x int, y int) error
	SendMouseButton(button string, x int, y int, down bool) error
	SendMouseWheel(x int, y int, delta int, horizontal bool) error
}

type CoordinateMapper interface {
	MapInputPoint(outputWidth int, outputHeight int, x int, y int) (int, int, error)
}

// Session is a long-lived remote desktop connection.
type Session interface {
	OutputBackend
	InputBackend
}

type Composite struct {
	inputName  string
	outputName string
	input      InputBackend
	output     OutputBackend
	resources  []Resource
	done       chan struct{}
	closeOnce  sync.Once
	doneOnce   sync.Once
	errMu      sync.RWMutex
	err        error
}

func NewComposite(inputName string, input InputBackend, outputName string, output OutputBackend) *Composite {
	outputResource := Resource(output)
	inputResource := Resource(input)
	resources := []Resource{outputResource}
	if inputResource != outputResource {
		resources = append(resources, inputResource)
	}
	c := &Composite{
		inputName:  inputName,
		outputName: outputName,
		input:      input,
		output:     output,
		resources:  resources,
		done:       make(chan struct{}),
	}
	for _, resource := range resources {
		go c.watch(resource)
	}
	return c
}

func (c *Composite) Status() Status {
	output := c.output.Status()
	input := c.input.Status()

	output.InputBackend = c.inputName
	output.OutputBackend = c.outputName
	output.InputProtocol = input.Protocol
	output.OutputProtocol = output.Protocol
	output.InputReady = componentHealthy(input)
	output.OutputReady = componentHealthy(output)
	output.InputWidth = input.Width
	output.InputHeight = input.Height
	output.OutputWidth = output.Width
	output.OutputHeight = output.Height
	output.InputError = input.Error
	output.OutputError = output.Error
	output.InputRegions = append([]Region(nil), input.Regions...)
	output.OutputRegions = append([]Region(nil), output.Regions...)
	output.Error = joinErrors(input.Error, output.Error)
	return output
}

func (c *Composite) Done() <-chan struct{} {
	return c.done
}

func (c *Composite) Err() error {
	c.errMu.RLock()
	defer c.errMu.RUnlock()
	if c.err != nil {
		return c.err
	}

	var errs []error
	for _, resource := range c.resources {
		if err := resource.Err(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (c *Composite) Close() error {
	c.closeOnce.Do(func() {
		for _, resource := range c.resources {
			if err := resource.Close(); err != nil {
				c.setErr(err)
			}
		}
		c.closeDone()
	})
	return c.Err()
}

func (c *Composite) watch(resource Resource) {
	<-resource.Done()
	if err := resource.Err(); err != nil {
		c.setErr(err)
	}
	c.closeDone()
}

func (c *Composite) setErr(err error) {
	if err == nil {
		return
	}
	c.errMu.Lock()
	defer c.errMu.Unlock()
	c.err = errors.Join(c.err, err)
}

func (c *Composite) closeDone() {
	c.doneOnce.Do(func() {
		close(c.done)
	})
}

func joinErrors(values ...string) string {
	var err error
	for _, value := range values {
		if value != "" {
			err = errors.Join(err, errors.New(value))
		}
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

func componentHealthy(status Status) bool {
	return status.Connected && status.Active && status.Ready && status.Error == ""
}
