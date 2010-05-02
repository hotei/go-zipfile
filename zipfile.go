// zipfile.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// <David Rook> ravenstone13@cox.net
// This is a work-in-progress

package main

import (
	"flag"
	"fmt"
    "os"
    "./zip"
	//    "unsafe"
)

const DEBUG = false

// list files in named archive
func test_1() {
	flag.Parse()
	fmt.Printf("Flag got %d args on cmd line after command name\n", flag.NArg())
	if flag.NArg() == 0 { // do nothing
	} else { // whitespace sensitive
		for i := 0; i < flag.NArg(); i++ {
			fmt.Printf("%d %s\n", i, flag.Arg(i))

			myzipper := new(zip.ZipFile)
			ok := myzipper.Init(flag.Arg(i)) // read headers into array, get size
			if !ok { /* TODO */
			}

			var files []string
			files = myzipper.ListFiles()
			if len(files) == 0 {
				if DEBUG {
					fmt.Printf("No files in zipfile %s ...???\n", myzipper.FileName)
				}
				continue
			}
			if files == nil || !ok { /* TODO problem */
			} else {
				for ndx, val := range files {
					fmt.Printf("file[%d] = %s\n", ndx, val)
				}
			}
		}
	}
}

func test_2() {
}


func test_3(fname string) {
	myzipper := new(zip.ZipFile)
	ok := myzipper.Init(fname) // read - this one is stored
	if !ok {
		fmt.Printf("Can't open requested file: %s\n", fname)
		return
	}
	myzipper.ListZip(0) // list the first file to stdout
}

func test_4(fname string) {
	myzipper := new(zip.ZipFile)
	ok := myzipper.Init(fname)
	if !ok {
		fmt.Printf("Can't open requested file: %s\n", fname)
		return
	}
	if myzipper.NumFiles >= 2 {
		//fmt.Printf("file[0] zip'd size = %d\n", myzipper.LocalHeaders[0].compressSize)
		//fmt.Printf("file[0] unzip'd size = %d\n", myzipper.LocalHeaders[0].unComprSize)
		//fmt.Printf("file[1] zip'd size = %d\n", myzipper.LocalHeaders[1].compressSize)
		//fmt.Printf("file[1] unzip'd size = %d\n", myzipper.LocalHeaders[1].unComprSize)
	}
}

// M A I N ----------------------------------------------------------------------- M A I N
func main() {
	fmt.Printf("<starting test of zipfile>\n")
	test_1() // list titles of all files
	// test_2()
	test_3("stuf.zip") // dump stuf.zip to stdout
	test_3("mini.zip") // dump mini.zip to stdout
	test_4("phpBB-2.0.20.zip")
	fmt.Printf("<end of test of newest zipfile>\n")
	os.Exit(0)
}

