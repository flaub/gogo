package main

import (
	"bytes"
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

type loopbackTest struct {
	tmpDir      string
	orig        string
	mnt         string
	mountFile   string
	mountSubdir string
	origFile    string
	origSubdir  string
	t           *testing.T
	conn        *fuse.Conn
	fs          *LoopbackFS
}

func newLoopbackTest(t *testing.T) *loopbackTest {
	this := &loopbackTest{
		t: t,
	}

	syscall.Umask(0)

	const name string = "hello.txt"
	const subdir string = "subdir"

	var err error
	this.tmpDir, err = ioutil.TempDir("", "gogo")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	this.orig = path.Join(this.tmpDir, "orig")
	this.mnt = path.Join(this.tmpDir, "mnt")

	os.Mkdir(this.orig, 0700)
	os.Mkdir(this.mnt, 0700)

	this.mountFile = path.Join(this.mnt, name)
	this.mountSubdir = path.Join(this.mnt, subdir)
	this.origFile = path.Join(this.orig, name)
	this.origSubdir = path.Join(this.orig, subdir)

	this.fs = NewLoopbackFS(this.orig)

	this.conn, err = fuse.Mount(this.mnt)
	if err != nil {
		t.Fatalf("fuse.Mount() failed: %v", err)
	}

	go fs.Serve(this.conn, this.fs)
	this.fs.WaitReady()

	return this
}

func (this *loopbackTest) Cleanup() {
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

func TestLoRead(t *testing.T) {
	tc := newLoopbackTest(t)
	defer tc.Cleanup()

	expected := []byte("Hello")
	err := ioutil.WriteFile(tc.origFile, expected, 0700)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	f, err := os.Open(tc.mountFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	var buf [1024]byte
	n, err := f.Read(buf[:])
	actual := buf[:n]

	if !bytes.Equal(expected, actual) {
		t.Errorf("Mismatch. Expected: %q, Actual: %q", expected, actual)
	}
}

func TestLoRemove(t *testing.T) {
	tc := newLoopbackTest(t)
	defer tc.Cleanup()

	expected := []byte("Hello")
	err := ioutil.WriteFile(tc.origFile, expected, 0700)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = os.Remove(tc.mountFile)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, err = os.Lstat(tc.origFile)
	if err == nil {
		t.Errorf("Lstat() after delete should have generated error.")
	}
}

func TestLoWrite(t *testing.T) {
	tc := newLoopbackTest(t)
	defer tc.Cleanup()

	expected := []byte("Hello")

	// Create (for write), write.
	func() {
		f, err := os.OpenFile(tc.mountFile, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			t.Fatalf("OpenFile failed: %v", err)
		}
		defer f.Close()

		n, err := f.Write(expected)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(expected) {
			t.Errorf("Write mismatch: %v of %v", n, len(expected))
		}
	}()

	fi, err := os.Lstat(tc.origFile)
	if err != nil || fi.Mode().Perm() != 0644 {
		t.Errorf("create mode error %o", fi.Mode()&0777)
	}

	var buf [1024]byte
	var actual []byte

	func() {
		f, err := os.Open(tc.origFile)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer f.Close()

		n, err := f.Read(buf[:])
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		actual = buf[:n]
	}()

	if !bytes.Equal(expected, actual) {
		t.Errorf("Mismatch. Expected: %q, Actual: %q", expected, actual)
	}
}

func TestLoWriteAppend(t *testing.T) {
	tc := newLoopbackTest(t)
	defer tc.Cleanup()

	expected1 := []byte("Hello")
	expected2 := []byte("Goodbye")

	// Create (for write), write.
	func() {
		f, err := os.OpenFile(tc.mountFile, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			t.Fatalf("OpenFile failed: %v", err)
		}
		defer f.Close()

		n, err := f.Write(expected1)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(expected1) {
			t.Errorf("Write mismatch: %v of %v", n, len(expected1))
		}
	}()

	fi, err := os.Lstat(tc.origFile)
	if fi.Mode().Perm() != 0644 {
		t.Errorf("create mode error %o", fi.Mode()&0777)
	}

	func() {
		f, err := os.OpenFile(tc.mountFile, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			t.Fatalf("OpenFile failed: %v", err)
		}
		defer f.Close()

		n, err := f.Write(expected2)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(expected2) {
			t.Errorf("Write mismatch: %v of %v", n, len(expected2))
		}
	}()

	expected := append(expected1, expected2...)

	f3, err := os.Open(tc.origFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f3.Close()

	var buf [1024]byte
	n, err := f3.Read(buf[:])
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	actual := buf[:n]

	if !bytes.Equal(expected, actual) {
		t.Errorf("Mismatch. Expected: %q, Actual: %q", expected, actual)
	}
}

func TestLoGcc(t *testing.T) {
	tc := newLoopbackTest(t)
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
