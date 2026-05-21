//go:build linux

package fusefs

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"headlessdesk/internal/control"
)

type Options struct {
	Debug bool
}

type Filesystem struct {
	service *control.Service
	options Options
}

type rootNode struct {
	fs.Inode
	service *control.Service
}

type readFile struct {
	fs.Inode
	read          func() ([]byte, error)
	appendNewline bool
}

type writeFile struct {
	fs.Inode
	write func([]byte) error
}

type cropDir struct {
	fs.Inode
	service *control.Service
}

type readHandle struct {
	content []byte
}

type writeHandle struct {
	mu     sync.Mutex
	buffer []byte
	write  func([]byte) error
	wrote  bool
	done   bool
	errno  syscall.Errno
}

func New(service *control.Service, options Options) *Filesystem {
	return &Filesystem{service: service, options: options}
}

func (f *Filesystem) Mount(mountpoint string) (*fuse.Server, error) {
	zero := time.Duration(0)
	root := &rootNode{service: f.service}
	return fs.Mount(mountpoint, root, &fs.Options{
		AttrTimeout:     &zero,
		EntryTimeout:    &zero,
		NegativeTimeout: &zero,
		MountOptions: fuse.MountOptions{
			Debug:         f.options.Debug,
			DisableXAttrs: true,
			FsName:        "headlessdesk",
			Name:          "headlessdesk",
		},
	})
}

func (r *rootNode) OnAdd(ctx context.Context) {
	r.addTextFile(ctx, "README.md", r.readme)
	r.addTextFile(ctx, "health.json", r.health)
	r.addTextFile(ctx, "status.json", r.health)
	r.addReadFile(ctx, "screenshot.png", r.screenshot)

	crop := &cropDir{service: r.service}
	r.AddChild("crop", r.NewPersistentInode(ctx, crop, fs.StableAttr{Mode: fuse.S_IFDIR}), false)

	input := &fs.Inode{}
	r.AddChild("input", r.NewPersistentInode(ctx, input, fs.StableAttr{Mode: fuse.S_IFDIR}), false)
	addWriteFile(ctx, input, "type", r.typeText)
	addWriteFile(ctx, input, "keypress", r.keypress)
	addWriteFile(ctx, input, "click.json", r.click)
	addWriteFile(ctx, input, "double_click.json", r.doubleClick)
	addWriteFile(ctx, input, "move.json", r.move)
	addWriteFile(ctx, input, "scroll.json", r.scroll)
	addWriteFile(ctx, input, "drag.json", r.drag)
}

func (r *rootNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFDIR | 0755
	return 0
}

func (r *rootNode) addReadFile(ctx context.Context, name string, read func() ([]byte, error)) {
	r.AddChild(name, r.NewPersistentInode(ctx, &readFile{read: read}, fs.StableAttr{Mode: fuse.S_IFREG}), false)
}

func (r *rootNode) addTextFile(ctx context.Context, name string, read func() ([]byte, error)) {
	r.AddChild(name, r.NewPersistentInode(ctx, &readFile{read: read, appendNewline: true}, fs.StableAttr{Mode: fuse.S_IFREG}), false)
}

func addWriteFile(ctx context.Context, dir *fs.Inode, name string, write func([]byte) error) {
	dir.AddChild(name, dir.NewPersistentInode(ctx, &writeFile{write: write}, fs.StableAttr{Mode: fuse.S_IFREG}), false)
}

func (r *rootNode) health() ([]byte, error) {
	return json.MarshalIndent(r.service.Status(), "", "  ")
}

func (r *rootNode) readme() ([]byte, error) {
	return []byte(`# headlessdesk fuse

- health.json: current backend status as JSON.
- status.json: same as health.json.
- screenshot.png: latest full screenshot as PNG.
- crop/<x>,<y>,<w>,<h>.png: cropped screenshot as PNG, for example crop/100,100,400,300.png.
- input/type: write raw text to type.
- input/keypress: write a key name to press and release.
- input/click.json: write {"x":640,"y":360,"button":"left"}.
- input/double_click.json: write {"x":640,"y":360,"button":"left"}.
- input/move.json: write {"x":640,"y":360}.
- input/scroll.json: write {"x":640,"y":360,"scrollY":120}.
- input/drag.json: write {"path":[{"x":1,"y":1},{"x":2,"y":2}]}.
`), nil
}

func (r *rootNode) screenshot() ([]byte, error) {
	return r.service.Screenshot(control.ScreenshotCommand{})
}

func (r *rootNode) typeText(data []byte) error {
	return r.service.Type(control.TypeCommand{Text: string(data)})
}

func (r *rootNode) keypress(data []byte) error {
	return r.service.Keypress(control.KeypressCommand{Key: string(data)})
}

func (r *rootNode) click(data []byte) error {
	var cmd control.ClickCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}
	return r.service.Click(cmd)
}

