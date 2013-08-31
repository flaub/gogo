package main

import (
	"os"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type FuseFS interface {
	Root() (fs.Node, fuse.Error)
	Init(req *fuse.InitRequest, resp *fuse.InitResponse, intr fs.Intr) fuse.Error
	Statfs(req *fuse.StatfsRequest, resp *fuse.StatfsResponse, intr fs.Intr) fuse.Error
	Destroy()
	WaitReady()
	WaitDestroy()
}

type FuseNode interface {
	Attr() fuse.Attr
	Getattr(req *fuse.GetattrRequest, resp *fuse.GetattrResponse, intr fs.Intr) fuse.Error
	Lookup(req *fuse.LookupRequest, resp *fuse.LookupResponse, intr fs.Intr) (fs.Node, fuse.Error)
	Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error)
	Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error)
	Setattr(req *fuse.SetattrRequest, resp *fuse.SetattrResponse, intr fs.Intr) fuse.Error
	Symlink(req *fuse.SymlinkRequest, intr fs.Intr) (fs.Node, fuse.Error)
	Readlink(req *fuse.ReadlinkRequest, intr fs.Intr) (string, fuse.Error)
	Link(req *fuse.LinkRequest, old fs.Node, intr fs.Intr) (fs.Node, fuse.Error)
	Remove(req *fuse.RemoveRequest, intr fs.Intr) fuse.Error
	Mkdir(req *fuse.MkdirRequest, intr fs.Intr) (fs.Node, fuse.Error)
	Rename(req *fuse.RenameRequest, newDir fs.Node, intr fs.Intr) fuse.Error
	Mknod(req *fuse.MknodRequest, intr fs.Intr) (fs.Node, fuse.Error)
	Fsync(req *fuse.FsyncRequest, intr fs.Intr) fuse.Error
	Forget()
	// Setattr derivatives
	Chown(req *fuse.SetattrRequest) fuse.Error
	Chmod(req *fuse.SetattrRequest) fuse.Error
	Chtimes(req *fuse.SetattrRequest) fuse.Error
	Truncate(req *fuse.SetattrRequest) fuse.Error
}

type FuseHandle interface {
	ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error)
	Read(req *fuse.ReadRequest, resp *fuse.ReadResponse, intr fs.Intr) fuse.Error
	Write(req *fuse.WriteRequest, resp *fuse.WriteResponse, intr fs.Intr) fuse.Error
	Flush(req *fuse.FlushRequest, intr fs.Intr) fuse.Error
	Release(req *fuse.ReleaseRequest, intr fs.Intr) fuse.Error
}

func fileInfoToDirent(info os.FileInfo) fuse.Dirent {
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
