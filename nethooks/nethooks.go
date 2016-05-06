package nethooks

import (
	"errors"
	"os"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libcompose/deploy/ops"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/yaml"
)

const (
	netmasterBaseURL = "http://netmaster:9999"
	applyLinksBasedPolicyFlag  = true
	applyLabelsBasedPolicyFlag = true
	applyDefaultPolicyFlag     = false
	applyContractPolicyFlag    = true
)

// CreateNetConfig creates network and policies in coniv-netmaster
func CreateNetConfig(p *project.Project) error {
	log.Debugf("Create network for the project '%s' ", p.Name)

	if err := validateProject(p); err != nil {
		os.Exit(1)
		return err
	}

	if err := checkUserCreds(p); err != nil {
		os.Exit(1)
		return err
	}

	if applyLinksBasedPolicyFlag {
		if err := applyLinksBasedPolicy(p); err != nil {
			return err
		}
		if err := clearSvcLinks(p); err != nil {
			return err
		}
		if err := clearExposedPorts(p); err != nil {
			return err
		}
	}

	return nil
}

// DeleteNetConfig removes network and policies in coniv-netmaster 
func DeleteNetConfig(p *project.Project) error {
	log.Debugf("Delete network for the project '%s' ", p.Name)

	if err := validateProject(p); err != nil {
		os.Exit(1)
		return err
	}

	if err := checkUserCreds(p); err != nil {
		os.Exit(1)
		return err
	}

	tenantName := getTenantNameFromProject(p)
	if err := deleteApp(tenantName, p); err != nil {
		log.Debugf("Unable to delete app. Error %v", err)
	}

	for _, svcName := range p.Configs.Keys() {
		if err := removeEpg(p, svcName); err != nil {
			log.Debugf("Unable to remove out-policy for service '%s'. Error %v", svcName, err)
		}

		if err := removePolicy(p, svcName, "in"); err != nil {
			log.Debugf("Unable to remove in-policy for service '%s'. Error %v", svcName, err)
		}

		if err := removePolicy(p, svcName, "out"); err != nil {
			log.Debugf("Unable to remove out-policy for service '%s'. Error %v", svcName, err)
		}

		if err := clearSvcLinks(p); err != nil {
			log.Errorf("Unable to clear service links. Error: %s", err)
		}
	}

	return nil
}

// Update service config for scale verb
func ScaleNetConfig(p *project.Project) error {
	log.Debugf("Scale network for the project '%s' ", p.Name)

	if applyLinksBasedPolicyFlag {
		if err := clearSvcLinks(p); err != nil {
			log.Errorf("Unable to clear service links. Error: %s", err)
		}
		if err := clearExposedPorts(p); err != nil {
			log.Errorf("Unable to clear exposed ports. Error: %s", err)
		}
	}
	if applyLabelsBasedPolicyFlag {
		log.Infof("Applying labels based policies")
	}

	return nil
}

// Generate Parameters: new information that was not set by users
func AutoGenParams(p *project.Project) error {
	networkName := getNetworkNameFromProject(p)
	tenantName := getTenantNameFromProject(p)
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		if svc.DNS.Len() == 0 {
			dnsAddr, err := getDnsInfo(networkName, tenantName)
			if err != nil {
				log.Errorf("error getting dns information for network %s: %s", networkName, err)
			}

			svc.DNS = yaml.NewStringorslice(dnsAddr)
			if svc.DNSSearch.Len() == 0 {
				netDomain := networkName + "." + tenantName
				tenantDomain := tenantName

				svc.DNSSearch = yaml.NewStringorslice(netDomain, tenantDomain)
			}
		}

		svc.Net = getFullSvcName(p, svcName)
	}

	return nil
}

// Generate labels to tag the services 
func AutoGenLabels(p *project.Project) error {
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		labels := svc.Labels.MapParts()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[NET_ISOLATION_GROUP_LABEL] = svcName

		userId, err := getSelfId()
		if err != nil {
			log.Errorf("error getting user id: %s", err)
			return err
		}
		labels[USER_LABEL] = userId

		svc.Labels = yaml.NewSliceorMap(labels)
	}

	return nil
}

// apply policies based on links (can be 'depends_on' in latest docker)
func applyLinksBasedPolicy(p *project.Project) error {
	links, err := getSvcLinks(p)
	if err != nil {
		log.Debugf("Unable to find links from service chains. Error %v", err)
		return err
	}

	if err := addEpgs(p); err != nil {
		log.Errorf("Unable to apply policies for unspecified tiers. Error %v", err)
		return err
	}

	policyRecs := make(map[string]policyCreateRec)
	for fromSvcName, toSvcNames := range links {
		for _, toSvcName := range toSvcNames {
			log.Infof("Creating policy contract from '%s' -> '%s'", fromSvcName, toSvcName)
			if err := applyInPolicy(p, fromSvcName, toSvcName, policyRecs); err != nil {
				log.Errorf("Failed to apply in-policy for service '%s': %s", toSvcName, err)
				return err
			}
		}
	}

	spMap, err := getSvcPorts(p)
	if err != nil {
		log.Debugf("Unable to find exposed ports from service chains. Error %v", err)
		return err
	}
	if err := applyExposePolicy(p, spMap, policyRecs); err != nil {
		log.Errorf("Unable to apply expose-policy %v", err)
		return err
	}

	tenantName := getTenantNameFromProject(p)
	if err := addApp(tenantName, p); err != nil {
		log.Errorf("Unable to create app with unspecified tiers. Error %v", err)
		return err
	}

	if applyDefaultPolicyFlag {
		if err := applyDefaultPolicy(p, policyRecs); err != nil {
			log.Errorf("Unable to apply policies for unspecified tiers. Error %v", err)
			return err
		}
	}

	return nil
}

// Checks User credentials to perform a given operation (move to Authz)
func checkUserCreds(p *project.Project) error {
	userId, err := getSelfId()
	if err != nil {
		log.Errorf("Unable to identify self: %s", err)
		return err
	}

	networkName := getNetworkNameFromProject(p)
	if err := ops.UserOpsCheckNetwork(userId, networkName); err != nil {
		log.Errorf("User '%s' not allowed on network '%s'", userId, networkName)
		return err
	}

	return nil
}

func validateProject(p *project.Project) error {
	netName := getNetworkNameFromProject(p)

	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		if getNetworkName(svc) != netName {
			log.Errorf("Mismatching networks '%s' vs '%s' for services not allowed",
				netName, getNetworkName(svc))
			return errors.New("mismatching networks")
		}
	}

	tenantName := getTenantNameFromProject(p)

	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		if getTenantName(svc) != tenantName {
			log.Errorf("Mismatching Tenants '%s' vs '%s' for services not allowed",
				tenantName, getTenantName(svc))
			return errors.New("mismatching tenants")
		}
	}

	return nil
}
