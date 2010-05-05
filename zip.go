// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// source can be found at http://www.github.com/hotei/go-zipfile
//
// <David Rook> ravenstone13@cox.net
// This is a work-in-progress
//     This version does only zip reading, no zip writing yet

/*
   package zip docs

PLEASE - take a look at the ziptest.go example for an overview of how the
library can be used.  

LIMITATIONS:
Most significant is that this is a read-only library at present.  Could change
but my first priority was to read zip files I have, not create new ones.

At present there is a limitation of 2GB on expanded
files if you set paranoid mode - ie if you want CRC32 checking done after
expansion.  This is a limitation currently imposed by the IEEECRC32 function.
With paranoid mode off you should be able to read files up to 9 Billion GiB.
Older versions of zip only supported a max 4 of GB file sizes but later 
zip versions expanded that to "big enough".  Older versions also limited the number
of files in an archive to 16 bits (65536 files) but newer versions have upped
that number to "big enough" also.

Paranoid mode will also abort if it encounters an invalid date, like month 13
or a modification date that's in the future compared to the time.Seconds() 
when the program is run.  Ie "NOW".  

Paranoid mode can be turned off by setting zip.Dashboard.Paranoid = false in
your program.  One reason for a paranoid mode is that in the MSDOS/MSWindows 
world a lot of virus programs messed with 
dates to purposely screw up your backup and restore programs.  With paranoid =
false you'll still see a warning to STDERR about the problems encountered, but
it will not abort.

Paranoid mode may cause smaller systems to run out of memory as the Open() function
pulls the contents of the zip archive entry into memory to uncompress and 
run the IEEEcrc32().  This obviously depends on the size of the files you're 
working with as well as your system's capacity.

This version of the zip library does NOT look at the central header areas at
the end of the zip archive.  Instead it builds headers on the fly by reading the
actual archived data. I feel that reading the actual data is useful to validate
the readability of the media. 

There is an opportunity to do some additional checking
in paranoid mode by comparing the actual headers with the stored ones.  That's
on the "TODO" list.  
<more>

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
)

type Dash struct {
	Debug    bool
	Paranoid bool
}

// used to control behavior of zip library code
var (
	Dashboard = new(Dash)
)

// A Reader provides sequential access to the contents of a zip archive.
// A zip archive consists of a sequence of files.
// The Next method advances to the next file in the archive (including the first),
// and then it can be treated as an io.Reader to access the file's data.
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

// ???  See PKWare APPNOTE.TXT for original header info
// describes one entry in zip archive, might be compressed or stored (ie. type 8 or 0 only)
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


func (h *Header) unpackLocalHeader(src [30]byte) os.Error {
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
	if Dashboard.Debug {
		sec := time.SecondsToUTC(h.Mtime)
		fmt.Printf("Header time parsed to : %s\n", sec.String())
	}
	return nil
}

// attach a zip reader interface to an open File object
func NewReader(r io.ReadSeeker) (*ZipReader, os.Error) {
	x := new(ZipReader)
	x.reader = r
	_, err := r.Seek(0, 0)
	if err != nil {
		fatal_err(err)
	}
	return x, nil
}

// grabs the next zip header from the file
// returns one header record for each stored file
//
func (r *ZipReader) Headers() []Header {
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
		if Dashboard.Debug {
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

// returns nil when no more data available
func (r *ZipReader) Next() (*Header, os.Error) {

	var localHdr [30]byte // size of header data fixed fields only
	n, err := r.reader.Read(&localHdr)
	if err != nil {
		fatal_err(err)
	}
	if n < 30 {
		fatal_err(ShortReadError)
	}
	if Dashboard.Debug {
		fmt.Printf("Read 30 bytes of header = %v\n", localHdr)
	}
	hdr := new(Header)
	hdr.Hreader = r.reader
	err = hdr.unpackLocalHeader(localHdr)
	if err != nil {
		return nil, err
	}
	if hdr.Size == -1 { // reached end of archive records, start of directory
		// not an error, returns nil to signal no more data
		return nil, nil
	}
	fileNameLen := sixteenBit(localHdr[26:28])
	// TODO read past end of archive without seeing Central Directory ? NOT POSSIBLE ?
	if fileNameLen == 0 {
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
	if Dashboard.Debug {
		fmt.Printf("filename archive: %s \n", fname)
	}
	hdr.Name = string(fname)
	// read extra data if present
	if Dashboard.Debug {
		fmt.Printf("reading extra data if present\n")
	}
	extraFieldLen := sixteenBit(localHdr[28:30])
	currentPos, err := r.reader.Seek(int64(extraFieldLen), 1)
	if err != nil {
		fatal_err(err)
	}
	// seek past compressed blob to start of next header
	hdr.Offset = currentPos // save this
	currentPos, err = r.reader.Seek(hdr.SizeCompr, 1)
	// side effect is to move r.reader pointer to start of next headers
	return hdr, nil
}

// Simple listing of header, same data should appear for the command "unzip -v file.zip"
// but with slightly different order and formatting
func (hdr *Header) Dump() {
	Mtime := time.SecondsToUTC(hdr.Mtime)
	fmt.Printf("%s: Size %d, Size Compressed %d, Type flag %d, LastMod %s, ComprMeth %d, Offset %d\n",
		hdr.Name, hdr.Size, hdr.SizeCompr, hdr.Typeflag, Mtime.String(), hdr.Compress, hdr.Offset)
}

// returns a Reader for the specified header
// it's users responsibility to compare actual vs stored crc if desired
// by setting zip.Dashboard.Paranoid = true
// see test suite for an example of how can easily be done if data will fit in RAM
// (Doable but it Gets trickier if it won't fit)
// Program will abort if file is larger than 2 gig and user is Paranoid
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
	if Dashboard.Debug {
		fmt.Printf("Header.Open() Read in %d bytes of compressed (deflated) data\n", n)
		h.Dump()
	}
	// got it in RAM, now need to expand it
	in := bytes.NewBuffer(comprData) // fill new buffer with compressed data
	inpt := flate.NewInflater(in)    // attach a reader to the buffer
	if err != nil {
		fatal_err(err)
	}
	if !Dashboard.Paranoid {
		return inpt, nil
	}

	if h.Size > TooBig {
		fatal_err(TooBigError)
	}
	// TODO ??? how do we insure eventual close of inpt
	// defer inpt.Close()        // make sure we eventually close the reader

	// if we're paranoid we want to crc32 the buffer and check vs stored crc32
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
	expdData := make([]byte, h.Size) // make the expanded buffer into a byte array
	n, err = b.Read(expdData)        // copy buffer into expdData
	if int64(n) < h.Size {
		fmt.Printf("copied %d, expected %d\n", n, h.Size)
		fatal_err(ShortReadError)
	}
	mycrc32 := crc32.ChecksumIEEE(expdData)
	if Dashboard.Debug {
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

	if Dashboard.Debug {
		fmt.Printf("year(%d) month(%d) day(%d) \n", year, month, day)
		fmt.Printf("hour(%d) minute(%d) second(%d)\n", hour, minute, second)
	}
	// TODO this checking is approximate for now, daysinmonth not checked fully
	// TODO we wont know file name at this point unless Debug is also true
	//  ? is that a problem or not ?
	if Dashboard.Paranoid {
		badDate := false
		badDate = badDate
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
			fmt.Fprintf(os.Stderr, "Encountered bad Mod Date: \n")
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

// convert from litte endian two byte slice to int16
func sixteenBit(n []byte) uint16 {
	if len(n) != 2 { /* TODO problem */
		fatal_err(Slice16Error)
	}
	var rc uint16
	rc = uint16(n[1])
	rc <<= 8
	rc |= uint16(n[0])
	return rc
}

// convert from litte endian four byte slice to int32
func thirtyTwoBit(n []byte) uint32 {
	if len(n) != 4 { /* TODO problem */
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
