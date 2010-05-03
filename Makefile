GOCOMP = $(HOME)/bin/6g
GOLINK = $(HOME)/bin/6l
GOLIB =  $(HOME)/lib

ziptest: ziptest.go zip.6
	$(GOCOMP) ziptest.go
	$(GOLINK) -o ziptest -L . ziptest.6

zip.6: zip.go
	$(GOCOMP) zip.go
