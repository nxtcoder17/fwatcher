## fwatcher

fwatcher is a simple utility to monitor file changes and run some commands on those events.

### Installation

#### linux x86_64 (amd64)
```console
curl -L0 https://github.com/nxtcoder17/fwatcher/releases/latest/download/fwatcher-linux-amd64 > ./fwatcher
```
#### linux arm64 (aarch64)
```console
curl -L0 https://github.com/nxtcoder17/fwatcher/releases/latest/download/fwatcher-linux-arm64 > ./fwatcher
```

#### macos amd64 (amd64)
```console
curl -L0 https://github.com/nxtcoder17/fwatcher/releases/latest/download/fwatcher-darwin-amd64 > ./fwatcher
```

#### macos arm64 (aarch64)
```console
curl -L0 https://github.com/nxtcoder17/fwatcher/releases/latest/download/fwatcher-darwin-arm64 > ./fwatcher
```

### Usage

```console
NAME:
   fwatcher - watches files in directories and operates on their changes

USAGE:
   fwatcher [global options] command [command options] [arguments...]

VERSION:
   dev

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug                                                                              toggles showing debug logs (default: false)
   --command value, -c value                                                            specifies command to execute on file change
   --dir value, -d value                                                                directory to watch on (default: "/var/home/nxtcoder17/workspace/nxtcoder17/fwatcher")
   --ignore-suffixes value, -i value [ --ignore-suffixes value, -i value ]              files suffixes to ignore
   --only-watch-suffixes value, -w value [ --only-watch-suffixes value, -w value ]      files suffixes to watch
   --exclude-dir value, -x value, -e value [ --exclude-dir value, -x value, -e value ]  directory to exclude from watching
   --no-default-ignore, -I                                                              disables ignoring from default ignore list (default: false)
   --help, -h                                                                           show help
   --version, -v                                                                        print the version
```

[See fwatcher in action](fwatcher_recording)

![fwatcher recording](https://github.com/nxtcoder17/fwatcher/assets/22402557/ce1b1908-cb9f-438f-85c1-3a8858265c40)
