# NexusCloner
Repository cloning tool for nexus

## Building
Base building
```
go build -ldflags="-s -w" -o $GOPATH/bin/NexusCloner
```

Build application with syslog support (temporary marked as legacy)
```
go build -ldflags="-s -w" -tags syslog $GOPATH/bin/NexusCloner
```

After all building processes u may compress the application with UPX
```
upx -9 -k $GOPATH/bin/NexusCloner
```


## Common binary information

```
NAME:
   NexusCloner - Repository cloning tool for nexus

USAGE:
   NexusCloner [global options] command [command options] [arguments...]

VERSION:
   1.0

AUTHOR:
   Vadimka K. <admin@vkom.cc>

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --loglevel level, -l level    log level (debug, info, warn, error, fatal, panic) (default: "info")
   --http-client-timeout value   internal HTTP client timeout (ms) (default: 5s)
   --http-client-insecure        disable TLS certificate verification
   --temp-path-prefix directory  Define prefix for temporary directory. If not defined, UNIX or WIN default will be used.
   --temp-path-save              Flag for saving temp path content before program close. Flag for debugging only.
   --srcRepoName name            Source repository name
   --srcRepoUrl url              Source repository url
   --srcRepoUsername value       Credentials for source repository access [$NCL_SRC_USERNAME]
   --srcRepoPassword value       Credentials for source repository access [$NCL_SRC_PASSWORD]
   --dstRepoName name            Destination repository name
   --dstRepoUrl url              Destination repository url
   --dstRepoUsername value       Credentials for destination repository access [$NCL_DST_USERNAME]
   --dstRepoPassword value       Credentials for destination repository access [$NCL_DST_PASSWORD]
   --skip-download               Skip download after finding missing assets. Flag for debugging only.
   --skip-download-errors        Continue synchronization process if missing assets download detected
   --skip-upload                 Skip upload after downloading missing assets. Flag for debugging only.
   --help, -h                    show help
   --version, -v                 print the version

COPYRIGHT:
   (c) 2021 mindhunter86
```