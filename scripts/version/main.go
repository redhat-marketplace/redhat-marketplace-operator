package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
	"github.ibm.com/symposium/redhat-marketplace-operator/version"
)

var filename string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Hugo",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", version.Version)
	},
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Print the version number of Hugo",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		currentVersion, _ := semver.NewVersion(version.Version)
		nextVersion := currentVersion

		if flag, _ := cmd.Flags().GetBool("major"); flag {
			*nextVersion = nextVersion.IncMajor()
		} else if flag, _ := cmd.Flags().GetBool("minor"); flag {
			*nextVersion = nextVersion.IncMinor()
		} else if flag, _ := cmd.Flags().GetBool("patch"); flag {
			*nextVersion = nextVersion.IncPatch()
		}

		if flag, _ := cmd.Flags().GetBool("prerelease"); flag {
			prereleaseName, _ := cmd.Flags().GetString("prerelease-name")
			build, _ := cmd.Flags().GetInt("build")
			prerelease := fmt.Sprintf("%s.%v", prereleaseName, build)

			*nextVersion, _ = nextVersion.SetPrerelease(prerelease)
		}

		SetInFile(nextVersion.String(), filename)

		if flag, _ := cmd.Flags().GetBool("commit"); flag {
			runCommand("git", "add", filename)
			runCommand("git", "commit", "-m", nextVersion.String())
			runCommand("git", "tag", nextVersion.String(), "--annotate", "--message", "release: "+nextVersion.String())
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.PersistentFlags().StringVar(&filename, "filename", "version/version.go", "file name of the version")
	nextCmd.Flags().Bool("commit", false, "commit and tag in git")
	nextCmd.Flags().Bool("major", false, "major release")
	nextCmd.Flags().Bool("minor", false, "minor release")
	nextCmd.Flags().Bool("patch", true, "patch release")
	nextCmd.Flags().Bool("prerelease", false, "prerelease")
	nextCmd.Flags().String("prerelease-name", "beta", "prerelease")
	nextCmd.Flags().Int("build", 1, "prerelease")
}

var rootCmd = &cobra.Command{
	Use:   "bump-version",
	Short: "Bumps the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", version.Version)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}

func findBasicLit(file *ast.File) (*ast.BasicLit, error) {
	for _, decl := range file.Decls {
		switch gd := decl.(type) {
		case *ast.GenDecl:
			if gd.Tok != token.CONST {
				continue
			}
			spec, _ := gd.Specs[0].(*ast.ValueSpec)
			if strings.ToUpper(spec.Names[0].Name) == "VERSION" {
				value, ok := spec.Values[0].(*ast.BasicLit)
				if !ok || value.Kind != token.STRING {
					return nil, fmt.Errorf("VERSION is not a string, was %#v\n", value.Value)
				}
				return value, nil
			}
		default:
			continue
		}
	}
	return nil, errors.New("bump_version: No version const found")
}

func writeFile(filename string, fset *token.FileSet, file *ast.File) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	return cfg.Fprint(f, fset, file)
}

func changeInFile(filename string, f func(*ast.BasicLit) error) error {
	fset := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	lit, err := findBasicLit(parsedFile)
	if err != nil {
		return fmt.Errorf("No Version const found in %s", filename)
	}
	if err := f(lit); err != nil {
		return err
	}
	writeErr := writeFile(filename, fset, parsedFile)
	return writeErr
}

// SetInFile sets the version in filename to newVersion.
func SetInFile(newVersion string, filename string) error {
	return changeInFile(filename, func(lit *ast.BasicLit) error {
		lit.Value = fmt.Sprintf("\"%s\"", newVersion)
		return nil
	})
}

// runCommand execs the given command and exits if it fails.
func runCommand(binary string, args ...string) {
	out, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when running command: %s.\nOutput was:\n%s", err.Error(), string(out))
		os.Exit(2)
	}
}
