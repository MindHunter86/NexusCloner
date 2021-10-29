# NexusCloner
Repository cloning tool for nexus

```
NAME:
   NexusCloner - Repository cloning tool for nexus

USAGE:
   main [global options] command [command options] [arguments...]

VERSION:
   1.0

AUTHOR:
   Vadimka K. <admin@vkom.cc>

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --dstRepoName value          Destination repository name
   --dstRepoPassword value      Credentials for destination repository access [$NCL_DST_PASSWORD]
   --dstRepoUrl value           Destination repository url
   --dstRepoUsername value      Credentials for destination repository access [$NCL_DST_USERNAME]
   --http-client-insecure       disable TLS certificate verification
   --http-client-timeout value  internal HTTP client timeout (ms) (default: 5s)
   --loglevel value, -l value   log level (debug, info, warn, error, fatal, panic) (default: "info")
   --skip-download              Skip download after finding missing assets. Flag for debugging only.
   --skip-download-errors       Continue synchronization process if missing assets download detected
   --skip-upload                Skip upload after downloading missing assets. Flag for debugging only.
   --srcRepoName value          Source repository name
   --srcRepoPassword value      Credentials for source repository access [$NCL_SRC_PASSWORD]
   --srcRepoUrl value           Source repository url
   --srcRepoUsername value      Credentials for source repository access [$NCL_SRC_USERNAME]
   --syslog-proto value         (default: "tcp")
   --syslog-server value        DON'T FORGET ABOUT TLS\SSL, COMRADE
   --syslog-tag value           
   --temp-path-prefix value     Define prefix for temporary directory. If not defined, UNIX or WIN default will be used.
   --temp-path-save             Flag for saving temp path content before program close. Flag for debugging only.
   --help, -h                   show help
   --version, -v                print the version

COPYRIGHT:
   (c) 2021 mindhunter86
```