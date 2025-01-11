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
   fwatcher-dev - simple tool to run commands on filesystem change events

USAGE:
   fwatcher-dev [global options] <Command To Run>

GLOBAL OPTIONS:
   --debug                                                          (default: false)
   --command value, -c value                                        [command to run] (default: "echo hi")
   --watch value, -w value [ --watch value, -w value ]              [dir] (to watch) | -[dir] (to ignore) (default: ".")
   --ext value, -e value [ --ext value, -e value ]                  [ext] (to watch) | -[ext] (to ignore)
   --ignore-list value, -I value [ --ignore-list value, -I value ]  disables ignoring from default ignore list (default: ".git", ".svn", ".hg", ".idea", ".vscode", ".direnv", "node_modules", ".DS_Store", ".log")
   --cooldown value                                                 cooldown duration (default: "100ms")
   --interactive                                                    interactive mode, with stdin (default: false)
   --sse                                                            run watcher in sse mode (default: false)
   --sse-addr value                                                 run watcher in sse mode (default: ":12345")
   --help, -h                                                       show help
```

[See fwatcher in action](fwatcher_recording)

![fwatcher recording](https://github.com/nxtcoder17/fwatcher/assets/22402557/ce1b1908-cb9f-438f-85c1-3a8858265c40)
