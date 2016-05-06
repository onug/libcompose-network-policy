package deploy

import (
	"os"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libcompose/deploy/labels"
	"github.com/docker/libcompose/deploy/nethooks"
	"github.com/docker/libcompose/deploy/ops"
	"github.com/docker/libcompose/project"
)

type eventType int

const (
	noEvent = eventType(iota)
	startEvent = eventType(iota)
	stopEvent = eventType(iota)
	scaleEvent = eventType(iota)
)

func getEvent(event string) eventType {
	switch event {
	case "up", "start":
		return startEvent
	case "down", "delete", "kill", "rm", "stop":
		return stopEvent
	case "scale":
		return scaleEvent
	case "create", "build", "ps", "port", "pull", "log", "restart":
		// unsupported
	}

	return noEvent
}

func PopulateEnvLabels(p *project.Project, csvLabels string) error {
	parts, err := labels.Parse(csvLabels)
	if err != nil {
		return err
	}

	if err := labels.Insert(p, parts); err != nil {
		return err
	}

	return nil
}

func PreHooks(p *project.Project, e string) error {
	if err := ops.LoadOps(); err != nil {
		log.Fatalf("Failed to load ops policies: %s", err)
		os.Exit(10)
		return err
	}

	if err := nethooks.Init(); err != nil {
		log.Fatalf("Failed to Init: %s", err)
		os.Exit(10)
		return err
	}

	event := getEvent(e)
	switch event {
	case startEvent:
		if err := nethooks.CreateNetConfig(p); err != nil {
			log.Fatalf("Failed to Create Network Config: %s", err)
			os.Exit(10)
			return err
		}
	case scaleEvent:
		if err := nethooks.ScaleNetConfig(p); err != nil {
			log.Fatalf("Failed to Scale Network Config: %s", err)
			os.Exit(10)
			return err
		}
	case stopEvent:
	}

	switch event {
	case startEvent, scaleEvent:
		if err := nethooks.AutoGenLabels(p); err != nil {
			log.Fatalf("Failed to AutoGenerate Lables: %s", err)
			os.Exit(10)
			return err
		}
		if err := nethooks.AutoGenParams(p); err != nil {
			log.Fatalf("Failed to AutoGenerate Params: %s", err)
			os.Exit(10)
			return err
		}
	}

	return nil
}

func PostHooks(p *project.Project, e string) error {
	event := getEvent(e)

	switch event {
	case startEvent:
	case scaleEvent:
	case stopEvent:
		if err := nethooks.DeleteNetConfig(p); err != nil {
			log.Debugf("Failed to Delete Netwrok Config Params: %s", err)
			return err
		}
	}

	return nil
}
