// Copyright 2009-2012 David Rook. All rights reserved.

/*
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 * source can be found at http://www.github.com/hotei/go-zipfile
 * 
 * <David Rook> ravenstone13@cox.net
 * This is a working work-in-progress
 *      This version does only zip reading, no zip writing yet 
 *      Updated to match new go package rqmts on 2011-12-13 working again
 * 
 *    Additional documentation for package 'zip' can be found in doc.go

 Pkg returns too many fatal errors.  Need to feed error back out of package
 and let caller determin what's really fatal.  Current version quits if fed a
 corrupted file.  Which may be common in our intended use environment.
*/

package zipfile

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"time"
)

const (
	BITS_IN_INT     = 32
	MSDOS_EPOCH     = 1980
	ZIP_LocalHdrSig = "PK\003\004"
	ZIP_CentDirSig  = "PK\001\002"
	ZIP_STORED      = 0
	ZIP_DEFLATED    = 8
	TooBig          = 1<<(BITS_IN_INT-1) - 1
	LocalHdrSize    = 30
)

var (
	CantCreatReader  = errors.New("Cant create a NewReader")
	InvalidSigError  = errors.New("Bad Local Hdr Sig (invalid magic number)")
	InvalidCompError = errors.New("Bad compression method value")
	ShortReadError   = errors.New("short read")
	FutureTimeError  = errors.New("file's last Mod time is in future")
	Slice16Error     = errors.New("sixteenBit() did not get a 16 bit arg")
	Slice32Error     = errors.New("thirtytwoBit() did not get a 32 bit arg")
	CRC32MatchError  = errors.New("Stored CRC32 doesn't match computed CRC32")
	TooBigError      = errors.New("Can't use CRC32 if file > 2GB, Try unsetting Paranoid")
	ExpandingError   = errors.New("Cant expand array")
	CantHappenError  = errors.New("Cant happen - but did anyway :-(")
)

// used to control behavior of zip library code
var (
	Verbose  bool
	Paranoid bool
)

// A ZipReader provides sequential or random access to the contents of a zip archive.
// A zip archive consists of a sequence of files.
// The Next method advances to the next file in the archive (including the first),
// and then it can be treated as an io.Reader to access the file's data.
// You can also pull all the headers with  h := rz.Headers() and then open
// an individual file number n with rdr := h[n].Open()  See test suite for more examples.
//
// Example:
// func test_2() {
//	const testfile = "stuf.zip"
//
//	input, err := os.Open(testfile, os.O_RDONLY, 0666)
//	if err != nil {
//		fatal_err(err)
//	}
//	fmt.Printf("opened zip file %s\n", testfile)
//	rz, err := zip.NewReader(input)
//	if err != nil {
//		fatal_err(err)
//	}
//	hdr, err := rz.Next()
//	rdr, err := hdr.Open()
//	_, err = io.Copy(os.Stdout, rdr) // open first file only
//	if err != nil {
//		fatal_err(err)
//	}
// }
type ZipReader struct {
	current_file int
	reader       io.ReadSeeker
}

func NewReader(r io.ReadSeeker) (*ZipReader, error) {
	x := new(ZipReader)
	x.reader = r
	_, err := r.Seek(0, 0) // make sure we've got a seekable input  ? may be unnecessary ?
	// err might not be nil on return - caller MUST test
	return x, err
}

// Describes one entry in zip archive, might be compressed or stored (ie. type 8 or 0 only)
type Header struct {
	Name        string
	Size        int64 // size while uncompressed
	SizeCompr   int64 // size while compressed
	Typeflag    byte
	Mtime       time.Time // use 'go' version of time, not MSDOS version
	Compress    uint16    // only one method implemented and thats flate/deflate
	Offset      int64
	StoredCrc32 uint32
	Hreader     io.ReadSeeker
}

