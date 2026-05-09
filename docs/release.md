# Release Guide

This guide records the routine test and release workflow for Enno.

## Daily Verification

After code changes, run:

```sh
make verify
```

This executes:

```sh
gofmt -w .
go mod tidy
go test ./...
```

While the CLI still lives in this repository, use `make cli-verify` when changes
touch `cmd/enno` or CLI-owned internal packages; it runs `make verify` and then
`go install ./cmd/enno`.

Use narrower commands while iterating:

```sh
make fmt
make tidy
make test
make install
make cli-verify
```

To see all available commands:

```sh
make help
```

## Version Files

Enno uses Semantic Versioning.

- `VERSION` stores the current release version without a leading `v`, for example `0.1.0`.
- `CHANGELOG.md` records release notes.
- Git tags use a leading `v`, for example `v0.1.0`.

Until the CLI is split into a standalone repository, this repository keeps one
shared `CHANGELOG.md` for SDK and in-repo CLI changes. After the split, the SDK
repository should track SDK/provider/tool changes here, while the CLI repository
maintains its own changelog and release workflow.

For a release, update these together:

1. Set the new version in `VERSION`.
2. Add a matching section in `CHANGELOG.md`, for example `## [0.2.0] - YYYY-MM-DD`.
3. Update changelog compare links at the bottom.

## Release Checklist

1. Confirm working tree changes are intentional:

```sh
git status
```

2. Run full verification:

```sh
make verify
```

3. Run release checks:

```sh
make release-check
```

4. Commit the release changes:

```sh
git add VERSION CHANGELOG.md
git commit -m "Prepare v$(cat VERSION)"
```

5. Create the release tag:

```sh
make tag
```

6. Push commits and tag:

```sh
git push origin main
git push origin v$(cat VERSION)
```

Pushing the tag triggers `.github/workflows/release.yml`, which runs `make verify` and creates a GitHub Release.

## Installing a Released Version

CLI:

```sh
go install github.com/dean2021/enno/cmd/enno@latest
```

Library:

```sh
go get github.com/dean2021/enno@latest
```

## Troubleshooting

If `make release-check` fails:

- Ensure `VERSION` is not empty.
- Ensure `VERSION` does not include a leading `v`.
- Ensure `CHANGELOG.md` has a matching `## [x.y.z]` section.
- Run `make verify` and fix any test failure. Run `make cli-verify` as well when the release includes in-repo CLI changes.

If the GitHub Release workflow does not run:

- Confirm the pushed tag matches `v*.*.*`.
- Confirm the tag was pushed to `origin`.
- Check the Actions tab in GitHub for workflow errors.
