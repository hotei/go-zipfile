// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// source can be found at http://www.github.com/hotei/go-zipfile
//
// <David Rook> ravenstone13@cox.net
// This is a work-in-progress
//     This version does only zip reading, no zip writing yet

/*
   Additional documentation for package 'zip' can be found in doc.go
*/
package zip


import (
	"bytes"
	"compress/flate"
	"hash/crc32"
	"fmt"
	"io"
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
	InvalidSigError  os.Error = os.ErrorString("Bad Local Hdr Sig (magic number)")
	InvalidCompError os.Error = os.ErrorString("Bad compression method value")
	ShortReadError   os.Error = os.ErrorString("short read")
	FutureTimeError  os.Error = os.ErrorString("file's last Mod time is in future")
	Slice16Error     os.Error = os.ErrorString("sixteenBit() did not get a 16 bit arg")
	Slice32Error     os.Error = os.ErrorString("thirtytwoBit() did not get a 32 bit arg")
	CRC32MatchError  os.Error = os.ErrorString("Stored CRC32 doesn't match computed CRC32")
	TooBigError      os.Error = os.ErrorString("Can't use CRC32 if file > 2GB, Try unsetting Paranoid")
	ExpandingError   os.Error = os.ErrorString("Cant expand array")
	CantHappenError  os.Error = os.ErrorString("Cant happen - but did anyway :-(")
)

// used to control behavior of zip library code
type Dash struct {
	Verbose  bool
	Paranoid bool
}

