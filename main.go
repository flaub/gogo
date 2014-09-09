package main

import (
	"code.google.com/p/go.exp/fsnotify"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Rule struct {
	Path    string
	Cmd     string
	Args    []string
	Events  string
	Ignores []string
	Pause   int
	timer   *time.Timer
}

type Spec map[string]Rule

func (this Spec) watch() {
	for path, rule := range this {
		rule.Path = path
		rule.watch()
	}
}

func (this *Rule) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	pause := time.Duration(this.Pause) * time.Millisecond
	this.timer = time.NewTimer(pause)

	go func() {
		for {
			select {
			case evt := <-watcher.Event:
				if !this.ignore(evt) {
					// log.Println("event:", evt)
					this.timer.Reset(pause)
				}
			case err := <-watcher.Error:
				log.Println("error:", err)
			case <-this.timer.C:
				this.run()
			}
		}
	}()

	log.Println("Watching path:", this.Path)
	err = watcher.Watch(this.Path)
	if err != nil {
		log.Fatal(err)
	}
}

func (this *Rule) ignore(evt *fsnotify.FileEvent) bool {
	for _, ignore := range this.Ignores {
		matched, err := filepath.Match(ignore, evt.Name)
		if err != nil {
			log.Println("error:", err)
			return true
		}
		if matched {
			// log.Println("Ignoring:", evt.Name)
			return true
		}
	}

	return false
}

func (this *Rule) run() {
	log.Println("run:", this.Cmd)
	cmd := exec.Command(this.Cmd, this.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func main() {
	file, err := os.Open("Gogo")
	if err != nil {
		log.Fatal(err)
	}

	dec := json.NewDecoder(file)
	var spec Spec
	err = dec.Decode(&spec)
	if err != nil {
		log.Fatal(err)
	}

	if len(spec) == 0 {
		log.Fatal("Nothing to do")
	}

	done := make(chan bool)

	spec.watch()

	<-done
}