// Unpack header based on PKWare's APPNOTE.TXT
func (h *Header) unpackLocalHeader(src []byte) error {
	if string(src[0:4]) != ZIP_LocalHdrSig {
		if string(src[0:4]) == ZIP_CentDirSig { // reached last file, now into directory
			h.Size = -1 // signal last file reached
			return nil
		}
		return InvalidSigError // has invalid sig and its not last file in archive
	}
	h.Compress = sixteenBit(src[8:10])

	if h.Compress != ZIP_STORED && h.Compress != ZIP_DEFLATED {
		return InvalidCompError
	}
	h.Size = int64(thirtyTwoBit(src[22:26]))
	h.SizeCompr = int64(thirtyTwoBit(src[18:22]))
	h.StoredCrc32 = thirtyTwoBit(src[14:18])

	pktime := sixteenBit(src[10:12])
	pkdate := sixteenBit(src[12:14])
	h.Mtime = makeGoDate(pkdate, pktime)
	if h.Mtime.After(time.Now()) {
		fmt.Fprintf(os.Stderr, "%s: %v\n", h.Name, FutureTimeError)
		if Paranoid {
			fatal_err(FutureTimeError)
		} else {
			// warning or just ignore it ?
		}
	}
	if Verbose {
		fmt.Printf("Header time parsed to : %s\n", h.Mtime.String())
	}
	return nil
}

// grabs the next zip header from the archive
// returns one header pointer for each stored file
func (r *ZipReader) Headers() ([]*Header, error) {
	Hdrs := make([]*Header, 0, 20)
	_, err := r.reader.Seek(0, 0)
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}
	for {
		hdr, err := r.Next()
		if err != nil {
			if Paranoid {
				fatal_err(err)
			} else {
				return nil, err
			}
		}
		if hdr == nil {
			break
		}
		if Verbose {
			hdr.Dump()
		}
		Hdrs = append(Hdrs, hdr)
	}
	return Hdrs, nil
}

// BUG(mdr) getting short read for UNK reasons in Next()
//		assume body of zip is getting stored in header for some minor savings
//		is it described (and I assume allowed) in spec?

// decode PK formats and convert to go values, returns next Header pointer or
// 		nil when no more data available
func (r *ZipReader) Next() (*Header, error) {

	//	var localHdr [LocalHdrSize]byte (pre fixup)
	// start by reading fixed size fields (Name,Extra are vari-len)
	// after fixup we need to see this .Read([]byte)
	localHdr := make([]byte, LocalHdrSize)
	n, err := r.reader.Read(localHdr)
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}
	if n < LocalHdrSize {
		fmt.Printf("Read %d bytes of header = %v %s\n", LocalHdrSize, localHdr, localHdr)
		fmt.Printf("n(%d) < LocalHdrSize(%d)\n", n, LocalHdrSize)
		if Paranoid {
			fmt.Printf("Read %d bytes of header = %v\n", LocalHdrSize, localHdr)
			fatal_err(ShortReadError)
		} else {
			return nil, ShortReadError // BUG why unexpected - sometimes
		}
	}
	if Verbose {
		fmt.Printf("Read %d bytes of header = %v\n", LocalHdrSize, localHdr)
	}
	hdr := new(Header)
	hdr.Hreader = r.reader
	err = hdr.unpackLocalHeader(localHdr)
	if err != nil {
		return nil, err
	}
	if hdr.Size == -1 { // reached end of archive records, start of directory
		// not an error, return nil to signal no more data
		return nil, nil
	}
	fileNameLen := sixteenBit(localHdr[26:28])
	// TODO read past end of archive without seeing Central Directory ? NOT POSSIBLE ?
	// what about multi-volume disks?  Do they have any Central Dir data?
	if fileNameLen == 0 {
		fmt.Fprintf(os.Stderr, "read past end of archive and didn't find the Central Directory")
		if Paranoid {
			fatal_err(CantHappenError)
		} else {
			// or is it just end-of-file on multi-vol?
			return nil, nil // ignore it
		}
	}
	fname := make([]byte, fileNameLen)
	n, err = r.reader.Read(fname)
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}
	if n < int(fileNameLen) {
		fmt.Printf("n < fileNameLen\n")
		if Paranoid {
			fatal_err(ShortReadError)
		} else {
			return nil, ShortReadError
		}
	}
	if Verbose {
		fmt.Printf("filename: %s \n", fname)
	}
	hdr.Name = string(fname)
	// read extra data if present
	if Verbose {
		fmt.Printf("reading extra data if present\n")
	}
	extraFieldLen := sixteenBit(localHdr[28:30])
	// skip over it if needed, but in either case, save current position after seek
	// ie. degenerate case is Seek(0,1) but do it anyway
	currentPos, err := r.reader.Seek(int64(extraFieldLen), 1)
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}
	hdr.Offset = currentPos
	// seek past compressed/stored blob to start of next header
	_, err = r.reader.Seek(hdr.SizeCompr, 1)
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}

	// NOTE: side effect is to move r.reader pointer to start of next header
	return hdr, nil
}

