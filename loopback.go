package main

import (
	"io"
	"log"
	"os"
	"path"
	"syscall"

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

func NewLoopbackFS(path string) *LoopbackFS {
	return &LoopbackFS{
		root:      NewLoopbackNode(path),
		onReady:   make(chan bool),
		onDestroy: make(chan bool),
	}
}

func (this *LoopbackFS) Root() (fs.Node, fuse.Error) {
	log.Printf("Root> %v", this.root.path)
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

type LoopbackNode struct {
	path    string
	onReady chan bool
}

func NewLoopbackNode(path string) *LoopbackNode {
	return &LoopbackNode{
		path: path,
	}
}

func InfoToDirent(info os.FileInfo) fuse.Dirent {
	mode := fuse.DT_Unknown
	st, _ := info.Sys().(*syscall.Stat_t)
	if (st.Mode & syscall.S_IFMT) == syscall.S_IFSOCK {
		mode |= fuse.DT_Socket
	}
	if (st.Mode & syscall.S_IFMT) == syscall.S_IFLNK {
		mode |= fuse.DT_Link
	}
	if (st.Mode & syscall.S_IFMT) == syscall.S_IFREG {
		mode |= fuse.DT_File
	}
	if (st.Mode & syscall.S_IFMT) == syscall.S_IFBLK {
		mode |= fuse.DT_Block
	}
	if (st.Mode & syscall.S_IFMT) == syscall.S_IFDIR {
		mode |= fuse.DT_Dir
	}
	if (st.Mode & syscall.S_IFMT) == syscall.S_IFCHR {
		mode |= fuse.DT_Char
	}
	if (st.Mode & syscall.S_IFMT) == syscall.S_IFIFO {
		mode |= fuse.DT_FIFO
	}
	// mode |= fuse.DirentType(st.Mode & 0x0777)
	return fuse.Dirent{
		Name:  info.Name(),
		Type:  mode,
		Inode: st.Ino,
	}
}

func (this *LoopbackNode) Attr() fuse.Attr {
	log.Printf("Attr> %q", this.path)
	if this.onReady != nil {
		close(this.onReady)
		this.onReady = nil
	}
	st := syscall.Stat_t{}
	err := syscall.Stat(this.path, &st)
	if err != nil {
		log.Printf("Attr> failed: %v", err)
		return fuse.Attr{}
	}
	return StatToAttr(&st)
}

func (this *LoopbackNode) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	path := path.Join(this.path, name)
	// log.Printf("Lookup> %q", path)
	file, err := os.Open(path)
	if err != nil {
		return nil, fuse.ENOENT
	}
	defer file.Close()
	return NewLoopbackNode(path), nil
}

func (this *LoopbackNode) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
	log.Printf("Open> %q (%v)", this.path, req)
	file, err := os.OpenFile(this.path, int(req.Flags), 0)
	if err != nil {
		log.Printf("Open> failed: %v", err)
		return nil, fuse.EIO
	}
	return NewLoopbackHandle(file), nil
}

func (this *LoopbackNode) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	path := path.Join(this.path, req.Name)
	log.Printf("Create> %q (0x%x, %v)", path, req.Flags, req.Mode)
	file, err := os.OpenFile(path, int(req.Flags)|os.O_CREATE, req.Mode)
	if err != nil {
		log.Printf("Create> failed: %v", err)
		return nil, nil, fuse.EIO
	}
	return NewLoopbackNode(path), NewLoopbackHandle(file), nil
}

func (this *LoopbackNode) Setattr(req *fuse.SetattrRequest, resp *fuse.SetattrResponse, intr fs.Intr) fuse.Error {
	log.Printf("Setattr> %q", this.path)
	if req.Valid.Mode() {
		log.Printf("Setattr> %q Mode: %v", this.path, req.Mode)
		err := os.Chmod(this.path, req.Mode)
		if err != nil {
			log.Printf("Setattr> os.Chmod() failed: %v", err)
			return fuse.EIO
		}
	}
	if req.Valid.Uid() || req.Valid.Gid() {
		log.Printf("Setattr> %q uid: %v gid %v", this.path, req.Uid, req.Gid)
		err := os.Chown(this.path, int(req.Uid), int(req.Gid))
		if err != nil {
			log.Printf("Setattr> os.Chown() failed: %v", err)
			return fuse.EIO
		}
	}
	if req.Valid.Size() {
		log.Printf("Setattr> %q Size: %v", this.path, req.Size)
		err := os.Truncate(this.path, int64(req.Size))
		if err != nil {
			log.Printf("Setattr> os.Truncate() failed: %v", err)
			return fuse.EIO
		}
	}
	if req.Valid.Atime() || req.Valid.Mtime() {
		log.Printf("Setattr> %q Atime: %v Mtime: %v", this.path, req.Atime, req.Mtime)
		err := os.Chtimes(this.path, req.Atime, req.Mtime)
		if err != nil {
			log.Printf("Setattr> os.Chtimes() failed: %v", err)
			return fuse.EIO
		}
	}
	return nil
}

