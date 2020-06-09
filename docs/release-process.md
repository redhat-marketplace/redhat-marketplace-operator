# Release Processes

<!-- markdown-toc start - Don't edit this section. Run M-x markdown-toc-refresh-toc -->
**Table of Contents**

- [Release Processes](#release-processes)
    - [Branches](#branches)
    - [Automatic Releases](#automatic-releases)
        - [Release](#release)
        - [Hotfix](#hotfix)
    - [Manual Releases](#manual-releases)
        - [Prerequesites](#prerequesites)
        - [Release or Bugfix](#release-or-bugfix)
        - [Hotfix](#hotfix-1)
    - [Updating Partner Connect](#updating-partner-connect)

<!-- markdown-toc end -->

*Prereq:* [install git flow extension](https://github.com/petervanderdoes/gitflow-avh/wiki/Installing-on-Mac-OS-X)

The Red Hat Marketplace operator uses a branch model called git-flow for release management.

You can read more about git flow [here](https://nvie.com/posts/a-successful-git-branching-model/) and there is a handy cheat sheet [here](https://danielkummer.github.io/git-flow-cheatsheet/index.html). Please download and [install git flow plugin as well](https://github.com/nvie/gitflow). Here are the steps to release the operator. The steps are listed here for manual release.

## Branches

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

1. Start from 'develop' branch.
  ```sh
  git checkout develop
  git pull
  ```
1. Run
  ```sh
  git flow release start $(make current-version)
  git flow release publish
  ```

  A new branch called release/x.x.x will be made for you and pushed to the repository.

1. Operator images should be built and pushed for you with the build. Additionally there will be a github check that will create assets for you. You will need to publish the images in partner connect.

1. Upload your generated bundle to [partner connect](#updating-partner-connect). And publish it when it passes.

1. Once the release is finished. Submit a PR to merge to master. Master build will deploy images, make the final bundle for upload to update stable.

1. Submit PR to merge master back into develop. Approve

### Hotfix

Same as release, but change git flow commands to `git flow hotfix`

## Manual Releases

### Prerequesites

1. [operator-sdk](https://sdk.operatorframework.io/docs/install-operator-sdk/)
1. [operator-courier](https://github.com/operator-framework/operator-courier)
1. [gitflow](https://github.com/nvie/gitflow)

### Release or Bugfix

*Warning*: to do these steps you need pull/push access to master/develop branch.

1. Start from 'develop' branch.
  ```sh
  git checkout develop
  git pull
  ```
1. Run
  ```sh
  git flow release start $(make current-version)
  git flow release publish
  ```

A new branch called release/x.x.x will be made for you and pushed to the repository.

1. Generate the csv files and commit them. The release branch should be used to create manifests for the beta channel. Updates to the bundle in Partner connect will only impact beta.

  ```sh
  make generate-csv generate-csv-manifest
  git add ./deploy/olm-catalog
  git commit -m "chore: updating OLM manifests"
  git push
```

1. Operator images should be built and pushed for you with the build. You will need to publish the images in partner connect for them to be used.

1. Upload your bundle to [partner connect](#updating-partner-connect). And publish it when it passes.

1. Once the release is finished. Run these commands:

  ```sh
  git flow release finish $(make current-version)
  ```


### Hotfix

Same as release, but change git flow commands to `git flow hotfix`


## Updating Partner Connect

Before releasing, we need to do a few steps in partner connect.

1. Go into [Partner Connect](https://connect.redhat.com)
1. Navigate to the "Red Hat Marketplace Operator Image" project.
1. Find the tag associated to the release and publish it.
  ![Publish Image](./images/pc-publish-image.png)
1. Download the bundle from the [Generate Bundle](https://github.com/redhat-marketplace/redhat-marketplace-operator/actions?query=workflow%3A%22Generate+Bundle%22) action.
  ![alt text](./images/github-action-generate-bundle.png)
  * The bundle with `stable-` is for updating the stable channel. This is the one you will primarily use.
  * The bundle starting with `beta-` is for updating the beta channel. This is currently broke.
  * Also notice the bundle describes what it's deploying `bundle-s0.1.2-b0.1.2` says stable is version 0.1.2 and beta is 0.1.2. This helps verify you're not making mistake.
1. Then navigate to the "Red Hat Marketplace Operator" project.
1. Once inside the operator project, selet `Operator Config`.
1. Now upload the zip file included in the bundle you downloaded from the action download.
  ![Publish Image](./images/pc-publish-operator.png)
1. You'll need to make sure the image is published and all the scorecard tests pass.
1. If all the tests are passed and you're read, select publish and your new version of the operator will be released.
