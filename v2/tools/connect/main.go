package main

import (
	"fmt"
	"os"

	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/tools/connect"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "redhat-marketplace-operator-tools",
	Short: "Utility script for redhat products",
	Run:   func(cmd *cobra.Command, args []string) {},
}

func init() {
	rootCmd.AddCommand(connect.WaitAndPublishCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}
