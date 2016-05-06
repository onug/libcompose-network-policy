package nethooks

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/project"
)

var composeFile string

func writeTmpFile(t *testing.T, yamlData []byte) {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("error creating a tmp file")
	}

	composeFile = tmpfile.Name()
	err = ioutil.WriteFile(composeFile, yamlData, 0644)
	if err != nil {
		t.Fatalf("error writing to tmp file %#v", err)
	}
}

func removeTmpFile(t *testing.T) {
	os.Remove(composeFile)
}

func TestAutoGenLabel(t *testing.T) {

	yamlData := []byte(`
            hello:
              image: hello-world
            `)

	writeTmpFile(t, yamlData)
	defer removeTmpFile(t)

	p, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{composeFile},
			ProjectName: "example",
		},
	})

	if err != nil {
		t.Fatalf("Unable to create a project. Error %v\n", err)
	}

	labelCount := map[string]int{}
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		labelCount[svcName] = len(svc.Labels.MapParts())
	}

	if err := AutoGenLabels(p); err != nil {
		t.Fatalf("Unable to auto insert labels to a project. Error %v\n", err)
	}

	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		if labelCount[svcName] == len(svc.Labels.MapParts()) {
			t.Fatalf("service '%s' did not insert any labels", svcName)
		}
	}

}

func TestNetworkNameFromLabel(t *testing.T) {

	yamlData := []byte(`
            hello:
              image: hello-world
              labels:
                io.contiv.network: "foo"
            `)

	writeTmpFile(t, yamlData)
	defer removeTmpFile(t)

	p, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{composeFile},
			ProjectName: "example",
		},
	})

	if err != nil {
		t.Fatalf("Unable to create a project. Error %v\n", err)
	}

	if getNetworkNameFromProject(p) != "foo" {
		t.Fatalf("Unable to parse network from labels")
	}
}

func TestNetworkName(t *testing.T) {

	yamlData := []byte(`
            hello:
              image: hello-world
              net: "bar"
            `)

	writeTmpFile(t, yamlData)
	defer removeTmpFile(t)

	p, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{composeFile},
			ProjectName: "example",
		},
	})

	if err != nil {
		t.Fatalf("Unable to create a project. Error %v\n", err)
	}

	if getNetworkNameFromProject(p) != "bar" {
		t.Fatalf("Unable to parse net from app composition")
	}
}

func TestTenantName(t *testing.T) {

	yamlData := []byte(`
            hello:
              image: hello-world
              labels:
                io.contiv.blah: "junk"
                io.contiv.tenant: "blue"
            `)

	writeTmpFile(t, yamlData)
	defer removeTmpFile(t)

	p, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{composeFile},
			ProjectName: "example",
		},
	})

	if err != nil {
		t.Fatalf("Unable to create a project. Error %v\n", err)
	}

	if getTenantNameFromProject(p) != "blue" {
		t.Fatalf("Unable to parse tenant from app composition")
	}
}

func TestConflictingNet(t *testing.T) {

	yamlData := []byte(`
            hello:
              image: hello-world
              net: dev
            hello2:
              image: hello-world
              net: test
            `)

	writeTmpFile(t, yamlData)
	defer removeTmpFile(t)

	p, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{composeFile},
			ProjectName: "example",
		},
	})

	if err != nil {
		t.Fatalf("Unable to create a project. Error %v\n", err)
	}

	if validateProject(p) == nil {
		t.Fatalf("Successful parsing of mismatching networks")
	}
}

func TestConflictingTenant(t *testing.T) {

	yamlData := []byte(`
            hello:
              image: hello-world
              labels:
                io.contiv.tenant: "blue"
            hello2:
              image: hello-world
              net: test
              labels:
                io.contiv.tenant: "green"
            `)

	writeTmpFile(t, yamlData)
	defer removeTmpFile(t)

	p, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{composeFile},
			ProjectName: "example",
		},
	})

	if err != nil {
		t.Fatalf("Unable to create a project. Error %v\n", err)
	}

	if validateProject(p) == nil {
		t.Fatalf("Successful parsing of mismatching tenants")
	}
}
