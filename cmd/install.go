package cmd

import (
	"fmt"
	"io"
	"os"
	"path"

	systemd "github.com/alexellis/faasd/pkg/systemd"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

func makeInstallCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "install",
		Short: "Install faasd",
		RunE:  runInstall,
	}
	cmd.Flags().Bool("prepare", false, "Only prepare secrets and working directories")
	return cmd
}

const faasdwd = "/run/faasd"
const faasContainerdwd = "/run/faas-containerd"

func runInstall(command *cobra.Command, _ []string) error {

	prepareOnly, prepErr := command.Flags().GetBool("prepare")
	if prepErr != nil {
		return prepErr
	}

	if err := ensureWorkingDir(path.Join(faasdwd, "secrets")); err != nil {
		return err
	}

	if err := ensureWorkingDir(faasContainerdwd); err != nil {
		return err
	}

	if basicAuthErr := makeBasicAuthFiles(path.Join(faasdwd, "secrets")); basicAuthErr != nil {
		return errors.Wrap(basicAuthErr, "cannot create basic-auth-* files")
	}

	if err := cp("prometheus.yml", faasdwd); err != nil {
		return err
	}

	if err := cp("resolv.conf", faasdwd); err != nil {
		return err
	}

	if !prepareOnly {
		err := binExists("/usr/local/bin/", "faas-containerd")
		if err != nil {
			return err
		}

		err = binExists("/usr/local/bin/", "faasd")
		if err != nil {
			return err
		}

		err = systemd.InstallUnit("faas-containerd", map[string]string{
			"Cwd":             faasContainerdwd,
			"SecretMountPath": path.Join(faasdwd, "secrets")})

		if err != nil {
			return err
		}

		err = systemd.InstallUnit("faasd", map[string]string{"Cwd": faasdwd})
		if err != nil {
			return err
		}

		err = systemd.DaemonReload()
		if err != nil {
			return err
		}

		err = systemd.Enable("faas-containerd")
		if err != nil {
			return err
		}

		err = systemd.Enable("faasd")
		if err != nil {
			return err
		}

		err = systemd.Start("faas-containerd")
		if err != nil {
			return err
		}

		err = systemd.Start("faasd")
		if err != nil {
			return err
		}
	}

	return nil
}

func binExists(folder, name string) error {
	findPath := path.Join(folder, name)
	if _, err := os.Stat(findPath); err != nil {
		return fmt.Errorf("unable to stat %s, install this binary before continuing", findPath)
	}
	return nil
}

func ensureWorkingDir(folder string) error {
	if _, err := os.Stat(folder); err != nil {
		err = os.MkdirAll(folder, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func cp(source, destFolder string) error {
	file, err := os.Open(source)
	if err != nil {
		return err

	}
	defer file.Close()

	out, err := os.Create(path.Join(destFolder, source))
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, file)

	return err
}
