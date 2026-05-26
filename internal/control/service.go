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

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

type Status = desktop.Status

type Crop = desktop.Crop

type Service struct {
	status desktop.Component
	output desktop.OutputBackend
	input  desktop.InputBackend
}

const keypressDuration = 20 * time.Millisecond
const clickDuration = 20 * time.Millisecond
const doubleClickPause = 120 * time.Millisecond
const dragStepPause = 16 * time.Millisecond

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
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

func NewService(status desktop.Component, output desktop.OutputBackend, input desktop.InputBackend) *Service {
	return &Service{status: status, output: output, input: input}
}

func (s *Service) Status() Status {
	return s.status.Status()
}

func (s *Service) Screenshot(cmd ScreenshotCommand) ([]byte, error) {
	var img image.Image
	var err error
	if cmd.Crop != nil {
		if output, ok := s.output.(desktop.CroppedOutputBackend); ok {
			img, err = output.ScreenshotCrop(*cmd.Crop)
			if err == nil || !errors.Is(err, desktop.ErrCroppedOutputUnavailable) {
				return encodePNG(img, err)
			}
		}
	}

	img, err = s.output.Screenshot()
	if err != nil || cmd.Crop == nil {
		return encodePNG(img, err)
	}
	img, err = cropImage(img, *cmd.Crop)
	return encodePNG(img, err)
}

func (s *Service) Click(cmd ClickCommand) error {
	buttonName, err := inputcode.ParseMouseButtonName(cmd.Button)
	if err != nil {
		if strings.TrimSpace(cmd.Button) == "" {
			return errors.New("button is required")
		}
		return err
	}
	if buttonName.String() == "" {
		return errors.New("button is required")
	}
	x, y, err := s.mapInputPoint(cmd.X, cmd.Y)
	if err != nil {
		return err
	}
	if err := s.input.MoveMouse(x, y); err != nil {
		return err
	}
	if err := s.input.SendMouseButton(buttonName, x, y, true); err != nil {
		return err
	}
	time.Sleep(clickDuration)
	return s.input.SendMouseButton(buttonName, x, y, false)
}

func (s *Service) DoubleClick(cmd DoubleClickCommand) error {
	if err := s.Click(ClickCommand(cmd)); err != nil {
		return err
	}
	time.Sleep(doubleClickPause)
	return s.Click(ClickCommand(cmd))
}

func (s *Service) Drag(cmd DragCommand) error {
	if len(cmd.Path) < 2 {
		return errors.New("path must contain at least two points")
	}
	path, err := s.mapInputPath(cmd.Path)
	if err != nil {
		return err
	}
	start := path[0]
	leftButton, err := inputcode.ParseMouseButtonName("left")
	if err != nil {
		return err
	}
	if err := s.input.MoveMouse(start.X, start.Y); err != nil {
		return err
	}
	if err := s.input.SendMouseButton(leftButton, start.X, start.Y, true); err != nil {
		return err
	}
	time.Sleep(clickDuration)
	for _, point := range path[1:] {
		if err := s.input.MoveMouse(point.X, point.Y); err != nil {
			_ = s.input.SendMouseButton(leftButton, point.X, point.Y, false)
			return err
		}
		time.Sleep(dragStepPause)
	}
	end := path[len(path)-1]
	return s.input.SendMouseButton(leftButton, end.X, end.Y, false)
}

func (s *Service) Move(cmd MoveCommand) error {
	x, y, err := s.mapInputPoint(cmd.X, cmd.Y)
	if err != nil {
		return err
	}
	return s.input.MoveMouse(x, y)
}

func (s *Service) Scroll(cmd ScrollCommand) error {
	if cmd.ScrollX == 0 && cmd.ScrollY == 0 {
		return errors.New("scrollX or scrollY is required")
	}
	x, y, err := s.mapInputPoint(cmd.X, cmd.Y)
	if err != nil {
		return err
	}
	if cmd.ScrollY != 0 {
		if err := s.input.SendMouseWheel(x, y, cmd.ScrollY, false); err != nil {
			return err
		}
	}
	if cmd.ScrollX != 0 {
		if err := s.input.SendMouseWheel(x, y, cmd.ScrollX, true); err != nil {
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
	keyName, err := inputcode.ParseKeyName(key)
	if err != nil {
		return err
	}
	if err := s.input.SendKey(keyName, true, false); err != nil {
		return err
	}
	time.Sleep(keypressDuration)
	return s.input.SendKey(keyName, false, false)
}

func (s *Service) Type(cmd TypeCommand) error {
	if cmd.Text == "" {
		return errors.New("text is required")
	}
	return s.input.TypeText(cmd.Text)
}

func (s *Service) Wait(cmd WaitCommand) error {
	if cmd.Duration < 0 {
		return errors.New("duration must be non-negative")
	}
	time.Sleep(cmd.Duration)
	return nil
}

func cropImage(img image.Image, crop Crop) (image.Image, error) {
	rect, err := cropBounds(img.Bounds(), crop)
	if err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, rect.Min, draw.Src)
	return dst, nil
}

func encodePNG(img image.Image, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, errors.New("screenshot image is nil")
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode screenshot png: %w", err)
	}
	return out.Bytes(), nil
}

func (s *Service) mapInputPath(path []Point) ([]Point, error) {
	mapped := make([]Point, len(path))
	for i, point := range path {
		x, y, err := s.mapInputPoint(point.X, point.Y)
		if err != nil {
			return nil, err
		}
		mapped[i] = Point{X: x, Y: y}
	}
	return mapped, nil
}

func (s *Service) mapInputPoint(x int, y int) (int, int, error) {
	mapper, ok := s.input.(desktop.CoordinateMapper)
	if !ok {
		return x, y, nil
	}
	output := s.output.Status()
	return mapper.MapInputPoint(output.Width, output.Height, x, y)
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
