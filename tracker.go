package main

import (
	"log"
	"os"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type TrackerFS struct {
	*LoopbackFS
	root *TrackerNode
	pgid int
}

type TrackerNode struct {
	*LoopbackNode
}

type TrackerHandle struct {
	*LoopbackHandle
}

func NewTrackerFS(path string) *TrackerFS {
	pgid, err := syscall.Getpgid(0)
	if err != nil {
		panic("getpgid() failed")
	}

	this := &TrackerFS{
		LoopbackFS: NewLoopbackFS(path),
		pgid:       pgid,
	}
	this.root = NewTrackerNode(this, path)
	return this
}

func NewTrackerNode(fs *TrackerFS, path string) *TrackerNode {
	return &TrackerNode{
		LoopbackNode: NewLoopbackNode(fs, path),
	}
}

func NewTrackerHandle(fs *TrackerFS, file *os.File) *TrackerHandle {
	return &TrackerHandle{
		LoopbackHandle: NewLoopbackHandle(fs, file),
	}
}

func (this *TrackerFS) NewNode(path string) fs.Node {
	return NewTrackerNode(this, path)
}

func (this *TrackerFS) NewHandle(file *os.File) fs.Handle {
	return NewTrackerHandle(this, file)
}

func (this *TrackerFS) Root() (fs.Node, fuse.Error) {
	log.Printf("TrackerFS.Root> %v", this.root.path)
	return this.root, nil
}

func (this *TrackerFS) Filter(req fuse.Request) fuse.Error {
	pid := int(req.Hdr().Pid)
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		log.Printf("TrackerNode.Filter> getpgid(%d) failed: %v", pid, err)
		return fuse.EPERM
	}
	if pgid != this.pgid {
		// log.Printf("TrackerNode.Filter> %v this_pgid: %d, that_pgid: %d", req, this.pgid, pgid)
		return fuse.EPERM
	}
	// log.Printf("TrackerNode.Filter> %v", req)
	return nil
}

func (this *TrackerNode) Getattr(req *fuse.GetattrRequest, resp *fuse.GetattrResponse, intr fs.Intr) fuse.Error {
	resp.AttrValid = 1 * time.Minute
	resp.Attr = this.Attr()
	log.Printf("TrackerNode> %q %v", this.path, req)
	// ACCESS_READ | ACCESS_VAR
	return nil
}

func (this *TrackerNode) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	node, handle, err := this.LoopbackNode.Create(req, resp, intr)
	if err == nil {
		// ACCESS_WRITE
		log.Printf("TrackerNode> %q %v", this.path, req)
	}
	return node, handle, err
}

func (this *TrackerNode) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
	handle, err := this.LoopbackNode.Open(req, resp, intr)
	if err == nil {
		// ACCESS_READ | ACCESS_WRITE
		log.Printf("TrackerNode> %q %v", this.path, req)
	}
	return handle, err
}

func (this *TrackerNode) Mknod(req *fuse.MknodRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Printf("TrackerNode> %q %v", this.path, req)
	// ACCESS_WRITE
	return this.LoopbackNode.Mknod(req, intr)
}

func (this *TrackerNode) Symlink(req *fuse.SymlinkRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Printf("TrackerNode> %q %v", this.path, req)
	// ACCESS_WRITE
	return this.LoopbackNode.Symlink(req, intr)
}

func (this *TrackerNode) Readlink(req *fuse.ReadlinkRequest, intr fs.Intr) (string, fuse.Error) {
	log.Printf("TrackerNode> %q %v", this.path, req)
	// ACCESS_READ
	return this.LoopbackNode.Readlink(req, intr)
}

func (this *TrackerNode) Remove(req *fuse.RemoveRequest, intr fs.Intr) fuse.Error {
	log.Printf("TrackerNode> %q %v", this.path, req)
	// ACCESS_UNLINK
	return this.LoopbackNode.Remove(req, intr)
}

func (this *TrackerNode) Truncate(req *fuse.SetattrRequest) fuse.Error {
	log.Printf("TrackerNode> %q %v", this.path, req)
	// ACCESS_WRITE
	return this.LoopbackNode.Truncate(req)
}

func (this *TrackerHandle) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	log.Printf("TrackerHandle.ReadDir> %q", this.file.Name())
	// ACCESS_READ
	return this.LoopbackHandle.ReadDir(intr)
}
