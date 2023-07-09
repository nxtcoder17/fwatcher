### fwatcher

fwatcher is a simple golang CLI tool to monitor file changes and run some commands on them.
It is intended to work like `nodemon`.

```console
NAME:
   fwatcher - watches files in directories and operates on their changes

USAGE:
   fwatcher [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug                                                                              toggles showing debug logs (default: false)
   --exec value                                                                         specifies command to execute on file change
   --dir value, -d value                                                                directory to watch on (default: "$PWD")
   --extensions value, --ext value [ --extensions value, --ext value ]                  file extensions to watch on
   --ignore-extensions value, --iext value [ --ignore-extensions value, --iext value ]  file extensions to ignore watching on
   --exclude-dir value, --exclude value [ --exclude-dir value, --exclude value ]        directory to exclude from watching
   --help, -h                                                                           show help
```

![fwatcher recording](https://github.com/nxtcoder17/fwatcher/assets/22402557/ce1b1908-cb9f-438f-85c1-3a8858265c40)
