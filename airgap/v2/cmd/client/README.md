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

## Listing files
- List latest files
```
client list --config <path_to_config>/config.yaml
```

- List latest files along with files marked for deletion
```
client list -a --config <path_to_config>/config.yaml
```

- Filtering files
```
client list --filter="created_at GREATER_THAN 2020-12-25" --filter="created_at LESS_THAN 2021-03-30" --config <path_to_config>/config.yaml
```

- Sorting files
```
client list -sort="size ASC" --config <path_to_config>/config.yaml
```

- Output file list to a csv
```
client list --output-dir=/path/to/dir --config <path_to_config>/config.yaml
```

## Downloading files
- Downloading a file by it's name
```
client download --config <path_to_config>/config.yaml --file-name <filename.extension> --output-directory <output_dir>
```

- Downloading a file by it's identifier
```
client download --config <path_to_config>/config.yaml --file-id <file_identifier> --output-directory <output_dir>
```

- Batch download
```
 client download --file-list-path <path_to_file>/files.csv --output-directory /path/to/output/dir --config <path_to_config>/config.yaml
```

**Note:** For more detailed instructions on commands, use the help flag for supported commands
```
client <command_name> -h
```
