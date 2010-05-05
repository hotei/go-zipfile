// ziptest.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// source can be found at http://www.github.com/hotei/go-zipfile
//
// <David Rook> ravenstone13@cox.net
// This is a work-in-progress
//     This version does only reading, no zip writing
package zip

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// Purpose: exercise NewReader(),Next(), Dump() on a valid zip file
// Run thru all files in archive, printing header info using debug mode
//
func Test001(t *testing.T) {
	const testfile = "testdata/stuf.zip"
	Dashboard.Debug = true
	f, err := os.Open(testfile, os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer f.Close()
	fmt.Printf("opened zip file %s\n", testfile)
	rz, err := NewReader(f)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	n := 1
	for {
		hdr, err := rz.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if hdr == nil { // no more data
			break
		}
		n++
		hdr.Dump()
	}
	fmt.Printf("Test001 fini\n")
}

// Purpose: exercise Headers()
// Run thru all files in archive, printing header info
//
func Test002(t *testing.T) {

	const testfile = "testdata/phpBB.zip"

	f, err := os.Open(testfile, os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer f.Close()
	fmt.Printf("opened zip file %s\n", testfile)
	rz, err := NewReader(f)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	filelist := rz.Headers()
	fmt.Printf("len filelist = %d\n", len(filelist))
	Dashboard.Debug = false
	for ndx, hdr := range filelist {
		//		fmt.Printf("hdr = %v\n",hdr)
		fmt.Printf("\nlisting from hdr %d\n", ndx)
		hdr.Dump()
	}
	fmt.Printf("Test002 fini\n")

}

// Purpose: Exercise Open() on one file
func Test003(t *testing.T) {

	const testfile = "testdata/stuf.zip"

	f, err := os.Open(testfile, os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer f.Close()
	fmt.Printf("opened zip file %s\n", testfile)
	rz, err := NewReader(f)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	hdr, err := rz.Next() // actually gets first entry this time
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	rdr, err := hdr.Open()
	_, err = io.Copy(os.Stdout, rdr) // open first file only
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	fmt.Printf("Test003 fini\n")
}

// Purpose: Exercise Open() on multiple, non-sequential files from Header list
// open and print only the html files from the archive
func Test004(t *testing.T) {

	const testfile = "testdata/phpBB.zip"

	f, err := os.Open(testfile, os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer f.Close()
	fmt.Printf("opened zip file %s\n", testfile)
	rz, err := NewReader(f)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	filelist := rz.Headers()
	fmt.Printf("len filelist = %d\n", len(filelist))

	Dashboard.Debug = false
	n := 1
	for ndx, hdr := range filelist {
		if strings.HasSuffix(hdr.Name, ".html") {
			fmt.Printf("\n%d listing from hdr %d\n", n, ndx)
			hdr.Dump()
			rdr, err := hdr.Open()
			_, err = io.Copy(os.Stdout, rdr)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			n++
		}
	}
	fmt.Printf("Test004 fini\n")
}
