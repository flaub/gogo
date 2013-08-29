package main

import (
	"os"
	"syscall"
	"time"

	"bazil.org/fuse"
)

func fileMode(st *syscall.Stat_t) os.FileMode {
	mode := os.FileMode(st.Mode & 0777)
	switch st.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK, syscall.S_IFWHT:
		mode |= os.ModeDevice
	case syscall.S_IFCHR:
		mode |= os.ModeDevice | os.ModeCharDevice
	case syscall.S_IFDIR:
		mode |= os.ModeDir
	case syscall.S_IFIFO:
		mode |= os.ModeNamedPipe
	case syscall.S_IFLNK:
		mode |= os.ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		mode |= os.ModeSocket
	}
	if st.Mode&syscall.S_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	if st.Mode&syscall.S_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if st.Mode&syscall.S_ISVTX != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

func StatToAttr(st *syscall.Stat_t) fuse.Attr {
	a := fuse.Attr{}
	a.Inode = uint64(st.Ino)
	a.Size = uint64(st.Size)
	a.Blocks = uint64(st.Blocks)
	a.Atime = time.Unix(st.Atimespec.Unix())
	a.Mtime = time.Unix(st.Mtimespec.Unix())
	a.Ctime = time.Unix(st.Ctimespec.Unix())
	a.Mode = fileMode(st)
	a.Nlink = uint32(st.Nlink)
	a.Uid = uint32(st.Uid)
	a.Gid = uint32(st.Gid)
	a.Rdev = uint32(st.Rdev)
	return a
}
