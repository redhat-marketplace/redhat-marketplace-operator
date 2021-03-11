# RHM Report Tool

The reporter tool can be ran locally for testing and quick debugging.

## Running the tool

1. Build the tool.

   ```sh
   CGO_ENABLED=0 go build -o redhat-marketplace-reporter ./cmd/reporter/main.go
   ```

2. Log in to an Openshift context (`oc login`). The tool requires a kubectl context to be selected that has access to the Meter Definition and the namespaces where the resources are being created.

3. Create a meter definition on the cluster in the target namespace. You'll want to set the start and end time to be within the last few hours and not in the future.

4. Port forward to the Prometheus of the Red Hat Marketplace and assign to the local port 9090.

   ```sh
   oc port-forward prometheus-rhm-marketplaceconfig-meterbase-0 9090:9090 -n openshift-redhat-marketplace
   ```

5. Run the report tool targeting the created meter definition.

   ```sh
   redhat-marketplace-reporter report --name meter-report-2020-08-17 --namespace openshift-redhat-marketplace --local --uploadTarget=local-path

   # --name // name of the the report to run created in step 3
   # --namespace // namespace of the report to run created in step 3
   # --local // set to target a local prometheus instance on port 9090
   # --uploadTarget=local-path // do not try to upload the data, just writes to disk
   ```

6. The files are written to the working directory you're currently in as a tar.gz file.
