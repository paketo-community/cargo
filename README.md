# `docker.io/paketocommunity/cargo`

The Rust Cargo Buildpack is a Cloud Native Buildpack that builds Rust applications using Cargo.

This buildpack is designed to work in collaboration with the [Rust Dist CNB](https://github.com/paketo-community/rust-dist) or [Rustup CNB](https://github.com/paketo-community/rustup) buildpacks which provide the actual Rust and Cargo binaries used by this buildpack.

## Behavior

If all of these conditions are met:

* `<APPLICATION_ROOT>/Cargo.toml` exists
* `<APPLICATION_ROOT>/Cargo.lock` exists

The buildpack will do the following:

* Requests that Rust and Cargo be installed
* If `$BP_CARGO_TINI_DISABLED` is false, `tini` is installed to the launch layer
* Uses `CARGO_HOME` to locate Cargo & tools
* Symlinks `<APPLICATION_ROOT/target>` to a cache layer, so that build artifacts are cached
* For each item in `$BP_CARGO_INSTALL_TOOLS`, `cargo install` is run and any `$BP_CARGO_INSTALL_TOOLS_ARGS` are included.
* Reads workspace members out of `Cargo.toml`
* For each workspace member, it executes `cargo install` to build and install binaries. Binaries are installed to a layer marked with `cache`
* All source code is removed from `/workspace`
* The application binaries are copied from the `cache` layer to `/workspace`
* Cleans `CARGO_HOME` as described [in the Cargo book](https://doc.rust-lang.org/cargo/guide/cargo-home.html#caching-the-cargo-home-in-ci)
* Reads binary targets from `Cargo.toml` and contributes process type for each target
  * Each process type launches the target using `tini` so that PID1 signal handling works out-of-the-box
  * If `$BP_CARGO_TINI_DISABLED` is set to true, `tini` will not be added to the process types

## Configuration

| Environment Variable           | Description                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `$BP_CARGO_INSTALL_ARGS`       | Additional arguments for `cargo install`. By default, `--locked`. The buildpack will also add `--color=never`, `--root=<destination layer>`, and `--path=<path-to-member>` for each workspace member. You cannot override those values. See more details below.                                                                                                                                        |
| `$BP_CARGO_WORKSPACE_MEMBERS`  | A comma delimited list of the workspace package names (this is the package name in the member's `Cargo.toml`, not what is in the workspace's `Cargo.toml`'s member list) to install. If the project is not using workspaces, this is not used. By default, for projects with a workspace, the buildpack will build all members in a workspace. See more details below.                                 |
| `$BP_STATIC_BINARY_TYPE`       | The type of static binary to build for tiny/static stacks. It defaults to a MUSLC static binary, but can be changed to a GNU LIBC based static binary. The two acceptable options are `muslc` and `gnulibc`.                                                                                                                                                                                           |
| `$BP_INCLUDE_FILES`            | Colon separated list of glob patterns to match source files. Any matched file will be retained in the final image. Defaults to `static/*:templates/*:public/*:html/*`.                                                                                                                                                                                                                                 |
| `$BP_EXCLUDE_FILES`            | Colon separated list of glob patterns to match source files. Any matched file will be specifically removed from the final image. If include patterns are also specified, then they are applied first and exclude patterns can be used to further reduce the fileset.                                                                                                                                   |
| `$BP_CARGO_TINI_DISABLED`      | Disable using `tini` to launch binary targets. Defaults to `false`, so `tini` is installed and used by default. Set to `true` and `tini` will not be installed or used.                                                                                                                                                                                                                                |
| `$BP_DISABLE_SBOM`             | Disable running the SBOM scanner. Defaults to `false`, so the scan runs. With larger projects this can take time and disabling the scan will speed up builds. You may want to disable this scane when building locally for a bit of a faster build, but you should not disable this in CI/CD pipelines or when you generate your production images.                                                    |
| `$BP_CARGO_INSTALL_TOOLS`      | Additional tools that should be installed by running `cargo install`. This should be a space separated list, and each item should contain the name of the tool to install like `cargo-bloat` or `diesel_cli`. Tools installed will be installed prior to compiling application source code and will be available on `$PATH` during build execution (but are not installed into the runtime container). |
| `$BP_CARGO_INSTALL_TOOLS_ARGS` | Any additional arguments to pass to `cargo install` when installing `$BP_CARGO_INSTALL_TOOLS`. The same list is passed through to every tool in the list. For example, `--no-default-features`.                                                                                                                                                                                                        |

### `BP_CARGO_INSTALL_ARGS`

Additional arguments for `cargo install`. This value defaults to `--locked`, and overriding it will completely override the defaults with what you set. Any additional arguments specified, are specified for each invocation of `cargo install`. The buildpack will execute `cargo install` once for each workspace member. If you're not using a workspace, then it executes a single time.

A few examples of what you can specify:

* `--path=./todo` to build a single member in a folder called `./todo` if you have a non-traditional folder structure
* `--bins` to build all binaries in your project
* `--bin=foo` to specifically build the foo binary when multiple binaries are present
* `-v` to get more verbose output from `cargo install`
* `--frozen` or `--locked` or customizing how Cargo will use the Cargo.lock file
* `--offline` for preventing Cargo from trying to access the Internet
* or any other valid arguments that can be passed to `cargo install`

You may **not** set `--color` and you may not set `--root`. These are fixed by the buildpack in order to make output look correct and to ensure that binaries are installed into the proper location.

### `BP_CARGO_WORKSPACE_MEMBERS`

This option may be used in conjunction with `BP_CARGO_INSTALL_ARGS`, however you may not set `--path` in `BP_CARGO_INSTALL_ARGS` when also setting `BP_CARGO_WORKSPACE_MEMBERS`, as the buildpack will control `--path` when building workspace members.

In summary:

* Use `BP_CARGO_INSTALL_ARGS` and `--path` to build one specific member of a workspace.
* Use `BP_CARGO_INSTALL_ARGS` to specify non-`--path` arguments to `cargo install`
* Use `BP_CARGO_WORKSPACE_MEMBERS` to specify one or more workspace members to build (using `BP_CARGO_WORKSPACE_MEMBERS` with only one member has identical behavior to `BP_CARGO_INSTALL_ARGS` and `--path`)
* Don't set either `BP_CARGO_INSTALL_ARGS` and `--path`, or `BP_CARGO_WORKSPACE_MEMBERS` and the buildpack will iterate through and build all of the members in workspace.

## Usage

In general, [you probably want the rust CNB instead](https://github.com/paketo-community/rust/#tldr). 

If you want to use this particular CNB directly, the easiest way is via image. Run `pack build -b paketo-community/cargo:<version> ...`.

## License

This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0
