package main

import (
	"path/filepath"

	"github.com/code-ready/crc/pkg/crc/constants"
	"github.com/code-ready/crc/pkg/crc/errors"
	"github.com/code-ready/crc/pkg/crc/machine"
	"github.com/code-ready/crc/pkg/crc/output"
	"github.com/code-ready/crc/pkg/os/shell"
	"github.com/spf13/cobra"
)

var debug bool
var name, host, identityFile, forceShell, ip string

// Modified from https://github.com/code-ready/crc/blob/master/cmd/crc/cmd/podman_env.go
var podmanEnvCmd = &cobra.Command{
	Use:   "podman-env",
	Short: "Setup podman environment",
	Long:  `Setup environment for 'podman' binary to access podman on CRC VM`,
	Run: func(cmd *cobra.Command, args []string) {
		userShell, err := shell.GetShell(forceShell)
		if err != nil {
			errors.ExitWithMessage(1, "Error running the podman-env command: %s", err.Error())
		}

		ipConfig := machine.IpConfig{
			Name:  name,
			Debug: debug,
		}

		if ip == "" {
			result, err := machine.Ip(ipConfig)
			if err != nil {
				errors.ExitWithMessage(1, "Error running the podman-env command: %s", err.Error())
			}
			ip = result.IP
		}

		output.Outln(shell.GetPathEnvString(userShell, constants.CrcBinDir))
		output.Outln(shell.GetEnvString(userShell, "PODMAN_USER", constants.DefaultSSHUser))
		output.Outln(shell.GetEnvString(userShell, "PODMAN_HOST", ip))
		output.Outln(shell.GetEnvString(userShell, "PODMAN_IDENTITY_FILE", getPrivateKeyPath(name)))
		output.Outln(shell.GetEnvString(userShell, "PODMAN_IGNORE_HOSTS", "1"))
		output.Outln(shell.GenerateUsageHint(userShell, "go run scripts/podman/main.go"))
	},
}

func getPrivateKeyPath(name string) string {
	return filepath.Join(constants.MachineInstanceDir, name, "id_rsa")
}

func init() {
	podmanEnvCmd.Flags().StringVar(&name, "name", constants.DefaultName, "name of your crc")
	podmanEnvCmd.Flags().StringVar(&ip, "ip", "", "override the ip")
	podmanEnvCmd.Flags().StringVar(&forceShell, "shell", "", "Set the environment for the specified shell: [fish, cmd, powershell, tcsh, bash, zsh]. Default is auto-detect.")
	podmanEnvCmd.Flags().BoolVar(&debug, "debug", false, "Debug on or off")
}

func main() {
	podmanEnvCmd.Execute()
}
