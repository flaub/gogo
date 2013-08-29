package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path"
	"syscall"
	"testing"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type testCase struct {
	tmpDir      string
	orig        string
	mnt         string
	mountFile   string
	mountSubdir string
	origFile    string
	origSubdir  string
	t           *testing.T
	conn        *fuse.Conn
	lofs        *LoopbackFS
}

func NewTestCase(t *testing.T) *testCase {
	this := &testCase{
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

	this.lofs = NewLoopbackFS(this.orig)

	this.conn, err = fuse.Mount(this.mnt)
	if err != nil {
		t.Fatalf("fuse.Mount() failed: %v", err)
	}

	go fs.Serve(this.conn, this.lofs)
	this.lofs.WaitReady()

	return this
}

func (this *testCase) Cleanup() {
	log.Printf("Cleanup> unmounting...")
	err := syscall.Unmount(this.mnt, 0)
	if err != nil {
		this.t.Fatalf("Unmount failed: %v", err)
	}

	os.RemoveAll(this.tmpDir)

	log.Printf("Cleanup> waiting for Destroy...")
	this.lofs.WaitDestroy()
}

func TestRead(t *testing.T) {
	tc := NewTestCase(t)
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

func TestRemove(t *testing.T) {
	tc := NewTestCase(t)
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

func TestWrite(t *testing.T) {
	tc := NewTestCase(t)
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

func TestWriteAppend(t *testing.T) {
	tc := NewTestCase(t)
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
	if fi.Mode().Perm() != 0644 {
		t.Errorf("create mode error %o", fi.Mode()&0777)
	}

	func() {
		f, err := os.OpenFile(tc.mountFile, os.O_WRONLY|os.O_APPEND, 0644)
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

	expected = append(expected, expected...)

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