// Simple listing of header, same data should appear for the command "unzip -v file.zip"
// but with slightly different order and formatting  TODO - make format more similar ?
func (hdr *Header) Dump() {
	if hdr.Name == "" {
		return
	}
	Mtime := hdr.Mtime.UTC()
	//	fmt.Printf("%s: Size %d, Size Compressed %d, Type flag %d, LastMod %s, ComprMeth %d, Offset %d\n",
	//		hdr.Name, hdr.Size, hdr.SizeCompr, hdr.Typeflag, Mtime.String(), hdr.Compress, hdr.Offset)
	var method string
	if hdr.Compress == 8 {
		method = "Deflated"
	} else {
		method = "Stored"
	}

	// sec := time.SecondsToUTC(hdr.Mtime)
	// fmt.Printf("Header time parsed to : %s\n", sec.String())
	fmt.Printf("%8d  %8s  %7d   %4d-%02d-%02d %02d:%02d:%02d  %08x  %s\n",
		hdr.SizeCompr, method, hdr.Size,
		Mtime.Year(), Mtime.Month(), Mtime.Day(), Mtime.Hour(), Mtime.Minute(), Mtime.Second(),
		hdr.StoredCrc32, hdr.Name)
}

func (h *Header) Open() (io.Reader, error) {
	_, err := h.Hreader.Seek(h.Offset, 0)
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}
	comprData := make([]byte, h.SizeCompr)
	n, err := h.Hreader.Read(comprData)
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}
	if int64(n) < h.SizeCompr {
		fmt.Printf("read(%d) which is less than stored compressed size(%d)", n, h.SizeCompr)
		if Paranoid {
			fatal_err(ShortReadError)
		} else {
			return nil, ShortReadError
		}
	}
	if Verbose {
		fmt.Printf("Header.Open() Read in %d bytes of compressed (deflated) data\n", n)
		// prints out filename etc so we can later validate expanded data is appropriate
		h.Dump()
	}
	// got it as comprData in RAM, now need to expand it
	in := bytes.NewBuffer(comprData) // fill new buffer with compressed data
	inpt := flate.NewReader(in)      // attach a reader to the buffer
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return inpt, err // TODO not always safe but currently necessary for big files, will fix soon
		}
	}

	// BUG(mdr) need to handle bigger files gracefully

	if h.Size > TooBig {
		if Paranoid {
			fatal_err(TooBigError)
		} else {
			return nil, TooBigError
		}
	}

	// normally we want to crc32 the buffer and check computed vs stored crc32
	b := new(bytes.Buffer) // create a new buffer with io methods
	var n2 int64
	n2, err = io.Copy(b, inpt) // now fill buffer from compressed data using inpt
	if err != nil {
		if Paranoid {
			fatal_err(err)
		} else {
			return nil, err
		}
	}
	if n2 < h.Size {
		fmt.Printf("Actually copied %d, expected to copy %d\n", n, h.Size)
		if Paranoid {
			fatal_err(ShortReadError)
		} else {
			return nil, ShortReadError
		}
	}
	// TODO this feels like an extra step but not sure how to shorten it yet
	// problem is we can't run ChecksumIEEE on Buffer, it requires []byte arg
	// update: Russ Cox provided advice on method of attack, still need to implement
	expdData := make([]byte, h.Size) // make the expanded buffer into a byte array
	n, err = b.Read(expdData)        // copy buffer into expdData
	if int64(n) < h.Size {
		fmt.Printf("copied %d, expected %d\n", n, h.Size)
		if Paranoid {
			fatal_err(ShortReadError)
		} else {
			return nil, ShortReadError
		}
	}
	mycrc32 := crc32.ChecksumIEEE(expdData)
	if Verbose {
		fmt.Printf("Computed Checksum = %0x, stored checksum = %0x\n", mycrc32, h.StoredCrc32)
	}
	if mycrc32 != h.StoredCrc32 {
		if Paranoid {
			fatal_err(CRC32MatchError)
		} else {
			return nil, CRC32MatchError
		}
	}
	if false {
		//========	this worked, producing legal zip files ==========
		fp, err := ioutil.TempFile(".", "test")
		if err != nil {
			fmt.Printf("walker: err %v\n", err)
		}
		nw, err := fp.Write(expdData)
		_ = nw
		if err != nil {
			fmt.Printf("zipfile: write fail err %v\n", err)
		}
		//===========================================================
	}
	bufReader := bytes.NewReader(expdData)
	return bufReader, nil // who closes bufReader and how?
}

