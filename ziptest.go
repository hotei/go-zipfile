// ziptest.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// <David Rook> ravenstone13@cox.net
// This is a work-in-progress
//     This version does only reading, no zip writing
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
		if err != nil {
			fatal_err(err)
		}
		if hdr == nil { // no more data
			break
		}
		n++
		hdr.Dump()
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
	if err != nil {
		fatal_err(err)
	}
	filelist := rz.Headers()
	filelist = filelist
	for _, hdr := range filelist {
		hdr.Dump()
	}
	fmt.Printf("test_1 c'est fini\n")
}

func test_2() {
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
	hdr, err := rz.Next()
	rdr, err := hdr.Open()
	_, err = io.Copy(os.Stdout, rdr) // open first file only
	if err != nil {
		fatal_err(err)
	}
}

func test_3() {

	// TODO can't do mycrc32 unless we get buffer uncompressed -- now what?  below is one way...

	/*
		expdData := make([]byte, h.Size)		// make the expanded buffer
		n, err = b.Read(expdData)		// copy expanded stuff to new buffer
		if n != h.Size  {
			fmt.Printf("copied %d, expected %d\n", n, h.Size) )
			fatal_err(os.EIO)
		}
		mycrc32 := crc32.ChecksumIEEE(expdData)
		fmt.Printf("Computed Checksum = %0x, stored checksum = %0x\n", mycrc32, h.StoredCrc32)
	*/
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
