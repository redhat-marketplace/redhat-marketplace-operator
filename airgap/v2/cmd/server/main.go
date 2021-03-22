package main

import (
	"net"
	"os"
	"os/signal"

	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileserver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/server"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

func main() {
	var api string
	var db string
	var join *[]string
	var dir string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "grpc-api",
		Short: "Command to start up grpc server and database",
		Long:  `Command to start up grpc server and establish a database connection`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := database.InitDB(dir, db, join, verbose)
			if err != nil {
				return err
			}

			lis, err := net.Listen("tcp", api)
			if err != nil {
				return err
			}

			s := grpc.NewServer()
			fileserver.RegisterFileServerServer(s, &server.Server{})
			go func() {
				if err := s.Serve(lis); err != nil {
					panic(err)
				}
			}()

			ch := make(chan os.Signal)
			signal.Notify(ch, unix.SIGINT)
			signal.Notify(ch, unix.SIGQUIT)
			signal.Notify(ch, unix.SIGTERM)

			<-ch

			lis.Close()
			d.Close()

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&api, "api", "a", "", "address used to expose the demo API")
	flags.StringVarP(&db, "db", "d", "", "address used for internal database replication")
	join = flags.StringSliceP("join", "j", nil, "database addresses of existing nodes")
	flags.StringVarP(&dir, "dir", "D", "/tmp/dqlite-demo", "data directory")
	flags.BoolVarP(&verbose, "verbose", "v", false, "verbose logging")

	cmd.MarkFlagRequired("api")
	cmd.MarkFlagRequired("db")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
