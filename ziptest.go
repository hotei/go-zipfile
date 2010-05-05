// ziptest.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// source can be found at http://www.github.com/hotei/go-zipfile
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
	"strings"
	"./zip"
	//    "unsafe"
)

func fatal_err(erx os.Error) {
	fmt.Printf("%s \n", erx)
	os.Exit(1)
}

// Run thru all files in archive, printing header info
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
	fmt.Printf("Read %d file headers from archive\n", n)
	fmt.Printf("test_0 c'est fini\n")
}

// Run thru all files in archive, printing header info
// this time using built-in func Headers
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
	fmt.Printf("len filelist = %d\n", len(filelist))
	zip.Dashboard.Debug = false
	for ndx, hdr := range filelist {
		//		fmt.Printf("hdr = %v\n",hdr)
		fmt.Printf("\nlisting from hdr %d\n", ndx)
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
	hdr, err := rz.Next() // actually gets first entry this time
	if err != nil {
		fatal_err(err)
	}
	rdr, err := hdr.Open()
	_, err = io.Copy(os.Stdout, rdr) // open first file only
	if err != nil {
		fatal_err(err)
	}
}

// open and print only the html files.
func test_3() {
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
	fmt.Printf("len filelist = %d\n", len(filelist))

	zip.Dashboard.Debug = false
	n := 1
	for ndx, hdr := range filelist {
		if strings.HasSuffix(hdr.Name, ".html") {
			fmt.Printf("\n%d listing from hdr %d\n", n, ndx)
			hdr.Dump()
			rdr, err := hdr.Open()
			_, err = io.Copy(os.Stdout, rdr)
			if err != nil {
				fatal_err(err)
			}
			n++
		}
	}
	fmt.Printf("test_3 c'est fini\n")
}

// M A I N ----------------------------------------------------------------------- M A I N
func main() {
	fmt.Printf("<starting test of newest ziptest>\n")
	zip.Dashboard.Debug = true
	zip.Dashboard.Paranoid = true
	test_0()
	test_1()
	test_2()
	test_3()

	fmt.Printf("<end of test of newest ziptest>\n")
	os.Exit(0)
}
