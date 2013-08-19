package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"github.com/aarzilli/golua/lua"
	"github.com/stevedonovan/luar"

	"github.com/kless/term"
)

type Context struct {
	luaState *lua.State
	mnt      *fuse.Conn
	dir      string
}

func NewContext() *Context {
	this := &Context{
		luaState: luar.Init(),
		dir:      ".gogo",
	}

	luar.Register(this.luaState, "gogo", luar.Map{
		"glob": filepath.Glob,
		"rule": this.Rule,
	})

	err := os.MkdirAll(this.dir, os.ModePerm)
	if err != nil {
		log.Fatal("Could not create .gogo directory")
	}

	syscall.Unmount(this.dir, 0) // just in case
	this.mnt, err = fuse.Mount(this.dir)
	if err != nil {
		log.Fatal(err)
	}

	return this
}

func (this *Context) Close() {
	this.luaState.Close()
	syscall.Unmount(this.dir, 0)
}

func (this *Context) Rule(command string, inputs, outputs []string) {
	fmt.Printf("Rule: %v -> %v -> %v\n", inputs, command, outputs)
}

func (this *Context) LoadFile(filename string) {
	err := this.luaState.DoFile(filename)
	if err != nil {
		panic(err)
	}
}

func (this *Context) Execute(done chan<- error) {
	done <- fs.Serve(this.mnt, FS{})
}

func awaitQuitKey(done chan<- bool) {
	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}
		if buf[0] == 'q' {
			done <- true
			return
		}
	}
}

func main() {
	ctx := NewContext()
	defer ctx.Close()

	ctx.LoadFile("Gogo.lua")

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	done := make(chan error, 1)
	go ctx.Execute(done)

	if term.IsTerminal(term.InputFD) {
		tty, err := term.New()
		if err != nil {
			panic(err)
		}
		defer tty.Restore()
		tty.CharMode()
		tty.EchoMode(false)
	}

	quitKey := make(chan bool, 1)
	go awaitQuitKey(quitKey)

	select {
	case err := <-done:
		log.Printf("Execute returned %v", err)
	case sig := <-sigc:
		log.Printf("Signal %s received, shutting down.", sig)
	case <-quitKey:
		log.Printf("Quit key pressed, shutting down.")
	}
}

// FS implements the hello world file system.
type FS struct{}

func (FS) Root() (fs.Node, fuse.Error) {
	return Dir{}, nil
}

// Dir implements both Node and Handle for the root directory.
type Dir struct{}

func (Dir) Attr() fuse.Attr {
	return fuse.Attr{Inode: 1, Mode: os.ModeDir | 0555}
}

func (Dir) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	if name == "hello" {
		return File{}, nil
	}
	return nil, fuse.ENOENT
}

var dirDirs = []fuse.Dirent{
	{Inode: 2, Name: "hello", Type: fuse.DT_File},
}

func (Dir) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	return dirDirs, nil
}

type File struct{}

func (File) Attr() fuse.Attr {
	return fuse.Attr{Mode: 0444}
}

func (File) ReadAll(intr fs.Intr) ([]byte, fuse.Error) {
	return []byte("hello, world\n"), nil
}
