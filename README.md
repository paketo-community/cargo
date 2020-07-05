# Rust Cargo Cloud Native Buildpack

The Rust Cargo Buildpack is a Cloud Native Buildpack V3 that build Rust applications.

This buildpack is designed to work in collaboration with the [Rust Dist CNB](https://github.com/dmikusa/rust-dist-cnb) buildpacks which provides the actual Rust and Cargo binaries used by this buildpack.

## Detection

The detection phase passes if both of the following conditions hold true:

- `<APPLICATION_ROOT>/Cargo.toml` exists
- `<APPLICATION_ROOT>/Cargo.lock` exists

## Integration

The Rust Cargo CNB will execute `cargo install`, which builds and installs your code into a layer that is available at runtime. The build will only happen if there are changes to `Cargo.lock` since the last build, otherwise the previous build is reused.

## Usage

In general, [you probably want the rust-cnb instead](https://github.com/dmikusa/rust-cnb/#tldr). If you want to use this particular CNB directly, the easiest way is via image.

Run `pack build -b dmikusa/rust-cargo:<version> ...`. Alternatively, follow the instructions in the next section to build locally and `pack build -b rust-cargo-cnb_xxxx` (folder generated by the build script). The latter is useful when developing.

## Building

To package this buildpack for consumption:

```bash
$ ./scripts/package.sh
```

This builds the buildpack's Go source using GOOS=linux by default. You can supply another value as the first argument to package.sh.