func (r *rootNode) doubleClick(data []byte) error {
	var cmd control.DoubleClickCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}
	return r.service.DoubleClick(cmd)
}

func (r *rootNode) move(data []byte) error {
	var cmd control.MoveCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}
	return r.service.Move(cmd)
}

func (r *rootNode) scroll(data []byte) error {
	var cmd control.ScrollCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}
	return r.service.Scroll(cmd)
}

func (r *rootNode) drag(data []byte) error {
	var cmd control.DragCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}
	return r.service.Drag(cmd)
}

func (d *cropDir) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFDIR | 0755
	return 0
}

func (d *cropDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream(nil), 0
}

func (d *cropDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	crop, ok := parseCropFilename(name)
	if !ok {
		return nil, syscall.ENOENT
	}
	out.Mode = fuse.S_IFREG | 0444
	return d.NewInode(ctx, &readFile{
		read: func() ([]byte, error) {
			return d.service.Screenshot(control.ScreenshotCommand{Crop: &crop})
		},
	}, fs.StableAttr{Mode: fuse.S_IFREG, Ino: cropInode(name)}), 0
}

func (f *readFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFREG | 0444
	return 0
}

func (f *readFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if flags&syscall.O_ACCMODE != syscall.O_RDONLY {
		return nil, 0, syscall.EROFS
	}

	content, err := f.read()
	if err != nil {
		log.Printf("fuse read failed: %v", err)
		return nil, 0, syscall.EIO
	}
	if f.appendNewline && (len(content) == 0 || content[len(content)-1] != '\n') {
		content = append(content, '\n')
	}
	return &readHandle{content: content}, fuse.FOPEN_DIRECT_IO, 0
}

func (h *readHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if off < 0 {
		return nil, syscall.EINVAL
	}
	if off >= int64(len(h.content)) {
		return fuse.ReadResultData(nil), 0
	}
	end := off + int64(len(dest))
	if end > int64(len(h.content)) {
		end = int64(len(h.content))
	}
	return fuse.ReadResultData(h.content[off:end]), 0
}

func (f *writeFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFREG | 0222
	return 0
}

func (f *writeFile) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFREG | 0222
	return 0
}

func (f *writeFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if flags&syscall.O_ACCMODE == syscall.O_RDONLY {
		return nil, 0, syscall.EACCES
	}
	return &writeHandle{write: f.write}, fuse.FOPEN_DIRECT_IO, 0
}

func (h *writeHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	if off < 0 {
		return 0, syscall.EINVAL
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.wrote = true
	end := int(off) + len(data)
	if end > len(h.buffer) {
		next := make([]byte, end)
		copy(next, h.buffer)
		h.buffer = next
	}
	copy(h.buffer[off:], data)
	return uint32(len(data)), 0
}

func (h *writeHandle) Flush(ctx context.Context) syscall.Errno {
	return h.finish()
}

func (h *writeHandle) Release(ctx context.Context) syscall.Errno {
	return h.finish()
}

func (h *writeHandle) finish() syscall.Errno {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.wrote {
		return 0
	}
	if h.done {
		return h.errno
	}
	h.done = true
	if err := h.write(h.buffer); err != nil {
		log.Printf("fuse write failed: %v", err)
		h.errno = syscall.EINVAL
		return h.errno
	}
	return 0
}

func parseCropFilename(name string) (control.Crop, bool) {
	if filepath.Ext(name) != ".png" {
		return control.Crop{}, false
	}
	parts := strings.Split(strings.TrimSuffix(name, ".png"), ",")
	if len(parts) != 4 {
		return control.Crop{}, false
	}

	values := make([]int, 4)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return control.Crop{}, false
		}
		values[i] = value
	}
	return control.Crop{X: &values[0], Y: &values[1], W: &values[2], H: &values[3]}, true
}

func cropInode(name string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte("crop/"))
	_, _ = hash.Write([]byte(name))
	return 1<<62 | hash.Sum64()&(1<<62-1)
}

var _ fs.NodeOnAdder = (*rootNode)(nil)
var _ fs.NodeGetattrer = (*rootNode)(nil)
var _ fs.NodeGetattrer = (*cropDir)(nil)
var _ fs.NodeReaddirer = (*cropDir)(nil)
var _ fs.NodeLookuper = (*cropDir)(nil)
var _ fs.NodeGetattrer = (*readFile)(nil)
var _ fs.NodeOpener = (*readFile)(nil)
var _ fs.FileReader = (*readHandle)(nil)
var _ fs.NodeGetattrer = (*writeFile)(nil)
var _ fs.NodeSetattrer = (*writeFile)(nil)
var _ fs.NodeOpener = (*writeFile)(nil)
var _ fs.FileWriter = (*writeHandle)(nil)
var _ fs.FileFlusher = (*writeHandle)(nil)
var _ fs.FileReleaser = (*writeHandle)(nil)
