package main

import (
	"os"
	"syscall"

	"bazil.org/fuse"
)

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
