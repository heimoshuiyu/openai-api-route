linux:
	go build -v -ldflags '-linkmode=external -extldflags=-static' -tags sqlite_omit_load_extension,netgo