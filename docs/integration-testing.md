# Integration tests

## Prereqs

Install these tools:

1. [krew](khttps://krew.sigs.k8s.io/docs/user-guide/setup/install)
2. [kuttl](https://kuttl.dev/docs/#pre-requisites)

   Use krew to install. Run this command after krew installed: `kubectl krew install kuttl`

## Run local

1. Connect to an OpenShift.
1. Create the pull secret env file in the root of the folder, do not check this in. It is added to .gitignore as well so only method is to explicitly add it. Use production RHM to get a pull secret.

   ```sh
   echo "PULL_SECRET=${YOURPULLSECRET}" >> ./redhat-marketplace-pull-secret.env
   ```

1. Run the test command

   **NOTE** It will add/remove the deployment and run tests so save any desired states like secrets, etc.

   ```sh
   make test-ci-int
   ```

## Adding new tests

The tool is called kuttl and will install the operator and the resources necessary to work.

If you want to add new tests, you can create additional test files in the test/integration folder. Look at the examples in the folder for ideas.

**Note**: At the start of your tests all services for the operator would have been installed. You can check statuses or make small changes to the meterbase and inspect the behavior. Just remember this is a shared environment and one change could impact another test result.

## Adding more complicated tests

If we need to check specific conditions that may cause other modules to fail, you can create a new kuttl test in test/e2e. The main test is `test/e2e/register-test`. You can clone this one and make small changes. This will be expanded on with more tests for registration to test bad states.

## Testing using kind

There is a way to test everything using kind (a local kubernetes cluster). This is how our CI/CD works. You'll need to run these commands instead of the above and have a ~/.docker config accessible to create a pull secret.

```sh
make setup-kind test-ci-int-kind
```

Kind requires some things to be injected to work but for the most part works identical to the Openshift equivalent.
