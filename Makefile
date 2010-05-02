GOCOMP = $(HOME)/bin/6g
GOLINK = $(HOME)/bin/6l
GOLIB =  $(HOME)/lib

zipfile: zipfile.go zip.6
	$(GOCOMP) zipfile.go
	$(GOLINK) -o zipfile -L . zipfile.6

zip.6: zip.go
	$(GOCOMP) zip.go
