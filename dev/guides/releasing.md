This guide provides step by step guidance for cutting a new release of Nomad.

1. Bump the version in `version/version.go`
2. Run `make prerelease`
3. Commit any changed, generated files.
4. On the Linux Vagrant run `make release`
5. `mv pkg/ pkg2/`. We move the compiled packages such that when we run step 6,
   the binaries are deleted.
6. On a Mac, run `make release`
7. `mv pkg2/* pkg/`. This moves all the binaries into the same folder and thus
   we are ready for releasing.
8. Ensure you have the hc-release environment variables set for being able to
   read/write to S3. This can be retrieved from 1password.
9. `./scripts/dist.sh <version>`. Formating of <version> is 0.x.x(-|rcx|betaX)
10. Bump the version to the new dev version.
11. Remove the generated files and commit

# Only on final releases

1. Add the new version to checkpoint.

# Modifying the website

Assuming master is the branch you want the website to reflect

1. On master, bump the version in `website/config.rb`
2. Delete the remote stable-website branch (`git push -d origin stable-website`)
3. Create the new stable website, `git checkout -b stable-website`
4. `git push origin stable-website`
5. In Slack run, `hashibot deploy nomad`
