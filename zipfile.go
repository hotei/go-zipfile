// zipfile.go

// Copyright 2009-2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


package main

import (
	"flag"
	"fmt"
	"os"
)

const DEBUG = true

// The "end of central directory" structure, magic number, size, and indices
// (section V.I in the format document)
const (
	structEndArchive = "<4s4H2LH"
	stringEndArchive = "PK\005\006"
	//	sizeEndCentDir = struct.calcsize(structEndArchive)
	sizeEndCentDir = 22 // so says python 2.6.2 interp
)

/*
  I.  End of central directory record:

        end of central dir signature    4 bytes  (0x06054b50)
        number of this disk             2 bytes
        number of the disk with the
        start of the central directory  2 bytes
        total number of entries in the
        central directory on this disk  2 bytes
        total number of entries in
        the central directory           2 bytes
        size of the central directory   4 bytes
        offset of start of central
        directory with respect to
        the starting disk number        4 bytes
        .ZIP file comment length        2 bytes
        .ZIP file comment       (variable size)
*/
type CentDirRec struct {
	CentDirSig        [4]byte
	ThisDiskNumber    int16
	CentDirDiskNumber int16
	ThisDiskDirEnts   int16
	CentDirDirEnts    int16
	CentDirTotalSize  int32
	CentDirOffset     int32
	CommentLength     int16
	Comment           []byte
}


// Quickly see if file is a ZIP file by checking the magic number
func is_zipfile(filename string) bool {
	if DEBUG {
		fmt.Printf("Testing is_zipfile( %s )\n", filename)
	}
	fpin, err := os.Open(filename, os.O_RDONLY, 0666)
	// err could be file not found, file found but not accessible etc
	// in any case test fails
	if err != nil {
		return false
	}
	defer fpin.Close()
	endrec, err := _EndRecData(fpin) // returns a ZipDir record
	if err != nil {
		return false
	}
	if endrec != nil {
		return true
	} // file readable and has correct magic number
	return false
}

func _EndRecData(fpin *os.File) (*CentDirRec, os.Error) {
	/*
		Return data from the "End of Central Directory" record, or nil.

	    The data is a list of the nine items in the ZIP "End of central dir"
	    record followed by a tenth item, the file seek offset of this record.
	*/
	filesize, err := fpin.Seek(0, 2)
	if DEBUG {
		fmt.Printf("filesize = %d\n", filesize)
	}

	_, err = fpin.Seek(-sizeEndCentDir, 2)
	if err != nil {
		return nil, err
	}
	var n int
	var data [sizeEndCentDir]byte
	n, err = fpin.Read(&data)
	fmt.Printf("data = %v\n", data)
	if n != sizeEndCentDir {
		return nil, os.EINVAL
	}
	cdr := new(CentDirRec)

	if (string(data[0:4]) == stringEndArchive) && (string(data[sizeEndCentDir-2:]) == "\000\000") {
		if DEBUG {
			fmt.Printf("looks like a good zipfile\n")
		}
		cdr = cdr       // [TODO]need struct.unpack() here
	} else {
		fmt.Printf("bad magic or EndCentDir in zipfile\n")
		fmt.Printf("magic = %v, end = %v\n", data[0:4], data[sizeEndCentDir-2:])
	}
	return cdr, nil
}

func main() {
	fmt.Printf("hello\n")
	flag.Parse()
	fmt.Printf("Flag got %d args on cmd line after command name\n", flag.NArg())
	if flag.NArg() == 0 { // do nothing
	} else { // whitespace sensitive
		for i := 0; i < flag.NArg(); i++ {
			fmt.Printf("%d %s\n", i, flag.Arg(i))
			is_zipfile(flag.Arg(i))
		}
	}
}
/*
def _EndRecData(fpin):
    """Return data from the "End of Central Directory" record, or None.

    The data is a list of the nine items in the ZIP "End of central dir"
    record followed by a tenth item, the file seek offset of this record."""

    # Determine file size
    fpin.seek(0, 2)
    filesize = fpin.tell()

    # Check to see if this is ZIP file with no archive comment (the
    # "end of central directory" structure should be the last item in the
    # file if this is the case).
    try:
        fpin.seek(-sizeEndCentDir, 2)
    except IOError:
        return None
    data = fpin.read()		 // read to end of file
    if data[0:4] == stringEndArchive and data[-2:] == "\000\000":
        # the signature is correct and there's no comment, unpack structure
        endrec = struct.unpack(structEndArchive, data)
        endrec=list(endrec)

        # Append a blank comment and record start offset
        endrec.append("")
        endrec.append(filesize - sizeEndCentDir)

        # Try to read the "Zip64 end of central directory" structure
        return _EndRecData64(fpin, -sizeEndCentDir, endrec)

    # Either this is not a ZIP file, or it is a ZIP file with an archive
    # comment.  Search the end of the file for the "end of central directory"
    # record signature. The comment is the last item in the ZIP file and may be
    # up to 64K long.  It is assumed that the "end of central directory" magic
    # number does not appear in the comment.
    maxCommentStart = max(filesize - (1 << 16) - sizeEndCentDir, 0)
    fpin.seek(maxCommentStart, 0)
    data = fpin.read()
    start = data.rfind(stringEndArchive)
    if start >= 0:
        # found the magic number; attempt to unpack and interpret
        recData = data[start:start+sizeEndCentDir]
        endrec = list(struct.unpack(structEndArchive, recData))
        comment = data[start+sizeEndCentDir:]
        # check that comment length is correct
        if endrec[_ECD_COMMENT_SIZE] == len(comment):
            # Append the archive comment and start offset
            endrec.append(comment)
            endrec.append(maxCommentStart + start)

            # Try to read the "Zip64 end of central directory" structure
            return _EndRecData64(fpin, maxCommentStart + start - filesize,
                                 endrec)

    # Unable to find a valid end of central directory structure
    return
*/
