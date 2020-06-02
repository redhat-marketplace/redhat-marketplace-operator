# Private registry on CRC

CRC ships with a private registry we can expose. This involves creating a CA,
having the CRC VM trust that CA, create certificates from that CA, and adding
that cert to your local machine.

Once all those steps are complete you can push to the private registry.

```sh
# generate key - stores it in the a .crc folder in the work directory
make registry-generate-key
# add the ca to the crc server
make registry-add-ca
# add the cert to your local machine
make registry-add-cert
# sets up the remote route for the registry
make registry-setup

# now - restart your docker daemon

# login to registry aka docker login
make registry-login

# add the docker login to the crc server
make registry-add-docker-auth
```

You can now tag and push to public-image-registry.apps-crc.testing/symposium
registry. The default of the operator build and agent build repos.

## Troubleshooting

You may find yourself in a state where your image cannot be pulled. Verify these
things:

```sh
# Re-run add-docker-auth. This will re-add auth
# to your service account
make registry-add-docker-auth
```

If add-docker-auth doesn't fix it. Try to run:

```sh
export SERVICE_ACCOUNT=redhat-marketplace-operator
oc secrets link $SERVICE_ACCOUNT my-docker-secrets --for=pull
```

This will give your service account access to the registry.

_Pro tip:_ deleting and recreeating the pod is the easiest method of requeing it.
