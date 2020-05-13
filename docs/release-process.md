# Release Processes

The Red Hat Marketplace operator uses a branch model called git-flow for release management.

You can read more about git flow [here](). Here are the steps to release the operator. The steps are listed here for manual release.

Branches:

| branch  |  use  |
|:--|:--|
| master | Master branch is current stable version.  |
| develop | Develop branch is where all future work is branched from and merged to. |
| feature/* | Feature branches for future work. Start off develop. |
| bugfix/* | Bug fixes for the next release. Lower priority. Branch from develop. |
| hotfix/* | Hot fix for production code. High priority. Branch from master. |
| release/* | Release branches for the next release. Hotfix is also a release branch. Can only be one. Branch from develop. |


When enough features are ready, a release branch is created off of develop. Release branch is worked until all testing is completed as a beta release. Once the beta release is stable, the release is merged into master. Merge to master creates the next official stable release.

Hotfixes are started off of master. Bugfixes off of stable.

## Automatic Releases

### Release

### Release

### Hotfix

## Manual Releases

### Release

### Hotfix
