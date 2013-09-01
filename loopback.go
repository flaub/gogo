package main

import (
	"io"
	"log"
	"os"
	"path"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type FuseSubsystem struct {
	mnt     *fuse.Conn
	workDir string
	mntDir  string
}

func NewFuseSubsystem() *FuseSubsystem {
	workDir := "/var/lib/gogo"
	this := &FuseSubsystem{
		workDir: workDir,
		mntDir:  path.Join(workDir, "fuse"),
	}

	err := os.MkdirAll(this.workDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Could not create gogo work directory: %q", this.workDir)
	}

	err = os.MkdirAll(this.mntDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Could not create gogo fuse directory: %q", this.mntDir)
	}

	syscall.Unmount(this.mntDir, 0) // just in case
	this.mnt, err = fuse.Mount(this.mntDir)
	if err != nil {
		log.Fatal(err)
	}
	return this
}

func (this *FuseSubsystem) Destroy() {
	syscall.Unmount(this.mntDir, 0)
}

func (this *FuseSubsystem) Execute() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	return fs.Serve(this.mnt, NewLoopbackFS(pwd))
}

type LoopbackFS struct {
	root      *LoopbackNode
	ready     int
	onReady   chan bool
	onDestroy chan bool
}

type LoopbackNode struct {
	path string
	fs   FuseFS
}

type LoopbackHandle struct {
	file *os.File
	fs   FuseFS
}

func NewLoopbackFS(path string) *LoopbackFS {
	this := &LoopbackFS{
		onReady:   make(chan bool),
		onDestroy: make(chan bool),
	}
	this.root = NewLoopbackNode(this, path)
	return this
}

func NewLoopbackNode(fs FuseFS, path string) *LoopbackNode {
	return &LoopbackNode{
		path: path,
		fs:   fs,
	}
}

func NewLoopbackHandle(fs FuseFS, file *os.File) *LoopbackHandle {
	return &LoopbackHandle{
		file: file,
		fs:   fs,
	}
}

func (this *LoopbackFS) Root() (fs.Node, fuse.Error) {
	log.Printf("LoopbackFS.Root> %v", this.root.path)
	return this.root, nil
}

func (this *LoopbackFS) Init(req *fuse.InitRequest, resp *fuse.InitResponse, intr fs.Intr) fuse.Error {
	log.Printf("Init> %v", req)
	return nil
}

func (this *LoopbackFS) Statfs(req *fuse.StatfsRequest, resp *fuse.StatfsResponse, intr fs.Intr) fuse.Error {
	// log.Printf("Statfs> %v", req)
	// FIXME: this is a hack to let clients know when a mount is ready to be used
	if this.ready < 2 {
		this.ready++
	}
	if this.ready == 2 && this.onReady != nil {
		close(this.onReady)
		this.onReady = nil
	}

	var st syscall.Statfs_t
	err := syscall.Statfs(this.root.path, &st)
	if err != nil {
		log.Printf("Statfs> failed: %v", err)
		return fuse.EIO
	}

	resp.Blocks = st.Blocks
	resp.Bfree = st.Bfree
	resp.Bavail = st.Bavail
	resp.Files = st.Files
	resp.Ffree = st.Ffree
	resp.Bsize = st.Bsize
	// resp.Namelen =
	// resp.Frsize =

	return nil
}

func (this *LoopbackFS) Destroy() {
	log.Printf("Destroy>")
	close(this.onDestroy)
}

func (this *LoopbackFS) WaitReady() {
	<-this.onReady
}

func (this *LoopbackFS) WaitDestroy() {
	<-this.onDestroy
}

func (this *LoopbackFS) NewNode(path string) fs.Node {
	return NewLoopbackNode(this, path)
}

func (this *LoopbackFS) NewHandle(file *os.File) fs.Handle {
	return NewLoopbackHandle(this, file)
}

// func (this *LoopbackNode) Getattr(req *fuse.GetattrRequest, resp *fuse.GetattrResponse, intr fs.Intr) fuse.Error {
// 	log.Printf("%q %v", this.path, req)
// 	resp.AttrValid = 1 * time.Minute
// 	resp.Attr = this.Attr()
// 	return nil
// }

func (this *LoopbackNode) Attr() fuse.Attr {
	// log.Printf("Attr> %q", this.path)
	var st syscall.Stat_t
	err := syscall.Stat(this.path, &st)
	if err != nil {
		log.Printf("Attr> failed: %v", err)
		return fuse.Attr{}
	}
	return statToAttr(&st)
}

func (this *LoopbackNode) Lookup(req *fuse.LookupRequest, resp *fuse.LookupResponse, intr fs.Intr) (fs.Node, fuse.Error) {
	// log.Print(req)
	path := path.Join(this.path, req.Name)
	var st syscall.Stat_t
	err := syscall.Stat(path, &st)
	if err != nil {
		// log.Printf("ENOENT: %v", req)
		return nil, fuse.ENOENT
	}
	return this.fs.NewNode(path), nil
}

