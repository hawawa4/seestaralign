---
name: release
description: Cut a release of seestaralign by pushing a semver git tag (vX.Y.Z), which makes the docker workflow publish the image to ghcr.io. Use when the user says "push a new release", "release X.Y.Z", "cut a release", "bump the version", or similar.
---

# Release

In this repo, "push a new release" means **push an annotated `vX.Y.Z` tag** on
`main`. The tag triggers `.github/workflows/docker.yml`, which runs the tests
and publishes `ghcr.io/hawawa4/seestaralign` as `:X.Y.Z`, `:X.Y`, `:X` and
`:latest`. There is no version file to bump and no GitHub Release to create.

## Steps

1. **Pick the version.**
   - If the user gave one ("release 1.2.0"), use it. Strip a leading `v`.
   - If they said "major" / "minor" / "patch", bump the latest tag
     (`git tag --list 'v*' --sort=-v:refname | head -1`) accordingly.
   - If they gave neither, propose the next one: a patch bump if the commits
     since the last tag are only fixes, otherwise a minor bump. Show it with
     the commit list and confirm before tagging.
   - It must be new: `git rev-parse -q --verify refs/tags/vX.Y.Z` must fail.

2. **Check the tree.**
   - `git fetch --tags origin`, be on `main`, and make sure `main` is not
     behind `origin/main`.
   - If there are uncommitted changes, ask whether to commit them into this
     release or leave them out. Never silently include or discard them.
   - Push `main` first if it is ahead of `origin/main`, so the tag points at a
     commit that exists on the remote.

3. **Test.** Run `just test` (takes ~30 s). Do not tag if it fails.

4. **Tag and push.**
   ```sh
   git tag -a vX.Y.Z -m "seestaralign X.Y.Z" -m "<one line per notable change since the last tag>"
   git push origin vX.Y.Z
   ```
   Build the message body from `git log --oneline <last-tag>..HEAD`
   (or the full history for the first release).

5. **Report.** Give the tag, the commit it points at, and the Actions page
   (`https://github.com/hawawa4/seestaralign/actions`). If `gh` is available,
   `gh run list --workflow docker.yml --limit 1` shows the triggered run.

Never move or delete an existing tag that has been pushed. If a release is
broken, fix forward with a patch release.
