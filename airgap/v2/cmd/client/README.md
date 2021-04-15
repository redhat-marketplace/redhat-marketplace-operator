# Airgap client
GRPC client to communicate with airgap services.

# Configuration overview
|Property|Default Value|Type|Description|
|----|-----|----|----|
|address|localhost:8001|String|GRPC server URL|
|insecure|true|Boolean|Toggle to enable/disable SSL communication with server|
|certificate-path||String|If SSL is enabled, specify certificate path here|

# Setup steps:

## Prerequisite
- Running instance of an Airgap gRPC server

## Install the client
- Clone the repository
```
cd ${GO_PATH}\src\github.com
git clone <ssh/https url>
```

- Navigate to this directory
```
${GO_PATH}\src\github.com\redhat-marketplace-operator\airgap\v2\cmd\client
```

- Execute this command through command prompt
```
go install
```

## Using the client
- Downloading a file by it's name
```
client download --config <path_to_config>/config.yaml --file-name <filename.extension> --output-directory <output_dir>
```

- Downloading a file by it's identifier
```
client download --config <path_to_config>/config.yaml --file-id <file_identifier> --output-directory <output_dir>
```
