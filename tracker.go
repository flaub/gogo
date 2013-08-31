package main

import (
	"log"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type TrackerFS struct {
	LoopbackFS
	root *TrackerNode
}

type TrackerNode struct {
	LoopbackNode
}

type TrackerHandle struct {
	LoopbackHandle
}

func NewTrackerFS(path string) *TrackerFS {
	return &TrackerFS{
		LoopbackFS: *NewLoopbackFS(path),
		root:       NewTrackerNode(path),
	}
}

func NewTrackerNode(path string) *TrackerNode {
	return &TrackerNode{
		LoopbackNode: *NewLoopbackNode(path),
	}
}

func NewTrackerHandle(h *LoopbackHandle) *TrackerHandle {
	return &TrackerHandle{
		LoopbackHandle: *h,
	}
}

func (this *TrackerFS) Root() (fs.Node, fuse.Error) {
	log.Printf("TrackerFS.Root> %v", this.root.path)
	return this.root, nil
}

func (this *TrackerNode) Getattr(req *fuse.GetattrRequest, resp *fuse.GetattrResponse, intr fs.Intr) fuse.Error {
	log.Printf("TrackerNode> %q %v", this.path, req)
	resp.AttrValid = 1 * time.Minute
	resp.Attr = this.Attr()
	// ACCESS_READ | ACCESS_VAR
	return nil
}

func (this *TrackerNode) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	log.Printf("TrackerNode> %q %v", this.path, req)
	// ACCESS_WRITE
	node, handle, err := this.LoopbackNode.Create(req, resp, intr)
	if err != nil {
		return node, handle, err
	}
	return node, NewTrackerHandle(handle.(*LoopbackHandle)), err
}

func (this *TrackerNode) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
	log.Printf("TrackerNode> %q %v", this.path, req)
	handle, err := this.LoopbackNode.Open(req, resp, intr)
	if err != nil {
		return handle, err
	}
	// ACCESS_READ | ACCESS_WRITE
	return NewTrackerHandle(handle.(*LoopbackHandle)), err
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
