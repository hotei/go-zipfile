// doc.go

// Copyright 2009-2010 David Rook. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// source can be found at http://www.github.com/hotei/go-zipfile
//
// <David Rook> ravenstone13@cox.net  AKA Hotei on golang.org and github
// This is a work-in-progress
//     This version does only zip reading, no zip writing yet

/*
This file contains additional documentation for zip library project

Very IMPORTANT: take a look at the read_test.go example for examples of how the
library can be used.

Goal is to allow go programs to read from 'zip' archive files.

BACKGROUND:

Zip was born in the days when diskettes held less than 1 MB and it was common to
use several diskettes or "volumes" to create an archive of a larger disk directory.
In multi-volume cases a central directory was stored on the last diskette so that
you wouldn't have to handle every diskette in a 20 volume set to find which diskette
held a specific file.  The restore program would read the last diskette and then
prompt you to insert the next appropriate diskette.

In our case it doesn't matter, because this version doesn't know or care
if it's multi-volume. This version of the zip library does NOT
look at the central header areas at
the end of the zip archive.  Instead it builds headers on the fly by reading the
actual archived data. Additionally reading the actual data may be useful to validate
the readability of older removeable media like 5.25 inch diskettes and early CDs.

The initial approach was to convert python's zipfile.py into go.  Since then the
method has changed a bit.  Rather than convert, this libarary was writen from scratch
based on PKWARE's APPNOTE.TXT.  APPNOTE.TXT describes the contents of zip files
from the perspective of the company who designed the zip protocol.  The resulting zip.go
library is ready for beta-testing and passes the initial test suite.

So far all testing has been on zip files smaller than 20 megabytes.

REFERENCES:

   http://www.pkware.com/documents/casestudies/APPNOTE.TXT

LIMITATIONS:

Probably most significant is that this is a read-only library at present.

At present there is a limitation of 2GB on expanded
files if you set paranoid mode - ie if you want CRC32 checking done after
expansion (which normally you would).
This is a limitation currently imposed by how I use the IEEECRC32 function,
and it will probably be lifted in the near future.
With paranoid mode off you should be able to read files over 2GB now.

Older versions of zip only supported a max 4 of GB file sizes but later
zip versions expanded that to "big enough" (64 bits).  Older versions also limited the number
of files in an archive to 16 bits (65536 files) but newer versions have upped
that number to "big enough" also.

Paranoid mode will also abort if it encounters an invalid date, like month 13
or a modification date that's in the future compared to the time.Now()
when the program is run.

Paranoid mode can be turned off by setting zip.Dashboard.Paranoid = false in
your program.  One reason for a paranoid mode is that in the MSDOS/MSWindows
world a lot of virus programs messed with
dates to purposely screw up your backup and restore programs.  With paranoid =
false you'll still see a warning to STDERR about the problems encountered, but
it will not abort.

Paranoid mode may cause smaller systems to run out of memory as the Open() function
currently pulls the contents of the zip archive entry into memory to uncompress and
run the IEEEcrc32().  This obviously depends on the size of the files you're
working with as well as your system's capacity. This behavior is on the TODO list
for improvement to reduce memory footprint.

There may be an opportunity to do some additional checking
in paranoid mode by comparing the actual headers with the ones stored in the
Central Directorys.  That's also on the "TODO" list.

*/
package documentation
