// zipfile.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"compress/flate"
	//    "unsafe"
)

const DEBUG = true
const (
	DOSEPOCH        = 1980
	ZIP_LocalHdrSig = "PK\003\004"
	ZIP_CentDirSig  = "PK\001\002"
	ZIP_STORED      = 0
	ZIP_DEFLATED    = 8
)

type ZipFile struct {
	fileName string
	fpin     *os.File
	fileSize int64
	headers  []ZipLocalHeader
	readposn int64
}

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
	dataOffset   int64 // where we seek to pick up the data
}

// test function to read thru file headers
func (z *ZipFile) Init(filename string) bool {
	var n int
	var curPos int64
	var newPos int64
	var err os.Error

	z.fileName = filename
	z.headers = make([]ZipLocalHeader, 0, 100)
	z.fpin, err = os.Open(filename, os.O_RDONLY, 0666)
	// err could be file not found, file found but not accessible etc
	// in any case test fails
	if err != nil {
		return false
	}
	defer z.fpin.Close()
	z.fileSize, err = z.fpin.Seek(0, 2) // get file size
	if DEBUG {
		fmt.Printf("file size = %d\n", z.fileSize)
	}
	if err != nil {
		return false
	}
	_, err = z.fpin.Seek(0, 0) // back to beginning

	hdr := new(ZipLocalHeader)
	//    fmt.Printf("size of hdr = %d\n", unsafe.Sizeof(*hdr))
	var hdrData [30]byte // size of header data fixed fields only

	for {
		n, err = z.fpin.Read(&hdrData)
		if err != nil || n != 30 { /* TODO problem */
			fmt.Printf("Header read failed\n")
			os.Exit(1)
		}
		//	    fmt.Printf("data = %v\n", hdrData)

		if string(hdrData[0:4]) != ZIP_LocalHdrSig {
			if string(hdrData[0:4]) == ZIP_CentDirSig {
				break
			}
			fmt.Printf("bad magic number in local headers\n")
			fmt.Printf("got %v\n", hdrData[0:4])
			return false
		}

		hdr.ver2Extract = SixteenBit(hdrData[4:6])
		//        fmt.Printf("Extract version req = %d\n", hdr.ver2Extract )

		hdr.generalBits = SixteenBit(hdrData[6:8])
		//        fmt.Printf("General bits = %d\n", hdr.generalBits )

		hdr.compressMeth = SixteenBit(hdrData[8:10])
		//fmt.Printf("Compress Method = %d\n", hdr.compressMeth)
		if hdr.compressMeth != ZIP_STORED && hdr.compressMeth != ZIP_DEFLATED {
			fmt.Printf("Trouble -- unimplemented compression meth %d\n", hdr.compressMeth)
			os.Exit(1)
		}

		hdr.lastModTime = SixteenBit(hdrData[10:12])
		//fmt.Printf("LastModTime = %d\n", hdr.lastModTime)
		//		zip_decode_time(hdr.lastModTime)

		hdr.lastModDate = SixteenBit(hdrData[12:14])
		//fmt.Printf("LastModDate = %d\n", hdr.lastModDate)
		//		zip_decode_date(hdr.lastModDate)

		hdr.crc32 = ThirtyTwoBit(hdrData[14:18])
		//        fmt.Printf("crc32 = %4x\n", hdr.crc32)

		hdr.compressSize = ThirtyTwoBit(hdrData[18:22])
		//        fmt.Printf("compressSize = %d\n", hdr.compressSize)

		hdr.unComprSize = ThirtyTwoBit(hdrData[22:26])
		//        fmt.Printf("unComprSize = %d\n", hdr.unComprSize)

		hdr.fileNameLen = SixteenBit(hdrData[26:28])
		//fmt.Printf("fileNameLen = %d\n", hdr.fileNameLen)

		if hdr.fileNameLen == 0 {
			fmt.Printf("Reached end of zip file\n")
			break
		}

		hdr.extraFldLen = SixteenBit(hdrData[28:30])
		//        fmt.Printf("extraFldLen = %d\n", hdr.extraFldLen)

		hdr.fileName = make([]byte, hdr.fileNameLen, hdr.fileNameLen)
		n, err = z.fpin.Read(hdr.fileName)
		if err != nil || n != int(hdr.fileNameLen) { /* TODO problem */
		}
		fmt.Printf("filename = %s\n", hdr.fileName)

		extra := make([]byte, hdr.extraFldLen, hdr.extraFldLen)
		n, err = z.fpin.Read(extra)
		if err != nil || n != int(hdr.extraFldLen) { /* TODO problem */
		}
		//        fmt.Printf("extra = %v\n", extra)
		curPos, err = z.fpin.Seek(0, 1)
		//        fmt.Printf("current position is %d\n", curPos )
		hdr.dataOffset = curPos
		newPos, err = z.fpin.Seek(int64(hdr.compressSize), 1) // advance to next header
		if newPos != int64(hdr.compressSize)+curPos {
			fmt.Printf("advance to next header failed %d != %d\n", newPos, int64(hdr.compressSize)+curPos)
			os.Exit(1)
		}
		if newPos >= z.fileSize {
			fmt.Printf("Reached end of zip file\n")
			break
		}
		l := len(z.headers)
		c := cap(z.headers)
		if l < c {
			z.headers = z.headers[0 : l+1]
			z.headers[l] = *hdr
		} else { // TODO
			if DEBUG {
				fmt.Printf("allocating more header records\n")
			}
			newHdrs := make([]ZipLocalHeader, l, c*2)
			n := copy(newHdrs, z.headers)
			if n != len(z.headers) {
				fmt.Printf("fatal copy error 1\n")
				os.Exit(1)
			}
			z.headers = newHdrs
			l = len(z.headers)
			c = cap(z.headers)
			z.headers = z.headers[0 : l+1]
			z.headers[l] = *hdr
		}
	}
	if DEBUG {
		fmt.Printf("Read in %d header record(s)\n", len(z.headers))
	}
	return true // ok
}

