# Building Faunatone from source

On Linux, statically linked binaries have been built with the command:

```
go build -tags static -ldflags "-s -w" -o ftone ./faunatone/
```

On Windows under MSYS2 MINGW64, statically linked binaries have been built
with the command:

```
CGO_LDFLAGS="-static-libgcc -static-libstdc++ -Wl,-Bstatic -lstdc++ -lpthread -Wl,-Bdynamic -mwindows" go build -tags static -ldflags "-s -w -H windowsgui" ./faunatone/
```

[go-winres](https://github.com/tc-hib/go-winres) is used for generating the
.syso resource files for Windows, but you will not need to do this unless you
are changing them.

You will need "dependencies".
