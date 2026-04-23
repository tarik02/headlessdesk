package control

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"strings"
	"time"

	"libfreerdp-golang-poc/internal/freerdp"
)

type Status = freerdp.Status

type Backend interface {
	Status() freerdp.Status
	ScreenshotPNG() ([]byte, error)
	SendKey(name string, down bool, repeat bool) error
	SendKeyScancode(scancode uint32, down bool, repeat bool) error
	TypeText(text string) error
	MoveMouse(x int, y int) error
	SendMouseButton(button string, x int, y int, down bool) error
	SendMouseWheel(x int, y int, delta int, horizontal bool) error
}

type Service struct {
	backend Backend
}

const keypressDuration = 20 * time.Millisecond
const clickDuration = 20 * time.Millisecond
const doubleClickPause = 120 * time.Millisecond
const dragStepPause = 16 * time.Millisecond

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Crop struct {
	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`
	W *int `json:"w,omitempty"`
	H *int `json:"h,omitempty"`
}

type ScreenshotCommand struct {
	Crop *Crop `json:"crop,omitempty"`
}

type ClickCommand struct {
	X      int
	Y      int
	Button string
}

type DoubleClickCommand struct {
	X      int
	Y      int
	Button string
}

type DragCommand struct {
	Path []Point
}

type MoveCommand struct {
	X int
	Y int
}

type ScrollCommand struct {
	X       int
	Y       int
	ScrollX int
	ScrollY int
}

type KeypressCommand struct {
	Key string
}

type TypeCommand struct {
	Text string
}

type WaitCommand struct {
	Duration time.Duration
}

func NewService(backend Backend) *Service {
	return &Service{backend: backend}
}

func (s *Service) Status() Status {
	return s.backend.Status()
}

func (s *Service) Screenshot(cmd ScreenshotCommand) ([]byte, error) {
	pngData, err := s.backend.ScreenshotPNG()
	if err != nil || cmd.Crop == nil {
		return pngData, err
	}
	return cropPNG(pngData, *cmd.Crop)
}

func (s *Service) Click(cmd ClickCommand) error {
	button := strings.TrimSpace(cmd.Button)
	if button == "" {
		return errors.New("button is required")
	}
	if err := s.backend.MoveMouse(cmd.X, cmd.Y); err != nil {
		return err
	}
	if err := s.backend.SendMouseButton(button, cmd.X, cmd.Y, true); err != nil {
		return err
	}
	time.Sleep(clickDuration)
	return s.backend.SendMouseButton(button, cmd.X, cmd.Y, false)
}

func (s *Service) DoubleClick(cmd DoubleClickCommand) error {
	if err := s.Click(ClickCommand{X: cmd.X, Y: cmd.Y, Button: cmd.Button}); err != nil {
		return err
	}
	time.Sleep(doubleClickPause)
	return s.Click(ClickCommand{X: cmd.X, Y: cmd.Y, Button: cmd.Button})
}

func (s *Service) Drag(cmd DragCommand) error {
	if len(cmd.Path) < 2 {
		return errors.New("path must contain at least two points")
	}
	start := cmd.Path[0]
	if err := s.backend.MoveMouse(start.X, start.Y); err != nil {
		return err
	}
	if err := s.backend.SendMouseButton("left", start.X, start.Y, true); err != nil {
		return err
	}
	time.Sleep(clickDuration)
	for _, point := range cmd.Path[1:] {
		if err := s.backend.MoveMouse(point.X, point.Y); err != nil {
			_ = s.backend.SendMouseButton("left", point.X, point.Y, false)
			return err
		}
		time.Sleep(dragStepPause)
	}
	end := cmd.Path[len(cmd.Path)-1]
	return s.backend.SendMouseButton("left", end.X, end.Y, false)
}

func (s *Service) Move(cmd MoveCommand) error {
	return s.backend.MoveMouse(cmd.X, cmd.Y)
}

func (s *Service) Scroll(cmd ScrollCommand) error {
	if cmd.ScrollX == 0 && cmd.ScrollY == 0 {
		return errors.New("scrollX or scrollY is required")
	}
	if cmd.ScrollY != 0 {
		if err := s.backend.SendMouseWheel(cmd.X, cmd.Y, cmd.ScrollY, false); err != nil {
			return err
		}
	}
	if cmd.ScrollX != 0 {
		if err := s.backend.SendMouseWheel(cmd.X, cmd.Y, cmd.ScrollX, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Keypress(cmd KeypressCommand) error {
	key := strings.TrimSpace(cmd.Key)
	if key == "" {
		return errors.New("key is required")
	}
	if err := s.backend.SendKey(key, true, false); err != nil {
		return err
	}
	time.Sleep(keypressDuration)
	return s.backend.SendKey(key, false, false)
}

func (s *Service) Type(cmd TypeCommand) error {
	if cmd.Text == "" {
		return errors.New("text is required")
	}
	return s.backend.TypeText(cmd.Text)
}

func (s *Service) Wait(cmd WaitCommand) error {
	if cmd.Duration < 0 {
		return errors.New("duration must be non-negative")
	}
	time.Sleep(cmd.Duration)
	return nil
}

func cropPNG(pngData []byte, crop Crop) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}

	rect, err := cropBounds(img.Bounds(), crop)
	if err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, rect.Min, draw.Src)

	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, fmt.Errorf("encode cropped screenshot: %w", err)
	}
	return out.Bytes(), nil
}

func cropBounds(bounds image.Rectangle, crop Crop) (image.Rectangle, error) {
	left := bounds.Min.X
	top := bounds.Min.Y
	right := bounds.Max.X
	bottom := bounds.Max.Y

	if crop.X != nil {
		left = *crop.X
	}
	if crop.Y != nil {
		top = *crop.Y
	}
	if crop.W != nil {
		right = left + *crop.W
	}
	if crop.H != nil {
		bottom = top + *crop.H
	}

	if left < bounds.Min.X || top < bounds.Min.Y || right > bounds.Max.X || bottom > bounds.Max.Y {
		return image.Rectangle{}, fmt.Errorf("crop rectangle (%d,%d,%d,%d) is outside screenshot bounds %s", left, top, right, bottom, bounds)
	}
	if left >= right || top >= bottom {
		return image.Rectangle{}, errors.New("crop rectangle must have positive width and height")
	}

	return image.Rect(left, top, right, bottom), nil
}
