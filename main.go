package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/aarzilli/golua/lua"
	"github.com/stevedonovan/luar"

	"github.com/kless/term"
)

type Context struct {
	luaState *lua.State
	fs       *FuseSubsystem
}

func NewContext() *Context {
	this := &Context{
		luaState: luar.Init(),
		fs:       NewFuseSubsystem(),
	}

	luar.Register(this.luaState, "gogo", luar.Map{
		"glob": filepath.Glob,
		"rule": this.Rule,
	})

	return this
}

func (this *Context) Destroy() {
	this.luaState.Close()
	this.fs.Destroy()
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
	done <- this.fs.Execute()
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
	defer ctx.Destroy()

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
