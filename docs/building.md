# Building Faunatone from source

On Linux, statically linked binaries have been built with the command:

```
go build -tags static -ldflags "-s -w" -o ftone ./faunatone/
```

On Windows under MSYS2, statically linked binaries have been built with the
command:

```
CGO_LDFLAGS="-static-libgcc -static-libstdc++ -Wl,-Bstatic -lstdc++ -lpthread -Wl,-Bdynamic -mwindows" go build -tags static -ldflags "-s -w -H windowsgui" ./faunatone/
```

You will need "dependencies".
