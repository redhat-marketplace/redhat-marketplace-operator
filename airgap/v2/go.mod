module github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2

go 1.16

require (
	github.com/canonical/go-dqlite v1.8.0
	github.com/go-co-op/gocron v1.5.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0
	github.com/golang/protobuf v1.5.0
	github.com/google/uuid v1.2.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/mitchellh/go-homedir v1.1.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.0
	github.com/twitchtv/twirp v8.1.0+incompatible
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20210320140829-1e4c9ba3b0c4
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/grpc v1.32.0
	google.golang.org/protobuf v1.26.0
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.21.5
	storj.io/drpc v0.0.24
)