// return nil, false if Zip is bad or empty
// TODO ? handle error better how ?
func (z *ZipFile) ListOfFiles() []string {
	if DEBUG {
		fmt.Printf("Testing ListOfFiles( %s )\n", z.fileName)
	}

	fileList := make([]string, len(z.headers), len(z.headers))
	for ndx, val := range z.headers {
		fileList[ndx] = string(val.fileName)
	}
	return fileList // success
}

// send specified zip'd data to stdout
func (z *ZipFile) ListZip(which int) {
	var n int
	var n2 int64
	inf, err := os.Open(z.fileName, os.O_RDONLY, 0666)
	defer inf.Close()
	inf.Seek(z.headers[which].dataOffset, 0)

	if z.headers[which].compressMeth == ZIP_DEFLATED {
		compData := make([]byte, z.headers[which].compressSize)
		n, err = inf.Read(compData)
		fmt.Printf("Read in %d bytes of uncompressed data\n", n)
		if err != nil {
			fmt.Printf("zip data read failed\n")
			os.Exit(1)
		}

		fmt.Printf("slice [0:80] of zip'd data: %v\n", compData[0:80])
		// got it in RAM, now need to expand it
		b := new(bytes.Buffer)          // create a new buffer with io methods
		in := bytes.NewBuffer(compData) // copy compressed data into new buf
		r := flate.NewInflater(in)
		if err != nil {
			fmt.Printf("%s has err = %v\n", z.fileName, err)
			os.Exit(1)
		}
		defer r.Close()
		b.Reset()					// empty out the buffer
		n2, err = io.Copy(b, r)		// now fill it from compressed data
		if err != nil {
			fmt.Printf("%s has err = %v\n", z.fileName, err)
			os.Exit(1)
			// fmt.Printf("r = %v\n", r)
			r.Close()
		}
		n2, err = io.Copy(os.Stdout, b)
		
		//s := b.String()				// 
		//fmt.Printf("OUTPUT of inflater: \n%s\n", s)
		n2 = n2
		//expdData := make([]byte, z.headers[which].unComprSize)
		//expdData = expdData
	}

	if z.headers[which].compressMeth == ZIP_STORED {
		expdData := make([]byte, z.headers[which].unComprSize)
		n, err = inf.Read(expdData)
		fmt.Printf("Read in %d bytes of uncompressed data\n", n)
		if err != nil {
			fmt.Printf("zip data read failed\n")
			os.Exit(1)
		}
		fmt.Printf("%v\n", expdData)
	}
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

			myzipper := new(ZipFile)
			ok = myzipper.Init(flag.Arg(i)) // read headers into array
			fmt.Printf("file[1] zip'd size = %d\n", myzipper.headers[0].compressSize)
			fmt.Printf("file[1] unzip'd size = %d\n", myzipper.headers[0].unComprSize)

			var files []string
			files = myzipper.ListOfFiles()
			if len(files) == 0 {
				fmt.Printf("No files in zipfile %s ...???\n", myzipper.fileName)
			}
			if files == nil || !ok { /* TODO problem */
			} else {
				for ndx, val := range files {
					fmt.Printf("file[%d] = %s\n", ndx+1, val)
				}
			}
		}
	}

	myzipper := new(ZipFile)
	ok = myzipper.Init("stuf.zip") // read headers into array
	myzipper.ListZip(0)
	os.Exit(0)
}

func zip_decode_date(d uint16) {
	var year, month, day uint16
	year = d & 0xfe00
	year >>= 9
	//fmt.Printf("year = %d\n", year+DOSEPOCH)

	month = d & 0x01e0
	month >>= 5
	//fmt.Printf("month = %d\n", month)

	day = d & 0x001f
	day = day
	//fmt.Printf("day = %d\n", day)
}

func zip_decode_time(t uint16) {
	var hour, minute, second uint16
	hour = t & 0xf800
	hour >>= 11
	//fmt.Printf("hour = %d\n", hour)

	minute = t & 0x01e0
	minute >>= 5
	//fmt.Printf("minute = %d\n", minute)

	second = (t & 0x001f) * 2
	second = second
	//fmt.Printf("second = %d\n", second)
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
