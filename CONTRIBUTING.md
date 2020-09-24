## Contributing to yay

Contributors are always welcome!

If you plan to make any large changes or changes that may not be 100% agreed
on, we suggest opening an issue detailing your ideas first.

Otherwise send us a pull request and we will be happy to review it.

### Dependencies

Yay depends on:

- go (make only)
- git
- base-devel
- pacman

Note: Yay also depends on a few other projects, these are pulled as go modules.

### Building

Run `make` to build Yay. This command will generate a binary called `yay` in
the same directory as the Makefile.

#### Docker Release

`make docker-release` will build the release packages for `aarch64` and for `x86_64`.

For `aarch64` to run on a `x86_64` platform `qemu-user-static(-bin)` must be
installed.

```
docker run --rm --privileged multiarch/qemu-user-static:register --reset
```

will register QEMU in the build agent. ARM builds tend to crash sometimes but
repeated runs tend to succeed.

### Code Style

All code should be formatted through `go fmt`. This tool will automatically
format code for you. We recommend, however, that you write code in the proper
style and use `go fmt` only to catch mistakes.

Use [pre-commit](https://pre-commit.com/) to validate your commits against the various
linters configured for this repository.

### Testing

Run `make test` to test Yay. This command will verify that the code is
formatted correctly, run the code through `go vet`, and run unit tests.
