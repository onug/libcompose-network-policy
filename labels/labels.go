package labels

import (
	"errors"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/yaml"
)

func Parse(csvLabels string) (map[string]string, error) {
	labels := map[string]string{}

	csvRecs := strings.Split(csvLabels, ",")
	if len(csvRecs) == 0 {
		return nil, errors.New("unable to parse labels")
	}

	for _, csvRec := range csvRecs {
		csvRec := strings.Trim(csvRec, ", ")
		csvValues := strings.Split(csvRec, ":")
		if len(csvValues) != 2 {
			log.Errorf("unable to parse labels: '%s'", csvRec)
			return nil, errors.New("unable to parse label record")
		}

		labels[csvValues[0]] = csvValues[1]
	}

	return labels, nil
}

func Insert(p *project.Project, parts map[string]string) error {
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		origParts := svc.Labels.MapParts()
		if origParts == nil {
			origParts = make(map[string]string)
		}

		for partKey, partValue := range parts {
			origParts[partKey] = partValue
		}
		log.Debugf("Updated composition labels for service %s: %#v", svcName, origParts)
		svc.Labels = yaml.NewSliceorMap(origParts)
	}

	return nil
}
