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

func newContainerResults(containers map[string]string) containerResults {
	results := make(containerResults)

	for key := range containers {
		results[key] = &containerResult{}
	}

	return results
}

func (c containerResults) Process(client *connectClient, containers map[string]string, tag string) {
	for pid, digest := range containers {
		if c[pid].Finished {
			continue
		}

		fmt.Printf("getting digest status for %s\n", pid)
		status, err := client.GetDigestStatus(pid, digest)
		if err != nil {
			c[pid].Finished = true
			c[pid].Error = errors.Wrap(err, "failed to get digest status")
			break
		}

		if status == nil {
			fmt.Printf("digest status is not ready\n")
			continue
		}

		if !status.IsPassed() {
			fmt.Printf("digest status is not passed for pid %s\n", pid)
			continue
		}

		result, err := client.PublishDigest(pid, digest, tag)
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

		if !result.IsOK() && !result.IsPublished() {
			err := errors.Errorf("pid %s failed to publish: %s", pid, result.Message)
			c[pid].Finished = true
			c[pid].Error = err
			continue
		}

		fmt.Printf("pid %s has been published\n", pid)
		c[pid].Finished = true
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
