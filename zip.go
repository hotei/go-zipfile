/*
   package zip docs

*/

package zip


import (
	//	"bytes"
	//	"compress/flate"
	//	"hash/crc32"

	"fmt"
	"io"
	"os"
	"time"
)

const DEBUG = true

const (
	MSDOS_EPOCH     = 1980
	ZIP_LocalHdrSig = "PK\003\004" // was 003004
	ZIP_CentDirSig  = "PK\001\002"
	ZIP_STORED      = 0
	ZIP_DEFLATED    = 8
)


var (
	ErrWriteTooLong    = os.NewError("write too long")
	ErrFieldTooLong    = os.NewError("header field too long")
	ErrWriteAfterClose = os.NewError("write after close")
)

var (
	InvalidSigError  os.Error = os.ErrorString("invalid Local Hdr Sig")
	InvalidCompError os.Error = os.ErrorString("invalid compression method")
	ShortReadError   os.Error = os.ErrorString("short read")
)


// A Reader provides sequential access to the contents of a zip archive.
// A zip archive consists of a sequence of files.
// The Next method advances to the next file in the archive (including the first),
// and then it can be treated as an io.Reader to access the file's data.
//
// Example:
//	zr := zip.NewReader(r)
//	for {
//		hdr, err := zr.Next()
//		if err != nil {
//			// handle error
//		}
//		if hdr == nil {
//			// end of zip archive
//			break
//		}
//		io.Copy(data, zr)
//	}
type Reader struct {
	current_file int
	reader       io.ReadSeeker
}

// describes one entry in zip archive, might be compressed or stored (ie. type 8 or 0 only)
type Header struct {
	Name      string
	Size      int64  // size while uncompressed
	SizeCompr int64  // size while compressed
	Typeflag  byte   // directory or regular file or ...
	Mtime     int64  // use 'go' version of time, not MSDOS version
	Compress  uint16 // only one method implemented and thats flate/deflate
	Offset    int64
	Crc32     uint32
}


func (h *Header) unpackLocalHeader(src [30]byte) os.Error {
	if string(src[0:4]) != ZIP_LocalHdrSig {
		if string(src[0:4]) == ZIP_CentDirSig { // reached last file, now into directory
			h.Size = -1 // signal last file reached
			return nil
		}
		return InvalidSigError
	}
	h.Compress = sixteenBit(src[8:10])

	if h.Compress != ZIP_STORED && h.Compress != ZIP_DEFLATED {
		return InvalidCompError
	}
	h.Size = int64(thirtyTwoBit(src[22:26]))
	h.SizeCompr = int64(thirtyTwoBit(src[18:22]))

	h.Crc32 = thirtyTwoBit(src[14:18])

	pktime := sixteenBit(src[10:12])
	pkdate := sixteenBit(src[12:14])
	h.Mtime = makeGoDate(pkdate, pktime)
	//	sec := time.SecondsToUTC(h.Mtime)
	//	fmt.Printf("Time parsed to : %s\n", sec.String() )
	return nil
}


func NewReader(r io.ReadSeeker) (*Reader, os.Error) {
	x := new(Reader)
	x.reader = r
	_, err := r.Seek(0, 0)
	if err != nil {
		fatal_err(err)
	}
	return x, nil
}

func ReaderAtSection(r io.ReaderAt, start, end int64) io.ReaderAt {
	return nil
}


func ReaderAtStream(r io.ReaderAt) io.Reader {
	return nil
}


func (r *Reader) Headers() []*Header {

	_, err := r.reader.Seek(0, 0)
	if err != nil {
		fatal_err(err)
	}
	n := 1
	for {
		hdr, err := r.Next()
		if err != nil {
			fatal_err(err)
		}
		if hdr == nil {
			break
		}
		fmt.Printf("Header[%d] filename %s\n", n, hdr.Name)
		n++
		Mtime := time.SecondsToUTC(hdr.Mtime)

		fmt.Printf("Size %d, Size Compressed %d, Type flag %d, LastMod %s, ComprMeth %d, Offset %d\n",
			hdr.Size, hdr.SizeCompr, hdr.Typeflag, Mtime.String(), hdr.Compress, hdr.Offset)
		// io.Copy(data, zr)	// copy data out of uncompressed buffer via zr
	}

	return nil
}

// returns ? for no more data?
func (r *Reader) Next() (*Header, os.Error) {

	var localHdr [30]byte // size of header data fixed fields only
	n, err := r.reader.Read(&localHdr)
	if err != nil || n != 30 {
		fatal_err(err)
	}
	fmt.Printf("Read 30 bytes of header = %v\n", localHdr)
	hdr := new(Header)
	err = hdr.unpackLocalHeader(localHdr)
	if err != nil {
		return nil, err
	}
	if hdr.Size == -1 { // reached end of archive records, start of directory
		return nil, nil
	}
	fileNameLen := sixteenBit(localHdr[26:28])
	if fileNameLen == 0 { // end of file reached
		return nil, nil
	}
	fname := make([]byte, fileNameLen)
	n, err = r.reader.Read(fname)
	if err != nil {
		fatal_err(err)
	}
	if n != int(fileNameLen) {
		fatal_err(ShortReadError)
	}
	fmt.Printf("filename archive: %s \n", fname)
	hdr.Name = string(fname)
	// read extra data if present
	fmt.Printf("reading extra data if present\n")
	extraFieldLen := sixteenBit(localHdr[28:30])
	currentPos, err := r.reader.Seek(int64(extraFieldLen), 1)
	if err != nil {
		fatal_err(err)
	}
	// read possibly compressed blob
	hdr.Offset = currentPos
	currentPos, err = r.reader.Seek(hdr.SizeCompr, 1)

	// optionally build array of headers here ???
	return hdr, nil
}


func fatal_err(erx os.Error) {
	fmt.Printf("%s \n", erx)
	os.Exit(1)
}


func (f *Header) Open() (io.Reader, os.Error) {
	return nil, nil
}


//	convert PKware date, time uint16s into seconds since Unix Epoch
func makeGoDate(d, t uint16) int64 {
	var year, month, day uint16
	year = d & 0xfe00
	year >>= 9
	//fmt.Printf("year = %d\n", year + MSDOS_EPOCH)

	month = d & 0x01e0
	month >>= 5
	//fmt.Printf("month = %d\n", month)

	day = d & 0x001f
	day = day

	var hour, minute, second uint16
	hour = t & 0xf800
	hour >>= 11
	fmt.Printf("hour = %d\n", hour)

	minute = t & 0x01e0
	minute >>= 5
	fmt.Printf("minute = %d\n", minute)

	second = (t & 0x001f) * 2
	second = second
	fmt.Printf("second = %d\n", second)

	ft := new(time.Time)
	ft.Year = int64(year) + MSDOS_EPOCH
	ft.Month = int(month)
	ft.Day = int(day)

	ft.Hour = int(hour)
	ft.Minute = int(minute)
	ft.Second = int(second)

	ft.ZoneOffset = 0
	ft.Zone = "UTC"

	return ft.Seconds()
}


// convert from litte endian two byte slice to int16
func sixteenBit(n []byte) uint16 {
	if len(n) != 2 { /* TODO problem */
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