func (this *LoopbackNode) Symlink(req *fuse.SymlinkRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Printf("Symlink> %q -> %q", this.path, req.NewName)
	err := os.Symlink(this.path, req.NewName)
	if err != nil {
		log.Printf("Symlink> failed: %v", err)
		return nil, fuse.EIO
	}
	return NewLoopbackNode(req.NewName), nil
}

func (this *LoopbackNode) Readlink(req *fuse.ReadlinkRequest, intr fs.Intr) (string, fuse.Error) {
	log.Printf("Readlink> %q", this.path)
	result, err := os.Readlink(this.path)
	if err != nil {
		log.Printf("Readlink> failed: %v", err)
		return "", fuse.EIO
	}
	return result, nil
}

func (this *LoopbackNode) Link(req *fuse.LinkRequest, old fs.Node, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Printf("Link> %q", this.path)
	err := os.Link(this.path, req.NewName)
	if err != nil {
		log.Printf("Link> failed: %v", err)
		return nil, fuse.EIO
	}
	return NewLoopbackNode(req.NewName), nil
}

func (this *LoopbackNode) Remove(req *fuse.RemoveRequest, intr fs.Intr) fuse.Error {
	path := path.Join(this.path, req.Name)
	log.Printf("Remove> %q", path)
	err := os.Remove(path)
	if err != nil {
		log.Printf("Remove> failed: %v", err)
		return fuse.EIO
	}
	return nil
}

// func (this *LoopbackNode) Access(req *fuse.AccessRequest, intr fs.Intr) fuse.Error {
// 	log.Printf("Access: %q", this.path)
// 	return nil
// }

func (this *LoopbackNode) Mkdir(req *fuse.MkdirRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	path := path.Join(this.path, req.Name)
	log.Printf("Mkdir> %q", path)
	err := os.Mkdir(path, req.Mode)
	if err != nil {
		log.Printf("Mkdir> failed: %v", err)
		return nil, fuse.EIO
	}
	return NewLoopbackNode(path), fuse.ENOSYS
}

func (this *LoopbackNode) Rename(req *fuse.RenameRequest, newDir fs.Node, intr fs.Intr) fuse.Error {
	log.Printf("Rename> %q -> %q", this.path, req.NewName)
	err := os.Rename(this.path, req.NewName)
	if err != nil {
		log.Printf("Rename> failed: %v", err)
		return fuse.EIO
	}
	newDir = NewLoopbackNode(req.NewName)
	return nil
}

func (this *LoopbackNode) Mknod(req *fuse.MknodRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	path := path.Join(this.path, req.Name)
	log.Printf("Mknod> %q", this.path)
	err := syscall.Mknod(path, uint32(req.Mode), int(req.Rdev))
	if err != nil {
		log.Printf("Mknod> failed: %v", err)
		return nil, fuse.EIO
	}
	return NewLoopbackNode(path), nil
}

func (this *LoopbackNode) Fsync(req *fuse.FsyncRequest, intr fs.Intr) fuse.Error {
	log.Printf("Fsync> %q", this.path)
	return fuse.ENOSYS
}

func (this *LoopbackNode) Forget() {
	log.Printf("Forget> %q", this.path)
}

type LoopbackHandle struct {
	file *os.File
}

func NewLoopbackHandle(file *os.File) *LoopbackHandle {
	return &LoopbackHandle{file}
}

func (this *LoopbackHandle) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	log.Printf("ReadDir> %q", this.file.Name())

	want := 500
	output := make([]fuse.Dirent, 0, want)

	for {
		infos, err := this.file.Readdir(want)

		for _, info := range infos {
			if info == nil {
				continue
			}
			output = append(output, InfoToDirent(info))
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
	log.Printf("Read> %q (%v)", this.file.Name(), req)
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
	log.Printf("Write> %q (%v)", this.file.Name(), req)
	var n int
	var err error
	if req.Offset == 0 {
		n, err = this.file.Write(req.Data)
	} else {
		n, err = this.file.WriteAt(req.Data, req.Offset)
	}
	if err != nil {
		log.Printf("Write> failed: %v", err)
		return fuse.EIO
	}
	resp.Size = n
	return nil
}

func (this *LoopbackHandle) Flush(req *fuse.FlushRequest, intr fs.Intr) fuse.Error {
	log.Printf("Flush> %q (%v)", this.file.Name(), req)
	err := this.file.Sync()
	if err != nil {
		log.Printf("Flush> failed: %v", err)
		return fuse.EIO
	}
	return nil
}

func (this *LoopbackHandle) Release(req *fuse.ReleaseRequest, intr fs.Intr) fuse.Error {
	log.Printf("Release> %q", this.file.Name())
	err := this.file.Close()
	if err != nil {
		log.Printf("Release> failed: %v", err)
		return fuse.EIO
	}
	return nil
}
