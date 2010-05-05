include $(GOROOT)/src/Make.$(GOARCH)

TARG=zip

GOFILES=\
	zip.go

include $(GOROOT)/src/Make.pkg

