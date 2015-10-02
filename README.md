`gingin` is a [fork](https://github.com/codegangsta/gin) of command line reloader for live reloading of Go web applications. I have changed the name from gin to differentiate from the excellent [Gin framework](https://github.com/gin-gonic/gin). Kudos to Codegangsta for the original, but I wanted some features and no one seemed to be ready to apply them.

## Installation

Assuming you have a working Go environment and `GOPATH/bin` is in your
`PATH`, `gingin` is a breeze to install:

```shell
go get github.com/coccodrillo/gingin
```

Then verify that `gingin` was installed correctly:

```shell
gingin -h
```

## Additional changes

I disliked the handling of the flags in original `gin` so I added arg runArgs,u which passes comma separated list of flags to the run command. This enables you to switch between production and development environments on the fly instead of changing your code or env variables:
 ```gingin -u="-env=development"``` thus passes "-env=development" to run command

I also added a command to exclude some folders since it helps with compilation times for larger projects - exclude,e takes in a comma separated list of folders to ignore when watching file changes

The original `gin` readme also said it adheres to the "silence is golden" principle, so it will only complain if there was a compiler error or if you successfully compile after an error. When having longer compile times and not getting an error it was difficult to figure out whether the change was already built. In this case, silence did not appear to be golden, so I added a message for every time when it reloads.