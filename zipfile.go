// zipfile.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


package main

import (
	"flag"
	"fmt"
	"os"
	//    "unsafe"
)

const DEBUG = true
const (
	ZIP_LocalHdrSig = "PK\003\004"
	ZIP_CentDirSig  = "PK\001\002"
	ZIP_STORED      = 0
	ZIP_DEFLATED    = 8
)

type ZipLocalHeader struct {
	zlhsig       string // (0x04034b50)  or "PK\003\004"
	ver2Extract  uint16 // need this version or higher to extract
	generalBits  uint16
	compressMeth uint16
	lastModTime  uint16
	lastModDate  uint16
	crc32        uint32
	compressSize uint32
	unComprSize  uint32
	fileNameLen  uint16
	extraFldLen  uint16
	fileName     []byte
	extraFld     []byte
}

// convert from litte endian two byte slice to int16
func SixteenBit(n []byte) uint16 {
	if len(n) != 2 { /* TODO problem */
	}
	var rc uint16
	rc = uint16(n[1])
	rc <<= 8
	rc |= uint16(n[0])
	return rc
}

// convert from litte endian four byte slice to int32
func ThirtyTwoBit(n []byte) uint32 {
	if len(n) != 4 { /* TODO problem */
	}
	var rc uint32
	rc = uint32(n[3])
	rc <<= 8
	rc |= uint32(n[2])
	rc <<= 8
	rc |= uint32(n[1])
	rc <<= 8
	rc |= uint32(n[0])
	return rc
}

// test function to read thru file headers
func readHeaders(filename string) bool {
	var n int
	var curPos int64
	var newPos int64

	if DEBUG {
		fmt.Printf("Testing readHeader( %s )\n", filename)
	}
	fpin, err := os.Open(filename, os.O_RDONLY, 0666)
	// err could be file not found, file found but not accessible etc
	// in any case test fails
	if err != nil {
		return false
	}
	defer fpin.Close()
	fileSize, err := fpin.Seek(0, 2) // get file size
	if DEBUG {
		fmt.Printf("file size = %d\n", fileSize)
	}
	if err != nil {
		return false
	}
	_, err = fpin.Seek(0, 0) // back to beginning

	hdr := new(ZipLocalHeader)
	//    fmt.Printf("size of hdr = %d\n", unsafe.Sizeof(*hdr))
	var hdrData [30]byte // size of header data fixed fields only

	for {
		n, err = fpin.Read(&hdrData)
		if err != nil || n != 30 { /* TODO problem */
			fmt.Printf("Header read failed\n")
			os.Exit(1)
		}
		//	    fmt.Printf("data = %v\n", hdrData)

		if string(hdrData[0:4]) == ZIP_LocalHdrSig {
			//            if DEBUG { fmt.Printf("good magic number\n") }
			hdr.zlhsig = ZIP_LocalHdrSig
		}
		hdr.ver2Extract = SixteenBit(hdrData[4:6])
		//        fmt.Printf("Extract version req = %d\n", hdr.ver2Extract )

		hdr.generalBits = SixteenBit(hdrData[6:8])
		//        fmt.Printf("General bits = %d\n", hdr.generalBits )

		hdr.compressMeth = SixteenBit(hdrData[8:10])
		fmt.Printf("Compress Method = %d\n", hdr.compressMeth)
		if hdr.compressMeth != ZIP_STORED && hdr.compressMeth != ZIP_DEFLATED {
			fmt.Printf("Trouble -- unimplemented compression meth %d\n", hdr.compressMeth)
			os.Exit(1)
		}
		hdr.lastModTime = SixteenBit(hdrData[10:12])
		//        fmt.Printf("LastModTime = %d\n", hdr.lastModTime )

		hdr.lastModDate = SixteenBit(hdrData[12:14])
		//        fmt.Printf("LastModDate = %d\n", hdr.lastModDate )

		hdr.crc32 = ThirtyTwoBit(hdrData[14:18])
		//        fmt.Printf("crc32 = %4x\n", hdr.crc32)

		hdr.compressSize = ThirtyTwoBit(hdrData[18:22])
		//        fmt.Printf("compressSize = %d\n", hdr.compressSize)

		hdr.unComprSize = ThirtyTwoBit(hdrData[22:26])
		//        fmt.Printf("unComprSize = %d\n", hdr.unComprSize)

		hdr.fileNameLen = SixteenBit(hdrData[26:28])
		fmt.Printf("fileNameLen = %d\n", hdr.fileNameLen)

		if hdr.fileNameLen == 0 {
			fmt.Printf("Reached end of zip file\n")
			break
		}

		hdr.extraFldLen = SixteenBit(hdrData[28:30])
		//        fmt.Printf("extraFldLen = %d\n", hdr.extraFldLen)

		fname := make([]byte, hdr.fileNameLen, hdr.fileNameLen)
		n, err = fpin.Read(fname)
		if err != nil || n != int(hdr.fileNameLen) { /* TODO problem */
		}
		fmt.Printf("filename = %s\n", fname)

		extra := make([]byte, hdr.extraFldLen, hdr.extraFldLen)
		n, err = fpin.Read(extra)
		if err != nil || n != int(hdr.extraFldLen) { /* TODO problem */
		}
		//        fmt.Printf("extra = %v\n", extra)
		curPos, err = fpin.Seek(0, 1)
		//        fmt.Printf("current position is %d\n", curPos )
		newPos, err = fpin.Seek(int64(hdr.compressSize), 1) // advance to next header
		if newPos != int64(hdr.compressSize)+curPos {
			fmt.Printf("advance to next header failed %d != %d\n", newPos, int64(hdr.compressSize)+curPos)
			os.Exit(1)
		}
		if newPos >= fileSize {
			fmt.Printf("Reached end of zip file\n")
			break
		}
	}
	return true // ok
}

