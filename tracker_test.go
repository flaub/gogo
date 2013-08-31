package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"
	"testing"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type trackerTest struct {
	tmpDir string
	orig   string
	mnt    string
	t      *testing.T
	conn   *fuse.Conn
	fs     *TrackerFS
}

func newTrackerTest(t *testing.T) *trackerTest {
	this := &trackerTest{
		t: t,
	}

	syscall.Umask(0)

	var err error
	this.tmpDir, err = ioutil.TempDir("", "gogo")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	this.orig = path.Join(this.tmpDir, "orig")
	this.mnt = path.Join(this.tmpDir, "mnt")

	os.Mkdir(this.orig, 0700)
	os.Mkdir(this.mnt, 0700)

	this.fs = NewTrackerFS(this.orig)

	this.conn, err = fuse.Mount(this.mnt)
	if err != nil {
		t.Fatalf("fuse.Mount() failed: %v", err)
	}

	go fs.Serve(this.conn, this.fs)
	this.fs.WaitReady()

	return this
}

func (this *trackerTest) Cleanup() {
	log.Printf("Cleanup> unmounting...")
	for {
		err := syscall.Unmount(this.mnt, 0)
		if err == nil {
			break
		}
		this.t.Logf("Unmount failed: %v", err)
		time.Sleep(10 * time.Millisecond)
	}

	os.RemoveAll(this.tmpDir)

	log.Printf("Cleanup> waiting for Destroy...")
	this.fs.WaitDestroy()
}

func TestTrGcc(t *testing.T) {
	tc := newTrackerTest(t)
	defer tc.Cleanup()

	const h_code = `
// nothing to see here
`

	const c_code = `
#include <stdio.h>
#include "test.h"
int main(int argc, char** argv) {
	printf("Hello\n");
	return 0;
}
`

	log.Print("Creating test files")

	err := ioutil.WriteFile(path.Join(tc.orig, "test.h"), []byte(h_code), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = ioutil.WriteFile(path.Join(tc.orig, "main.c"), []byte(c_code), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("gcc", "-c", "main.c", "-o", "main.o")
	cmd.Dir = tc.mnt
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("Running: %v", cmd.Args)
	err = cmd.Run()
	if err != nil {
		t.Fatalf("gcc failed: %v", err)
	}

	cmd = exec.Command("gcc", "main.o", "-o", "hi")
	cmd.Dir = tc.mnt
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("Running: %v", cmd.Args)
	err = cmd.Run()
	if err != nil {
		t.Fatalf("gcc failed: %v", err)
	}
}
