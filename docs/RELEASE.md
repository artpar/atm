# Release

ATM releases are produced by GoReleaser from version tags.

## User Install Paths

Go users can install directly from the module:

```sh
go install github.com/artpar/atm@latest
```

Users can install the latest release archive with:

```sh
curl -fsSL https://github.com/artpar/atm/releases/latest/download/install.sh | sh
```

Users who do not have Go installed can download release archives from:

```text
https://github.com/artpar/atm/releases
```

Current release artifacts target:

- macOS amd64
- macOS arm64
- Linux amd64
- Linux arm64

Windows artifacts are intentionally not published yet because ATM's current
process discovery uses Unix-style process commands and filesystem behavior.

## Maintainer Flow

Run the local checks:

```sh
go test ./...
goreleaser check
goreleaser release --snapshot --clean
```

Create a release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The GitHub release workflow builds archives and checksums from the tag.

Linux `.deb`, `.rpm`, and `.apk` packages are also built by GoReleaser through
nFPM and attached to the GitHub release.
