// zipfile.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// <David Rook> ravenstone13@cox.net
// This is a work-in-progress

package main

import (
	"bytes"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	//	"io/ioutil"
	"os"
	"compress/flate"
	//    "unsafe"
)

const DEBUG = false

const (
	MSDOS_EPOCH     = 1980
	ZIP_LocalHdrSig = "PK\003\004"
	ZIP_CentDirSig  = "PK\001\002"
	ZIP_STORED      = 0
	ZIP_DEFLATED    = 8
)

type ZipFile struct {
	fileName string
	//	fpin     *os.File   // does it make sense to carry this in struct?
	fileSize int64
	numFiles uint32
	headers  []ZipLocalHeader
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


// function to read thru whole file picking up all file headers into headers[]
// keep fileName, calculate archive size fileSize, count numFiles
func (z *ZipFile) Init(filename string) bool {
	var n int
	var curPos int64
	var newPos int64
	var err os.Error
	var fpin *os.File

	z.fileName = filename
	z.headers = make([]ZipLocalHeader, 0, 100)
	fpin, err = os.Open(filename, os.O_RDONLY, 0666)
	// err could be file not found, file found but not accessible etc
	// in any case test fails
	if err != nil {
		return false
	}
	defer fpin.Close()
	z.fileSize, err = fpin.Seek(0, 2) // get file size
	if DEBUG {
		fmt.Printf("%s size = %d\n", z.fileName, z.fileSize)
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
		n, err = fpin.Read(hdr.fileName)
		if err != nil || n != int(hdr.fileNameLen) { /* TODO problem */
		}
		if DEBUG {
			fmt.Printf("zip includes file: %s\n", hdr.fileName)
		}

		extra := make([]byte, hdr.extraFldLen, hdr.extraFldLen)
		n, err = fpin.Read(extra)
		if err != nil || n != int(hdr.extraFldLen) { /* TODO problem */
		}
		//        fmt.Printf("extra = %v\n", extra)
		curPos, err = fpin.Seek(0, 1)
		//        fmt.Printf("current position is %d\n", curPos )
		hdr.dataOffset = curPos
		newPos, err = fpin.Seek(int64(hdr.compressSize), 1) // advance to next header
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
		z.numFiles++
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
		if err != nil || uint32(n) != z.headers[which].compressSize {
			fmt.Printf("zip data read failed\n")
			os.Exit(1)
		}
		fmt.Printf("Read in %d bytes of compressed (deflated) data\n", n)

		// got it in RAM, now need to expand it
		b := new(bytes.Buffer)          // create a new buffer with io methods
		in := bytes.NewBuffer(compData) // fill new buffer with compressed data
		r := flate.NewInflater(in)      // attach a reader to the buffer
		if err != nil {
			fmt.Printf("%s has err = %v\n", z.fileName, err)
			os.Exit(1)
		}
		defer r.Close()         // make sure we eventually close the reader
		b.Reset()               // empty out the buffer
		n2, err = io.Copy(b, r) // now fill buffer again from compressed data in the reader object
		if err != nil {
			fmt.Printf("%s has err = %v\n", z.fileName, err)
			os.Exit(1)
		}

		expdData := make([]byte, z.headers[which].unComprSize)
		n, err = b.Read(expdData)
		if err != nil {
			fmt.Printf("%s has err = %v\n", z.fileName, err)
			os.Exit(1)
		}

		fmt.Printf("Size of expdData = %d\n", len(expdData))
		mycrc32 := crc32.ChecksumIEEE(expdData)
		fmt.Printf("Computed Checksum = %0x, stored checksum = %0x\n", mycrc32, z.headers[which].crc32)

		if mycrc32 != z.headers[which].crc32 {
			fmt.Printf("CRC32 mismatch for %s\n", z.fileName)
			os.Exit(1)
		}
		n2, err = io.Copy(os.Stdout, b) // copy the buffer to output
		if n2 != int64(z.headers[which].unComprSize) {
			fmt.Printf("%s has err = %v\n", z.fileName, os.EINVAL)
			os.Exit(1)
		}

		if err != nil {
			fmt.Printf("%s has err = %v\n", z.fileName, err)
			os.Exit(1)
		}
		// if buffer data is text we can do this
		//s := b.String()				//
		//fmt.Printf("OUTPUT of inflater: \n%s\n", s)
	}

	if z.headers[which].compressMeth == ZIP_STORED {
		expdData := make([]byte, z.headers[which].unComprSize)
		n, err = inf.Read(expdData)
		fmt.Printf("Read in %d bytes of uncompressed (stored) data\n", n)
		if err != nil {
			fmt.Printf("zip data read failed\n")
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(expdData))
	}
}

// list files in named archive
func test_1() {
	flag.Parse()
	fmt.Printf("Flag got %d args on cmd line after command name\n", flag.NArg())
	if flag.NArg() == 0 { // do nothing
	} else { // whitespace sensitive
		for i := 0; i < flag.NArg(); i++ {
			fmt.Printf("%d %s\n", i, flag.Arg(i))

			myzipper := new(ZipFile)
			ok := myzipper.Init(flag.Arg(i)) // read headers into array, get size
			if !ok { /* TODO */
			}

			var files []string
			files = myzipper.ListOfFiles()
			if len(files) == 0 {
				if DEBUG {
					fmt.Printf("No files in zipfile %s ...???\n", myzipper.fileName)
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
	myzipper := new(ZipFile)
	ok := myzipper.Init(fname) // read - this one is stored
	if !ok {
		fmt.Printf("Can't open requested file: %s\n", fname)
		return
	}
	myzipper.ListZip(0) // list the first file to stdout
}

func test_4(fname string) {
	myzipper := new(ZipFile)
	ok := myzipper.Init(fname)
	if !ok {
		fmt.Printf("Can't open requested file: %s\n", fname)
		return
	}
	if myzipper.numFiles >= 2 {
		fmt.Printf("file[0] zip'd size = %d\n", myzipper.headers[0].compressSize)
		fmt.Printf("file[0] unzip'd size = %d\n", myzipper.headers[0].unComprSize)
		fmt.Printf("file[1] zip'd size = %d\n", myzipper.headers[1].compressSize)
		fmt.Printf("file[1] unzip'd size = %d\n", myzipper.headers[1].unComprSize)
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
	fmt.Printf("<end of test of zipfile>\n")
	os.Exit(0)
}

func zip_decode_date(d uint16) {
	var year, month, day uint16
	year = d & 0xfe00
	year >>= 9
	//fmt.Printf("year = %d\n", year + MSDOS_EPOCH)

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
