# NexusCloner
Repository cloning tool for nexus

## Table of Contents:
- [Download](#download)
- [Build](#build)
- [Usage examples](#usage-examples)
   - [One repository](#one-repository)
   - [Two and more repositories](#two-and-more-repositories)
   - [Path filtering](#path-filtering)
- [Testing](#testing)
- [Usage page](#usage-page)

----

## Download
Check releases. There are windows, linux, macos binaries available for downloading.  
Also you can download docker image from ghcr.io/MindHunter86/NexusCloner (github package registry). Read more in repository packages page.

## Build
Base build
```
go build -ldflags="-s -w" -o $GOPATH/bin/NexusCloner
```

After all building processes u may compress the application with [UPX](https://upx.github.io/)
```
upx -9 -k $GOPATH/bin/NexusCloner
```

## Usage examples
### One repository
Simple task - clone *reponame* from *nexus1.example.com* to *nexus2.example.com*:
```
./NexusCloner https://nexus1.example.com/reponame https://nexus2.example.com/reponame
```

If your repositories require authentication, you can use user:pass data in URL format:
```
./NexusCloner https://username:password@nexus1.example.com/reponame https://username:password@nexus2.example.com/reponame
```

If your repositories have selfsign certificate, please, use parameter **--http-client-insecure**
  
If your repositories are slow, or you have big files, that requires long downloading use **--http-client-timeout**.


### Two and more repositories
For the first, you need to prepare file with repository names for migration.
  
For example:
```
cat <<-EOF | tee repositories.txt
repository_name1
repository_name2
repository_name3
repository_name4
repository_name5
EOF
```

After that, you can use these small hack for multi-thread sync:
```
cat repositories.txt | xargs -n1 | xargs -ri -P4 https://nexus1.example.com/{} https://nexus2.example.com/{}
```
  
**-P4** in example above is *thread count*. Modify this argument if you need.


### Path filtering
Sometimes you need clone repository particularly. There is **--path-filter** for this tasks. The variable is requires valid regexp for further filtering.
  
Example. You have some repository with tree of artifacts:
```
com/example/internal/artifact1
com/example/internal/artifact2
com/example/public/artifact1
com/example/public/artifact2
com/example/public/artifact3
com/example/public/other1
com/example/public/other2
com/example/public/other3
```

And you need sync *ONLY* public data. Your command will be:
```
./NexusCloner --path-filter "com/example/public" https://nexus1.example.com/reponame https://nexus2.example.com/reponame
```

Or you need sync *ONLY* artifacts from public path:
```
./NexusCloner --path-filter "com/example/public/artifacts.*" https://nexus1.example.com/reponame https://nexus2.example.com/reponame
```
  
With regexp you can do any magic.


## Testing
There is no test files, sorry =(

## Usage page

```
NAME:
   NexusCloner - Repository cloning tool for nexus

USAGE:
   main [global options] command [command options] [arguments...]

AUTHOR:
   Vadimka K. <admin@vkom.cc>

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --verbose LEVEL, -v LEVEL      Verbose LEVEL (value from 5(debug) to 0(panic) and -1 for log disabling(quite mode)) (default: 4)
   --quite, -q                    Flag is equivalent to verbose -1
   --http-client-timeout TIMEOUT  Internal HTTP client connection TIMEOUT (format: 1000ms, 1s) (default: 10s)
   --http-client-insecure         Flag for TLS certificate verification disabling
   --temp-path-prefix directory   Define prefix for temporary directory. If not defined, UNIX or WIN default will be used.
   --temp-path-save               Flag for saving temp path content before program close. Flag for debugging only.
   --skip-download                Skip download after finding missing assets. Flag for debugging only.
   --skip-download-errors         Continue synchronization process if missing assets download detected
   --skip-upload                  Skip upload after downloading missing assets. Flag for debugging only.
   --path-filter path             Regexp value with path for syncing. (default: ".*")
   --help, -h                     show help
   --version, -V                  print the version

COPYRIGHT:
   (c) 2021 mindhunter86
```