func (this *LoopbackNode) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
	// log.Printf("LoopbackNode> %q %v", this.path, req)
	file, err := os.OpenFile(this.path, int(req.Flags), 0)
	if err != nil {
		log.Printf("Open> failed: %v", err)
		return nil, fuse.EIO
	}
	resp.Flags = 0
	return this.fs.NewHandle(file), nil
}

func (this *LoopbackNode) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	// log.Printf("%q %v", this.path, req)
	path := path.Join(this.path, req.Name)
	file, err := os.OpenFile(path, int(req.Flags)|os.O_CREATE, req.Mode)
	if err != nil {
		log.Printf("Create> failed: %v", err)
		return nil, nil, fuse.EIO
	}
	resp.Flags = 0
	return this.fs.NewNode(path), this.fs.NewHandle(file), nil
}

func (this *LoopbackNode) Setattr(req *fuse.SetattrRequest, resp *fuse.SetattrResponse, intr fs.Intr) fuse.Error {
	// log.Printf("Setattr> %q", this.path)
	if req.Valid.Mode() {
		err := this.Chmod(req)
		if err != nil {
			return err
		}
	}
	if req.Valid.Uid() || req.Valid.Gid() {
		err := this.Chown(req)
		if err != nil {
			return err
		}
	}
	if req.Valid.Size() {
		err := this.Truncate(req)
		if err != nil {
			return err
		}
	}
	if req.Valid.Atime() || req.Valid.Mtime() {
		err := this.Chtimes(req)
		if err != nil {
			return err
		}
	}
	resp.AttrValid = 1 * time.Minute
	resp.Attr = this.Attr()
	return nil
}

func (this *LoopbackNode) Chmod(req *fuse.SetattrRequest) fuse.Error {
	// log.Printf("Setattr> %q Mode: %v", this.path, req.Mode)
	err := os.Chmod(this.path, req.Mode)
	if err != nil {
		log.Printf("Setattr> os.Chmod() failed: %v", err)
		return fuse.EIO
	}
	return nil
}

func (this *LoopbackNode) Chown(req *fuse.SetattrRequest) fuse.Error {
	// log.Printf("Setattr> %q uid: %v gid %v", this.path, req.Uid, req.Gid)
	err := os.Chown(this.path, int(req.Uid), int(req.Gid))
	if err != nil {
		log.Printf("Setattr> os.Chown() failed: %v", err)
		return fuse.EIO
	}
	return nil
}

func (this *LoopbackNode) Chtimes(req *fuse.SetattrRequest) fuse.Error {
	fi, err := os.Stat(this.path)
	if err != nil {
		log.Printf("Setattr> os.Stat() failed: %v", err)
		return fuse.EIO
	}
	st, _ := fi.Sys().(*syscall.Stat_t)
	var atime time.Time
	if req.Valid.Atime() {
		// log.Printf("Setattr> %q Atime: %v", this.path, req.Atime)
		atime = req.Atime
	} else {
		atime = time.Unix(st.Atimespec.Unix())
	}
	var mtime time.Time
	if req.Valid.Mtime() {
		// log.Printf("Setattr> %q Mtime: %v", this.path, req.Mtime)
		mtime = req.Mtime
	} else {
		mtime = time.Unix(st.Mtimespec.Unix())
	}
	err = os.Chtimes(this.path, atime, mtime)
	if err != nil {
		log.Printf("Setattr> os.Chtimes() failed: %v", err)
		return fuse.EIO
	}
	return nil
}

func (this *LoopbackNode) Truncate(req *fuse.SetattrRequest) fuse.Error {
	// log.Printf("Setattr> %q Size: %v", this.path, req.Size)
	err := os.Truncate(this.path, int64(req.Size))
	if err != nil {
		log.Printf("Setattr> os.Truncate() failed: %v", err)
		return fuse.EIO
	}
	return nil
}

func (this *LoopbackNode) Symlink(req *fuse.SymlinkRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	// log.Printf("Symlink> %q -> %q", this.path, req.NewName)
	err := os.Symlink(this.path, req.NewName)
	if err != nil {
		log.Printf("Symlink> failed: %v", err)
		return nil, fuse.EIO
	}
	return this.fs.NewNode(req.NewName), nil
}

func (this *LoopbackNode) Readlink(req *fuse.ReadlinkRequest, intr fs.Intr) (string, fuse.Error) {
	// log.Printf("%q %v", this.path, req)
	result, err := os.Readlink(this.path)
	if err != nil {
		log.Printf("Readlink> failed: %v", err)
		return "", fuse.EIO
	}
	return result, nil
}