// return nil, false if Zip is bad or empty
// TODO ? handle error better how ?
func ListOfFiles(filename string) ([]string, bool) {
	var curPos, newPos int64
	var chRead int

	if DEBUG {
		fmt.Printf("Testing ListOfFiles( %s )\n", filename)
	}
	fpin, err := os.Open(filename, os.O_RDONLY, 0666)
	// err could be file not found, file found but not accessible etc
	// in any case test fails
	if err != nil {
		return nil, false
	}
	defer fpin.Close()
	fileSize, err := fpin.Seek(0, 2) // get file size
	if DEBUG {
		fmt.Printf("file size = %d\n", fileSize)
	}
	if err != nil {
		return nil, false
	}
	_, err = fpin.Seek(0, 0) // back to beginning

	hdr := new(ZipLocalHeader)
	//    fmt.Printf("size of hdr = %d\n", unsafe.Sizeof(*hdr))
	var hdrData [30]byte // size of header data fixed fields only

	fileList := make([]string, 0, 100)
	fnum := 0
	for {
		chRead, err = fpin.Read(&hdrData)
		if err != nil || chRead != 30 { /* TODO problem */
			fmt.Printf("Header read failed\n")
			return nil, false
		}

		if string(hdrData[0:4]) != ZIP_LocalHdrSig {
			if string(hdrData[0:4]) == ZIP_CentDirSig {
				break
			}
			fmt.Printf("bad magic number in local headers\n")
			fmt.Printf("got %v\n", hdrData[0:4])
			return nil, false
		}

		hdr.fileNameLen = SixteenBit(hdrData[26:28])

		if hdr.fileNameLen == 0 {
			// fmt.Printf("Reached end of zip file\n")
			break
		}

		fname := make([]byte, hdr.fileNameLen, hdr.fileNameLen)
		chRead, err = fpin.Read(fname)
		if err != nil || chRead != int(hdr.fileNameLen) { /* TODO problem */
		}
		fnum++
		fmt.Printf("File[%d]: %s\n", fnum, fname)

		l := len(fileList)
		c := cap(fileList)
		if l < c { // new item will fit
			fileList = fileList[0 : l+1]
			fileList[l] = string(fname)
		} else { // allocate more space
			newList := make([]string, l, c*2)
			n := copy(newList, fileList)
			if n != len(fileList) {
				fmt.Printf("fatal copy error 1\n")
				os.Exit(1)
			}
			fileList = newList
			l = len(fileList)
			c = cap(fileList)
			fileList = fileList[0 : l+1]
			fileList[l] = string(fname)
		}
		// skip the extra data fields
		hdr.extraFldLen = SixteenBit(hdrData[28:30])
		curPos, err = fpin.Seek(int64(hdr.extraFldLen), 1)
		curPos = curPos // TODO
		hdr.compressSize = ThirtyTwoBit(hdrData[18:22])
		newPos, err = fpin.Seek(int64(hdr.compressSize), 1) // seek past zip'd data
		if err != nil { /* TODO */
		}
		if newPos >= fileSize {
			// fmt.Printf("Reached end of zip file\n")
			break
		}
	}

	return fileList, true // success
}

func main() {
	var ok bool = true
	fmt.Printf("hello\n")
	flag.Parse()
	fmt.Printf("Flag got %d args on cmd line after command name\n", flag.NArg())
	if flag.NArg() == 0 { // do nothing
	} else { // whitespace sensitive
		for i := 0; i < flag.NArg(); i++ {
			fmt.Printf("%d %s\n", i, flag.Arg(i))
			//ok = readHeaders(flag.Arg(i))
			//if ok == false { /* tell somebody */ }

			var files []string
			files, ok = ListOfFiles(flag.Arg(i))
			if !ok {
				fmt.Printf("ListOfFiles failed\n")
			}
			if files == nil || !ok { /* TODO problem */
			} else {
				for ndx, val := range files {
					fmt.Printf("file[%d] = %s\n", ndx+1, val)
				}
			}
		}
	}
	os.Exit(0)
}
