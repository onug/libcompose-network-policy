package labels

import (
	"testing"

	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/project"
)

func TestParseLabels(t *testing.T) {
	labelStr := `io.contiv.env:prod, io.contiv.tenant:t1`

	parts, err := Parse(labelStr)
	if err != nil {
		t.Fatalf("Error parsing labels. Error %v", err)
	}

	if val, ok := parts["io.contiv.env"]; !ok {
		t.Fatalf("Error finding the key in the parsed map")
	} else if val != "prod" {
		t.Fatalf("Error finding the correct value for the key in the parsed map")
	}

	if val, ok := parts["io.contiv.tenant"]; !ok {
		t.Fatalf("Error finding the key in the parsed map")
	} else if val != "t1" {
		t.Fatalf("Error finding the correct value for the key in the parsed map")
	}
}

func TestInsertLabels(t *testing.T) {
	labelStr := `io.contiv.env:prod, io.contiv.tenant:t1`

	parts, err := Parse(labelStr)
	if err != nil {
		t.Fatalf("Unable to parse labels. Error %v", err)
	}

	p, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{"docker-compose.yml"},
			ProjectName: "example",
		},
	})

	if err != nil {
		t.Fatalf("Unable to create a project. Error %v\n", err)
	}

	if Insert(p, parts); err != nil {
		t.Fatalf("Unable to insert new labels to a project. Error %v", err)
	}

	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		parts = svc.Labels.MapParts()
		if parts == nil {
			t.Fatalf("Unable to find inserted parts in the project")
		}

		if val, ok := parts["io.contiv.env"]; !ok {
			t.Fatalf("Error finding the key in the parsed map")
		} else if val != "prod" {
			t.Fatalf("Error finding the correct value for the key in the parsed map")
		}

		if val, ok := parts["io.contiv.tenant"]; !ok {
			t.Fatalf("Error finding the key in the parsed map")
		} else if val != "t1" {
			t.Fatalf("Error finding the correct value for the key in the parsed map")
		}
	}
}
