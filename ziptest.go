// ziptest.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// <David Rook> ravenstone13@cox.net
// This is a work-in-progress

package main

import (
	//	"flag"
	"fmt"
	"io"
	"os"
	"./zip"
	//    "unsafe"
)

const DEBUG = false

func fatal_err(erx os.Error) {
	fmt.Printf("%s \n", erx)
	os.Exit(1)
}

func test_0() {
	const testfile = "stuf.zip"

	input, err := os.Open(testfile, os.O_RDONLY, 0666)
	if err != nil {
		fatal_err(err)
	}
	fmt.Printf("opened zip file %s\n", testfile)
	rz, err := zip.NewReader(input)
	if err != nil {
		fatal_err(err)
	}
	// Run thru all files in archive, printing header info
	n := 1
	for {
		hdr, err := rz.Next()
		if err != nil { fatal_err(err) }
		if hdr == nil {
			break
		}
		fmt.Printf("Filename [%d] is %s\n", n, hdr.Name)
		n++
		fmt.Printf("Size %d, Size Compressed %d, Type flag %d, LastMod %d, ComprMeth %d, Offset %d\n",
			hdr.Size, hdr.SizeCompr, hdr.Typeflag, hdr.Mtime, hdr.Compress, hdr.Offset)
		// io.Copy(data, zr)	// copy data out of uncompressed buffer via zr
	}
	fmt.Printf("test_0 c'est fini\n")
}

func test_1() {
	const testfile = "phpBB.zip"

	input, err := os.Open(testfile, os.O_RDONLY, 0666)
	if err != nil {
		fatal_err(err)
	}
	fmt.Printf("opened zip file %s\n", testfile)
	rz, err := zip.NewReader(input)
	if err != nil { fatal_err(err) }

	filelist := rz.Headers()
	filelist = filelist
	fmt.Printf("test_1 c'est fini\n")
	for _, hdr := range filelist {
		fmt.Printf("Size %d, Size Compressed %d, Type flag %d, LastMod %d, ComprMeth %d, Offset %d\n",
				   hdr.Size, hdr.SizeCompr, hdr.Typeflag, hdr.Mtime, hdr.Compress, hdr.Offset)
	}
}

func test_2() {
	const testfile = "stuf.zip"
	
	input, err := os.Open(testfile, os.O_RDONLY, 0666)
	if err != nil { fatal_err(err) }
	fmt.Printf("opened zip file %s\n", testfile)
	rz, err := zip.NewReader(input)
	if err != nil { fatal_err(err) }
	hdr, err := rz.Next()
	rdr, err := hdr.Open()
	_, err = io.Copy(os.Stdout, rdr)
	if err != nil { fatal_err(err) }
}

// M A I N ----------------------------------------------------------------------- M A I N
func main() {
	fmt.Printf("<starting test of newest ziptest>\n")
	// test_0()
	// test_1()
	test_2()
	
	fmt.Printf("<end of test of newest ziptest>\n")
	os.Exit(0)
}
