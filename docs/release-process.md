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

### Hotfix

## Manual Releases

### Prerequesites

1. [operator-sdk](https://sdk.operatorframework.io/docs/install-operator-sdk/)
1. [operator-courier](https://github.com/operator-framework/operator-courier)
1. [gitflow](https://github.com/nvie/gitflow)

### Release or Bugfix

1. Start from 'develop' branch.
```
git checkout develop
git pull
```
1. Run
```shell
git flow release start $(make current-version)
git flow release publish
```

A new branch called release/x.x.x will be made for you and pushed to the repository.

1. Generate the csv files and commit them. The release branch should be used to create manifests for the beta channel. Updates to the bundle in Partner connect will only impact beta.

```
make generate-csv generate-csv-manifest
git add ./deploy/olm-catalog
git commit -m "chore: updating OLM manifests"
git push
```

1. Operator images should be built and pushed for you with the build. You will need to publish the images in partner connect for them to be used.

1. Upload your bundle to partner connect. And publish it when it passes.

1. Once the release is finished. Submit a PR to merge to master. Master build will deploy images, make the final bundle for upload to update stable.



### Hotfix
