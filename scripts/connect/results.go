package connect

import (
	"fmt"

	"emperror.dev/errors"
)

type containerResults map[string]*containerResult

type containerResult struct {
	Finished bool
	Error    error
}

func newContainerResults(pids map[string]string) containerResults {
	results := make(containerResults)

	for key := range pids {
		results[key] = &containerResult{}
	}

	return results
}

func (c containerResults) Process(client *connectClient, pids map[string]string, tag string) {
	for pid, digest := range pids {
		if c[pid].Finished {
			continue
		}

		fmt.Printf("getting digest status for %s\n", pid)
		fetchedTag, err := client.GetTag(pid, digest)
		if err != nil {
			c[pid].Finished = true
			c[pid].Error = errors.Wrap(err, "failed to get digest status")
			continue
		}

		if fetchedTag == nil {
			fmt.Printf("tag not found\n")
			continue
		}

		fmt.Printf("retrieved (%s) for pid %s\n", fetchedTag.String(), pid)

		switch fetchedTag.ScanStatus {
		case "scan_in_progress":
			fmt.Printf("pid %s still scanning\n", pid)
			continue
		case "failed":
			c[pid].Finished = true
			c[pid].Error = errors.Wrap(err, "scan failed")
			continue
		}

		if fetchedTag.Published {
			fmt.Printf("pid %s has been published\n", pid)
			c[pid].Finished = true
			continue
		}

		fmt.Printf("pid %s with tag %s passed scan, publishing...\n", pid, tag)

		result, err := client.PublishDigest(pid, fetchedTag.Digest, tag)
		if err != nil {
			if result != nil {
				err = errors.Wrapf(err, "pid %s failed to publish: %s %s", pid, result.Status, result.Message)
			} else {
				err = errors.Wrapf(err, "pid %s failed to publish", pid)
			}

			c[pid].Finished = true
			c[pid].Error = err
			continue
		}

		if !result.IsOK() && !result.IsError() {
			err := errors.Errorf("pid %s failed to publish: %s", pid, result.Message)
			c[pid].Error = err
			continue
		}

		fmt.Printf("pid %s has been submitted for publish\n", pid)
	}

}

func (c containerResults) HasError() bool {
	for _, v := range c {
		if v.Error != nil {
			return true
		}
	}

	return false
}

func (c containerResults) IsFinished() bool {
	for _, v := range c {
		if !v.Finished {
			return false
		}
	}

	return true
}

func (c containerResults) PrintErrors() {
	for id, v := range c {
		if v.Error != nil {
			fmt.Printf("error with %s: %s\n", id, v.Error)
		}
	}
}
