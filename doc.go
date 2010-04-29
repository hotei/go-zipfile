// doc.go

/*
Additional documentation for zipfile project

Initial goal was to convert python's zipfile.py into go.  Since then the
goal has changed a bit.  Rather than convert, I'm looking at a direct rewrite
based on PKWARE's APPNOTE.TXT.  APPNOTE.TXT describes the contents of zip files
from the perspective of the people who "own" the protocol.

One reason for backing off the "conversion" is the number of python modules that
would have to be converted before zipfile.py.  So far the dependency list is:
    zipfile.py
        import struct
        import shutil
        import binascii
        import cStringIO
        import zlib

        import time
        import stat
        import os
        import sys

    I believe the last 4 are similar enough to go libs that conversion should be 'trivial'
    Not so sure about 1-5.

    struct for starters seems non-trivial


*/


package documentation
