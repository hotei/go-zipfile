GOCOMP = $(HOME)/bin/6g
GOLINK = $(HOME)/bin/6l


zipfile: zipfile.go
	$(GOCOMP) zipfile.go
	$(GOLINK) -o zipfile zipfile.6 

clean:
	rm -f zipfile *.core *~