var (
	Dashboard = new(Dash)
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

func NewReader(r io.ReadSeeker) (*ZipReader, os.Error) {
	x := new(ZipReader)
	x.reader = r
	_, err := r.Seek(0, 0) // make sure we've got a seekable input  ? may be unnecessary ?
	if err != nil {
		fatal_err(err)
	}
	return x, nil
}

// Describes one entry in zip archive, might be compressed or stored (ie. type 8 or 0 only)
type Header struct {
	Name        string
	Size        int64 // size while uncompressed
	SizeCompr   int64 // size while compressed
	Typeflag    byte
	Mtime       int64  // use 'go' version of time, not MSDOS version
	Compress    uint16 // only one method implemented and thats flate/deflate
	Offset      int64
	StoredCrc32 uint32
	Hreader     io.ReadSeeker
}

// Unpack header based on PKWare's APPNOTE.TXT
func (h *Header) unpackLocalHeader(src [LocalHdrSize]byte) os.Error {
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
	if h.Mtime > time.Seconds() {
		fmt.Fprintf(os.Stderr, "%s: %s\n", h.Name, FutureTimeError.String())
		if Dashboard.Paranoid {
			fatal_err(FutureTimeError)
		}
	}
	if Dashboard.Verbose {
		sec := time.SecondsToUTC(h.Mtime)
		fmt.Printf("Header time parsed to : %s\n", sec.String())
	}
	return nil
}


// grabs the next zip header from the archive
// returns one header record for each stored file
func (r *ZipReader) Headers() []Header {
	// initial cap of 20 is arbitrary but kept low to accomodate small RAM systems, this is roughly 1KB
	Hdrs := make([]Header, 20)

	_, err := r.reader.Seek(0, 0)
	if err != nil {
		fatal_err(err)
	}
	for {
		hdr, err := r.Next()
		if err != nil {
			fatal_err(err)
		}
		if hdr == nil {
			break
		}
		if Dashboard.Verbose {
			hdr.Dump()
		}

		l := len(Hdrs)
		c := cap(Hdrs)
		if l < c { // append hdr to current array
			Hdrs = Hdrs[0 : l+1]
			Hdrs[l] = *hdr
		} else { // other wise double size of array cap and retry
			newHdrs := make([]Header, l, c*2)
			n := copy(newHdrs, Hdrs)
			if n != len(Hdrs) {
				fatal_err(ExpandingError)
			}
			Hdrs = newHdrs
			l := len(Hdrs)
			// c := cap(Hdrs)
			Hdrs = Hdrs[0 : l+1]
			Hdrs[l] = *hdr
		}
	}
	return Hdrs
}

// decode PK formats and convert to go values, returns next Header record or
// nil when no more data available
func (r *ZipReader) Next() (*Header, os.Error) {

	var localHdr [LocalHdrSize]byte // start by reading fixed size fields (Name,Extra are vari-len)
	n, err := r.reader.Read(&localHdr)
	if err != nil {
		fatal_err(err)
	}
	if n < LocalHdrSize {
		fatal_err(ShortReadError)
	}
	if Dashboard.Verbose {
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
		if Dashboard.Paranoid {
			fatal_err(CantHappenError)
		}
		// or is it just end-of-file on multi-vol?
		return nil, nil
	}
	fname := make([]byte, fileNameLen)
	n, err = r.reader.Read(fname)
	if err != nil {
		fatal_err(err)
	}
	if n < int(fileNameLen) {
		fatal_err(ShortReadError)
	}
	if Dashboard.Verbose {
		fmt.Printf("filename: %s \n", fname)
	}
	hdr.Name = string(fname)
	// read extra data if present
	if Dashboard.Verbose {
		fmt.Printf("reading extra data if present\n")
	}
	extraFieldLen := sixteenBit(localHdr[28:30])
	// skip over it if needed, but in either case, save current position after seek
	// ie. degenerate case is Seek(0,1) but do it anyway
	currentPos, err := r.reader.Seek(int64(extraFieldLen), 1)
	if err != nil {
		fatal_err(err)
	}
	hdr.Offset = currentPos
	// seek past compressed/stored blob to start of next header
	_, err = r.reader.Seek(hdr.SizeCompr, 1)
	if err != nil {
		fatal_err(err)
	}

	// NOTE: side effect is to move r.reader pointer to start of next header
	return hdr, nil
}

// Simple listing of header, same data should appear for the command "unzip -v file.zip"
// but with slightly different order and formatting  TODO - make format more similar ?
func (hdr *Header) Dump() {
	Mtime := time.SecondsToUTC(hdr.Mtime)
	fmt.Printf("%s: Size %d, Size Compressed %d, Type flag %d, LastMod %s, ComprMeth %d, Offset %d\n",
		hdr.Name, hdr.Size, hdr.SizeCompr, hdr.Typeflag, Mtime.String(), hdr.Compress, hdr.Offset)
}

func (h *Header) Open() (io.Reader, os.Error) {
	_, err := h.Hreader.Seek(h.Offset, 0)
	if err != nil {
		fatal_err(err)
	}
	comprData := make([]byte, h.SizeCompr)
	n, err := h.Hreader.Read(comprData)
	if err != nil {
		fatal_err(err)
	}
	if int64(n) < h.SizeCompr {
		fatal_err(ShortReadError)
	}
	if Dashboard.Verbose {
		fmt.Printf("Header.Open() Read in %d bytes of compressed (deflated) data\n", n)
		// prints out filename etc so we can later validate expanded data is appropriate
		h.Dump()
	}
	// got it as comprData in RAM, now need to expand it
	in := bytes.NewBuffer(comprData) // fill new buffer with compressed data
	inpt := flate.NewInflater(in)    // attach a reader to the buffer
	if err != nil {
		fatal_err(err)
	}
	if !Dashboard.Paranoid {
		return inpt, nil // TODO not safe but currently necessary for big files, will fix soon
	}

	if h.Size > TooBig {
		fatal_err(TooBigError)
	}

	// normally we want to crc32 the buffer and check computed vs stored crc32
	b := new(bytes.Buffer) // create a new buffer with io methods
	var n2 int64
	n2, err = io.Copy(b, inpt) // now fill buffer from compressed data using inpt
	if err != nil {
		fatal_err(err)
	}
	if n2 < h.Size {
		fmt.Printf("Actually copied %d, expected to copy %d\n", n, h.Size)
		fatal_err(ShortReadError)
	}
	// TODO this feels like an extra step but not sure how to shorten it yet
	// problem is we can't run ChecksumIEEE on Buffer, it requires []byte arg
	// update: Russ Cox provided advice on method of attack, still need to implement
	expdData := make([]byte, h.Size) // make the expanded buffer into a byte array
	n, err = b.Read(expdData)        // copy buffer into expdData
	if int64(n) < h.Size {
		fmt.Printf("copied %d, expected %d\n", n, h.Size)
		fatal_err(ShortReadError)
	}
	mycrc32 := crc32.ChecksumIEEE(expdData)
	if Dashboard.Verbose {
		fmt.Printf("Computed Checksum = %0x, stored checksum = %0x\n", mycrc32, h.StoredCrc32)
	}
	if mycrc32 != h.StoredCrc32 {
		fatal_err(CRC32MatchError)
	}
	// TODO can we reuse in and inpt without problems?
	in2 := bytes.NewBuffer(comprData)
	inpt2 := flate.NewInflater(in2)
	if err != nil {
		fatal_err(err)
	}
	// TODO ??? how do we insure eventual close of inpt2
	return inpt2, nil
}


//	convert PKware date, time uint16s into seconds since Unix Epoch
func makeGoDate(d, t uint16) int64 {
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

	ft := new(time.Time)
	ft.Year = int64(year) + MSDOS_EPOCH
	ft.Month = int(month)
	ft.Day = int(day)
	ft.Hour = int(hour)
	ft.Minute = int(minute)
	ft.Second = int(second)
	ft.ZoneOffset = 0
	ft.Zone = "UTC"

	if Dashboard.Verbose {
		fmt.Printf("year(%d) month(%d) day(%d) \n", year, month, day)
		fmt.Printf("hour(%d) minute(%d) second(%d)\n", hour, minute, second)
	}
	// TODO this checking is approximate for now, daysinmonth not checked fully
	// TODO we wont know file name at this point unless Verbose is also true
	//  ? is that a problem or not ?
	if Dashboard.Paranoid {
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
	return ft.Seconds()
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

func fatal_err(erx os.Error) {
	fmt.Printf("%s \n", erx)
	os.Exit(1)
}


func ReaderAtSection(r io.ReaderAt, start, end int64) io.ReaderAt {
	return nil
}


func ReaderAtStream(r io.ReaderAt) io.Reader {
	return nil
}
