# `docker.io/paketocommunity/cargo`

The Rust Cargo Buildpack is a Cloud Native Buildpack that builds Rust applications using Cargo.

This buildpack is designed to work in collaboration with the [Rust Dist CNB](https://github.com/paketo-community/rust-dist) or [Rustup CNB](https://github.com/paketo-community/rustup) buildpacks which provide the actual Rust and Cargo binaries used by this buildpack.

## Behavior

If all of these conditions are met:

* `<APPLICATION_ROOT>/Cargo.toml` exists
* `<APPLICATION_ROOT>/Cargo.lock` exists

The buildpack will do the following:

* Requests that Rust and Cargo be installed
* Uses `CARGO_HOME` to locate Cargo & tools
* Symlinks `<APPLICATION_ROOT/target>` to a cache layer, so that build artifacts are cached
* Reads workspace members out of `Cargo.toml`
* For each workspace member, it executes `cargo install` to build and install binaries. Binaries are installed to a layer marked with `cache`
* All source code is removed from `/workspace`
* The application binary is copied from the `cache` layer to `/workspace`
* Cleans `CARGO_HOME` as described [in the Cargo book](https://doc.rust-lang.org/cargo/guide/cargo-home.html#caching-the-cargo-home-in-ci)

## Configuration

| Environment Variable          | Description                                                                                                                                                                                                                                                                                                                                                            |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `$BP_CARGO_INSTALL_ARGS`      | Additional arguments for `cargo install`. By default, the buildpack will run `cargo install --color=never --root=<destination layer> --path=<path-to-member>` for each workspace member. See more details below.                                                                                                                                                       |
| `$BP_CARGO_WORKSPACE_MEMBERS` | A comma delimited list of the workspace package names (this is the package name in the member's `Cargo.toml`, not what is in the workspace's `Cargo.toml`'s member list) to install. If the project is not using workspaces, this is not used. By default, for projects with a workspace, the buildpack will build all members in a workspace. See more details below. |
| `$BP_CARGO_EXCLUDE_FOLDERS`   | A comma delimited list of the top-level folders that should not be deleted, which means they will persist through to the generated image. This *only* applies to top level folders. You only need the folder name, not a full path.                                                                                                                                    |

### `BP_CARGO_INSTALL_ARGS`

Additional arguments for `cargo install`. Any additional arguments specified, are specified for each invocation of `cargo install`. The buildpack will execute `cargo install` once for each workspace member. If you're not using a workspace, then it executes a single time.

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
