// reader_test.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// source can be found at http://www.github.com/hotei/go-zipfile
//
// <David Rook> ravenstone13@cox.net
// This is a work-in-progress
//     This version does only reading, no zip writing
//     Verbose mode will eventually go away

package zip

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Purpose: exercise NewReader(),Next(), Dump() on a valid zip file
// Run thru all files in archive, printing header info using Verbose mode
//
func Test001(t *testing.T) {
	fmt.Printf("Test001 start\n")
	const testfile = "testdata/stuf.zip"
	f, err := os.Open(testfile)
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
	fmt.Printf("Test002 start\n")
	const testfile = "testdata/phpBB.zip"

	f, err := os.Open(testfile)
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
	for _, hdr := range filelist {
		//		fmt.Printf("hdr = %v\n",hdr)
		//  fmt.Printf("\nlisting from hdr %d\n", ndx)
		hdr.Dump()
	}
	fmt.Printf("Test002 fini\n")

}

// Purpose: Exercise Open() on one file
func TestX003(t *testing.T) {
	fmt.Printf("Test003 start\n")
	const testfile = "testdata/stuf.zip"

	f, err := os.Open(testfile)
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
// open and print only the php files from the archive
func TestSeqRead(t *testing.T) {
	fmt.Printf("TestSeqRead start\n")

	const testfile = "testdata/phpBB.zip"

	f, err := os.Open(testfile)
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

	o, err := os.OpenFile("/dev/null", os.O_WRONLY, 0666)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer o.Close()
	hnum := 1
	Dashboard.Paranoid = true
	for ndx, hdr := range filelist {
		if strings.HasSuffix(hdr.Name, ".php") {
			if Dashboard.Verbose {
				fmt.Printf("%4d: ", ndx)
				hdr.Dump()
			}
			if hdr.Size == 0 {
				continue
			} //  is this a case that io.Copy doesn't handle gracefully?
			ndx = ndx
			rdr, err := hdr.Open()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			//			_, err = io.Copy(os.Stdout, rdr)
			var n int64
			n, err = io.Copy(o, rdr)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if n != hdr.Size {
				fmt.Printf("size expected (%d) doesn't match size read (%d)", hdr.Size, n)
				t.Fail()
				// t.Fatalf("Unexpected error: %v", err)
			}
			hnum++
		}
	}
	fmt.Printf("TestSeqRead finishing normally\n")
}

// normally a real process would do something interesting with the blob contents
// here we just make sure it's readable and contains the right number of bytes
// by doing an io.Copy() out to the null device
// Open() has already validated the CRC32 so no need to do so again
// we use channel c to indicate success or failure back to the main test func
// channel l signals completion in either case
// failure results in c getting a message sent where n < 0, success n > 0
func processBlob(hdrNum int, blob io.Reader, size int64, c chan int) {
	o, err := os.OpenFile("/dev/null", os.O_WRONLY, 0666)
	if err != nil {
		c <- -hdrNum
		return
	}
	defer o.Close()
	var n int64
	n, err = io.Copy(o, blob)
	if Dashboard.Verbose {
		fmt.Printf("processed header number %d\n", hdrNum)
	}
	if err != nil {
		c <- -hdrNum
		return
	}
	if n != size {
		fmt.Printf("Header %d, size expected (%d) doesn't match size read (%d)", hdrNum, size, n)
		c <- -hdrNum
		return
	}
	c <- hdrNum
	return
}

// Test multiple instances of processBlob()
// Doesn't really test concurrent reads on archive hmmmm...
func TestConcurrent(t *testing.T) {
	fmt.Printf("TestConcurrent starting\n")
	var MAX_GORU = 14

	tmp := os.Getenv("GOMAXPROCS")
	MAXPROCS, err := strconv.Atoi(tmp)
	if err != nil {
		MAXPROCS = 3
	} else {
		fmt.Printf("GOMAXPROCS = %d\n", MAXPROCS)
	}
	MAX_GORU = MAXPROCS * 3

	const testfile = "testdata/phpBB.zip"
	fmt.Printf("TestConcurrent uses a max of %d GoRus\n", MAX_GORU)

	c := make(chan int, 1)        // used to return value to caller when done
	l := make(chan int, MAX_GORU) // limit loop to creating <= MAX_GORU goroutines at once

	f, err := os.Open(testfile)
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
	Dashboard.Paranoid = true // make sure we always do the CRC32IEEE() check
	Dashboard.Verbose = false
	hnum := 0
	// spread out the work among up to MAX_GORU CPUs
	workout := 0
	for ndx, hdr := range filelist {
		if strings.HasSuffix(hdr.Name, ".php") {
			//			if Dashboard.Verbose {
			fmt.Printf("%4d: ", ndx)
			hdr.Dump()
			//			}
			if hdr.Size == 0 {
				continue
			} //  is this a case that io.Copy doesn't handle gracefully?
			ndx = ndx
			rdr, err := hdr.Open()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			l <- 1
			// this should block if MAXPROCS goroutines are active already
			// motivation for this is to reduce possibly large memory footprint if
			// multiple large blobs decompress at the same time, a few is ok, thousands... not so good
			go processBlob(hnum, rdr, hdr.Size, c)
			<-l
			workout++
		}
		hnum++
	}
	for workout > 0 {
		rc := <-c
		fmt.Printf("rc = %d \n", rc)
		if rc < 0 {
			t.Fatalf("Unexpected error: %v", err)
		}
		workout--
	}
	fmt.Printf("Non-sequential rc results is sign of success\n")
	fmt.Printf("TestConcurrent finishing normally\n")
}

/* // Test template
func TestXXX (t *testing.T) {
    if false {
        t.Fatalf("Unexpected error: %v", err)
    }
}
*/
