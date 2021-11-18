# NexusCloner
Repository cloning tool for nexus

## Download
Check releases. There are windows, linux, macos binaries available for downloading.
Also you can download docker image from ghcr.io/MindHunter86/NexusCloner (github package registry). Read more in repository packages page.

## Build
Base build
```
go build -ldflags="-s -w" -o $GOPATH/bin/NexusCloner
```

Build application with syslog support (temporary marked as legacy)
```
go build -ldflags="-s -w" -tags syslog $GOPATH/bin/NexusCloner
```

After all building processes u may compress the application with [UPX](https://upx.github.io/)
```
upx -9 -k $GOPATH/bin/NexusCloner
```

## Usage examples
### One repository
Preperare credentials for source and destination repositories if u need:
```
cat <<-EOF | tee someFilePath.env
NCL_DST_USERNAME=login
NCL_SRC_PASSWORD=password
EOF
```

Start repository migrations:
```
. someFilePath.env ; ./NexusCloner --srcRepoUrl https://source.repository.com --srcRepoName repositoryname --dstRepoUrl https://destination.repository.com --dstRepoName repositoryname
```
or if you prefer docker (replace *#IMAGE LINK#* with your builded docker image)
```
docker run --env-file someFilePath.env #IMAGE LINK# --srcRepoUrl https://source.repository.com --srcRepoName repositoryname --dstRepoUrl https://destination.repository.com --dstRepoName repositoryname
```

### Two and more repositories
Preperare credentials for source and destination repositories if u need:
```
cat <<-EOF | tee someFilePath.env
NCL_DST_USERNAME=login
NCL_SRC_PASSWORD=password
EOF
```
Prepare file with repository names for migration
```
cat <<-EOF | tee someFilePath2.list
repoA
repoB
repo...
EOF
```
Start parallel migration (replace *#THREAD COUNT#* with some int value)
```
. someFilePath.env \
  && cat someFilePath2.list | xargs -n1 | xargs -ri -P #THREAD COUNT# ./NexusCloner -l warn --srcRepoUrl https://source.repository.com --srcRepoName {} --dstRepoUrl https://destination.repository.com --dstRepoName {}
```

## Testing:
There is no test files, sorry =(
There is only hardcore and debuging with printf() =)

## Usage page

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
