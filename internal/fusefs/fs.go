package fusefs

import (
	"context"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const fileName = "game.iso"

type rootNode struct {
	fs.Inode
	file  *gameNode
	mu    sync.Mutex
	child *fs.Inode
}

func (r *rootNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if name != fileName {
		return nil, syscall.ENOENT
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.child == nil {
		st := fs.StableAttr{Mode: fuse.S_IFREG}
		r.child = r.NewInode(ctx, r.file, st)
	}
	return r.child, syscall.F_OK
}

func (r *rootNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0o555 | fuse.S_IFDIR
	return syscall.F_OK
}

func (r *rootNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewListDirStream([]fuse.DirEntry{{Name: fileName, Mode: fuse.S_IFREG}}), syscall.F_OK
}

type gameNode struct {
	fs.Inode
	reader *Reader
	size   uint64
}

func (g *gameNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0o444 | fuse.S_IFREG
	out.Size = g.size
	out.Blocks = (g.size + 511) / 512
	return syscall.F_OK
}

func (g *gameNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if flags&syscall.O_ACCMODE != syscall.O_RDONLY {
		return nil, 0, syscall.EACCES
	}
	return nil, fuse.FOPEN_KEEP_CACHE, syscall.F_OK
}

func (g *gameNode) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := g.reader.ReadAt(dest, off)
	if err != nil {
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(dest[:n]), syscall.F_OK
}

func Mount(mountpoint string, reader *Reader, size int64) (*fuse.Server, error) {
	root := &rootNode{file: &gameNode{reader: reader, size: uint64(size)}}
	return fs.Mount(mountpoint, root, &fs.Options{
		MountOptions: fuse.MountOptions{Name: "cache22"},
	})
}