//	convert PKware date, time uint16s into seconds since Unix Epoch
func makeGoDate(d, t uint16) time.Time {
	var year, month, day uint16
	year = d & 0xfe00
	year >>= 9
	month = d & 0x01e0
	month >>= 5
	day = d & 0x001f
	day = day

	var hour, minute, second uint16
	hour = t & 0xf800
	hour >>= 11
	minute = t & 0x01e0
	minute >>= 5
	second = (t & 0x001f) * 2
	second = second

	ftYear := int(year + MSDOS_EPOCH)
	ftMonth := time.Month(month)
	ftDay := int(day)
	ftHour := int(hour)
	ftMinute := int(minute)
	ftSecond := int(second)
	//	ftZoneOffset := 0
	ftZone := time.UTC

	ft := time.Date(ftYear, ftMonth, ftDay, ftHour, ftMinute, ftSecond, 0, ftZone)
	if Verbose {
		fmt.Printf("year(%d) month(%d) day(%d) \n", year, month, day)
		fmt.Printf("hour(%d) minute(%d) second(%d)\n", hour, minute, second)
	}
	// TODO this checking is approximate for now, daysinmonth not checked fully
	// TODO we wont know file name at this point unless Verbose is also true
	//  ? is that a problem or not ?
	if Paranoid {
		badDate := false
		// no such thing as a bad year as 0..127 are valid
		// and represent 1980 thru 2107
		// if a file's Mtime is in the future Paranoid will it catch later
		if !inRangeInt(1, int(month), 12) {
			badDate = true
		}
		if !inRangeInt(1, int(day), 31) {
			badDate = true
		}
		if !inRangeInt(0, int(hour), 23) {
			badDate = true
		}
		if !inRangeInt(0, int(minute), 59) {
			badDate = true
		}
		if !inRangeInt(0, int(second), 59) {
			badDate = true
		}
		if badDate {
			fmt.Fprintf(os.Stderr, "Encountered bad Mod Date/Time: \n")
			fmt.Fprintf(os.Stderr, "year(%d) month(%d) day(%d) \n", year, month, day)
			fmt.Fprintf(os.Stderr, "hour(%d) minute(%d) second(%d)\n", hour, minute, second)
		}
	}
	return ft
}

// true if b is between a and c, order not important
// 1,3,5 => true
// 5,3,1 => true
// 1,7,5 => false
// 5,7,1 => false
func inRangeInt(a, b, c int) bool {
	if a > c {
		a, c = c, a
	}
	if a > b {
		return false
	}
	if b > c {
		return false
	}
	return true
}

// convert from little endian two byte slice to int16
func sixteenBit(n []byte) uint16 {
	if len(n) != 2 {
		fatal_err(Slice16Error)
	}
	var rc uint16
	rc = uint16(n[1])
	rc <<= 8
	rc |= uint16(n[0])
	return rc
}

// convert from little endian four byte slice to int32
func thirtyTwoBit(n []byte) uint32 {
	if len(n) != 4 {
		fatal_err(Slice32Error)
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

func fatal_err(erx error) {
	fmt.Printf("stopping because: %s \n", erx)
	os.Exit(1)
}

func ReaderAtSection(r io.ReaderAt, start, end int64) io.ReaderAt {
	return nil
}

func ReaderAtStream(r io.ReaderAt) io.Reader {
	return nil
}