func (this *LoopbackNode) Link(req *fuse.LinkRequest, old fs.Node, intr fs.Intr) (fs.Node, fuse.Error) {
	// log.Printf("%q %v", this.path, req)
	err := os.Link(this.path, req.NewName)
	if err != nil {
		log.Printf("Link> failed: %v", err)
		return nil, fuse.EIO
	}
	return this.fs.NewNode(req.NewName), nil
}

func (this *LoopbackNode) Remove(req *fuse.RemoveRequest, intr fs.Intr) fuse.Error {
	// log.Printf("%q %v", this.path, req)
	path := path.Join(this.path, req.Name)
	err := os.Remove(path)
	if err != nil {
		log.Printf("Remove> failed: %v", err)
		return fuse.EIO
	}
	return nil
}

func (this *LoopbackNode) Access(req *fuse.AccessRequest, intr fs.Intr) fuse.Error {
	// log.Printf("Access> %q %v", this.path, req)
	err := syscall.Access(this.path, req.Mask)
	if err != nil {
		log.Printf("Access> failed :%v", err)
		return fuse.EPERM
	}
	return nil
}

func (this *LoopbackNode) Mkdir(req *fuse.MkdirRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	// log.Printf("%q %v", this.path, req)
	path := path.Join(this.path, req.Name)
	err := os.Mkdir(path, req.Mode)
	if err != nil {
		log.Printf("Mkdir> failed: %v", err)
		return nil, fuse.EIO
	}
	return this.fs.NewNode(path), fuse.ENOSYS
}

func (this *LoopbackNode) Rename(req *fuse.RenameRequest, newDir fs.Node, intr fs.Intr) fuse.Error {
	// log.Printf("Rename> %q -> %q", this.path, req.NewName)
	err := os.Rename(this.path, req.NewName)
	if err != nil {
		log.Printf("Rename> failed: %v", err)
		return fuse.EIO
	}
	newDir = this.fs.NewNode(req.NewName)
	return nil
}

func (this *LoopbackNode) Mknod(req *fuse.MknodRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	// log.Printf("%q %v", this.path, req)
	path := path.Join(this.path, req.Name)
	err := syscall.Mknod(path, uint32(req.Mode), int(req.Rdev))
	if err != nil {
		log.Printf("Mknod> failed: %v", err)
		return nil, fuse.EIO
	}
	return this.fs.NewNode(path), nil
}

func (this *LoopbackNode) Fsync(req *fuse.FsyncRequest, intr fs.Intr) fuse.Error {
	// log.Printf("%q %v", this.path, req)
	return fuse.ENOSYS
}

func (this *LoopbackNode) Forget() {
	// log.Printf("Forget> %q", this.path)
}

func (this *LoopbackHandle) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	// log.Printf("ReadDir> %q", this.file.Name())

	want := 500
	output := make([]fuse.Dirent, 0, want)

	for {
		infos, err := this.file.Readdir(want)

		for _, info := range infos {
			if info == nil {
				continue
			}
			output = append(output, fileInfoToDirent(info))
		}

		if len(infos) < want || err == io.EOF {
			break
		}

		if err != nil {
			log.Printf("ReadDir> Readdir() failed: %v", err)
			break
		}
	}

	return output, nil
}

func (this *LoopbackHandle) Read(req *fuse.ReadRequest, resp *fuse.ReadResponse, intr fs.Intr) fuse.Error {
	// log.Printf("%q %v", this.file.Name(), req)
	resp.Data = make([]byte, req.Size)
	n, err := this.file.ReadAt(resp.Data, req.Offset)
	if err != nil && err != io.EOF {
		log.Printf("Read> failed: %v", err)
		return fuse.EIO
	}
	resp.Data = resp.Data[:n]
	return nil
}

func (this *LoopbackHandle) Write(req *fuse.WriteRequest, resp *fuse.WriteResponse, intr fs.Intr) fuse.Error {
	// log.Printf("%q %v", this.file.Name(), req)
	var n int
	var err error
	// if req.Offset == 0 {
	// 	n, err = this.file.Write(req.Data)
	// } else {
	n, err = this.file.WriteAt(req.Data, req.Offset)
	// }
	if err != nil {
		log.Printf("Write> failed: %v", err)
		return fuse.EIO
	}
	resp.Size = n
	return nil
}

func (this *LoopbackHandle) Flush(req *fuse.FlushRequest, intr fs.Intr) fuse.Error {
	// log.Printf("%q %v", this.file.Name(), req)
	err := this.file.Sync()
	if err != nil {
		log.Printf("Flush> failed: %v", err)
		return fuse.EIO
	}
	return nil
}

func (this *LoopbackHandle) Release(req *fuse.ReleaseRequest, intr fs.Intr) fuse.Error {
	// log.Printf("%q %v", this.file.Name(), req)
	err := this.file.Close()
	if err != nil {
		log.Printf("Release> failed: %v", err)
		return fuse.EIO
	}
	return nil
}
