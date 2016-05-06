package nethooks

import (
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	contivClient "github.com/contiv/contivmodel/client"
	"github.com/docker/go-connections/nat"
	"github.com/docker/libcompose/deploy/ops"
	"github.com/docker/libcompose/config"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/yaml"
)

type policyCreateRec struct {
	nextRuleId    int
	policyApplied bool
}

var cl *contivClient.ContivClient

func Init() error {
	var err error
	cl, err = contivClient.NewContivClient(netmasterBaseURL)
	if err != nil {
		log.Errorf("Error connecting to netmaster")
	}

	return err
}

func getRuleStr(ruleID int) string {
	return string(ruleID + '0')
}

func getInPolicyStr(projectName, svcName string) string {
	return projectName + "_" + svcName + "-in"
}

func getOutPolicyStr(projectName, svcName string) string {
	return projectName + "_" + svcName + "-out"
}

func getTenantNameFromProject(p *project.Project) string {
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		return getTenantName(svc)
	}
	return ""
}

func getTenantName(svc *config.ServiceConfig) string {
	tenantName := TENANT_DEFAULT

	tenantLabel := ops.LabelOpsGetTenant()
	if tenantLabel == "" {
		tenantLabel = TENANT_LABEL
	}
	if labels := svc.Labels.MapParts(); labels != nil {
		if value, ok := labels[tenantLabel]; ok {
			tenantName = value
		}
	}
	return tenantName
}

func getNetworkName(svc *config.ServiceConfig) string {
	networkName := svc.Net
	if svc.Net == "" {
		networkName = NETWORK_DEFAULT
	}

	if labels := svc.Labels.MapParts(); labels != nil {
		if value, ok := labels[NETWORK_LABEL]; ok {
			networkName = value
		}
	}
	return networkName
}

func getNetworkNameFromProject(p *project.Project) string {
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		return getNetworkName(svc)
	}
	return ""
}

func getFullSvcName(p *project.Project, svcName string) string {
	svc, _ := p.Configs.Get(svcName)
	netName := getNetworkName(svc)
	tenantName := getTenantNameFromProject(p)

	fullSvcName := p.Name + "_" + svcName + "." + netName
	if tenantName != TENANT_DEFAULT {
		fullSvcName = fullSvcName + "/" + tenantName
	}

	return fullSvcName
}

func getSvcName(p *project.Project, svcName string) string {
	if p == nil {
		return svcName
	}

	return p.Name + "_" + svcName
}

func getFromEpgName(p *project.Project, fromSvcName string) string {
	if applyContractPolicyFlag {
		return getSvcName(p, fromSvcName)
	}

	return ""
}

func getSvcLinks(p *project.Project) (map[string][]string, error) {
	links := make(map[string][]string)

	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		log.Debugf("svc %s === %+v ", svcName, svc)
		svcLinks := svc.Links.Slice()
		log.Debugf("found links for svc '%s' %#v ", svcName, svcLinks)
		links[svcName] = svcLinks
	}

	return links, nil
}

func clearSvcLinks(p *project.Project) error {
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		// if len(svc.Links.Slice()) > 0 {
		svc.Links = yaml.NewMaporColonSlice([]string{})
		log.Debugf("clearing links for svc '%s' %#v ", svcName, svc.Links)
		// }
	}
	return nil
}

func extractPort(ps string) string {
	lastCol := strings.LastIndex(ps, ":")
	if lastCol == -1 {
		return ps
	}

	return ps[lastCol+1:]
}

func getSvcPorts(p *project.Project) (map[string][]string, error) {
	sPorts := make(map[string][]string)
	res := []string{}
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		if len(svc.Ports) > 0 {
			pList := svc.Ports
			for _, ps := range pList {
				res = append(res, extractPort(ps))
			}
			sPorts[svcName] = res
			log.Debugf("Service %v port %v", svcName, sPorts[svcName])
		}
	}

	return sPorts, nil
}

func clearExposedPorts(p *project.Project) error {
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		if len(svc.Expose) > 0 {
			log.Debugf("svc.Expose: %v svc.Ports %v", svc.Expose, svc.Ports)
			svc.Expose = []string{}
			log.Debugf("clearing exposed ports for svc '%s' %#v ", svcName, svc.Links)
		}
		svc.Ports = []string{}
	}
	return nil
}

func addDenyAllRule(tenantName, networkName, fromEpgName, policyName string, ruleID int) error {
	rule := &contivClient.Rule{
		Action:        "deny",
		Direction:     "in",
		FromEndpointGroup: fromEpgName,
		FromNetwork:       networkName,
		PolicyName:    policyName,
		Priority:      ruleID,
		Protocol:      "tcp",
		RuleID:        getRuleStr(ruleID),
		TenantName:    tenantName,
	}
	if err := cl.RulePost(rule); err != nil {
		log.Errorf("Unable to create deny all rule %#v. Error: %v", rule, err)
		return err
	}

	return nil
}

func addInAcceptRule(tenantName, networkName, fromEpgName, policyName, protoName string, portID, ruleID int) error {
	rule := &contivClient.Rule{
		Action:        "allow",
		Direction:     "in",
		FromEndpointGroup: fromEpgName,
		FromNetwork:       networkName,
		PolicyName:    policyName,
		Port:          portID,
		Priority:      ruleID,
		Protocol:      protoName,
		RuleID:        getRuleStr(ruleID),
		TenantName:    tenantName,
	}
	if err := cl.RulePost(rule); err != nil {
		log.Errorf("Unable to create allow rule %#v. Error: %v", rule, err)
		return err
	}

	return nil
}

func addOutAcceptAllRule(tenantName, networkName, fromEpgName, policyName string, ruleID int) error {
	rule := &contivClient.Rule{
		Action:        "allow",
		Direction:     "out",
		FromEndpointGroup: fromEpgName,
		FromNetwork:       networkName,
		PolicyName:    policyName,
		Priority:      ruleID,
		Protocol:      "tcp",
		RuleID:        getRuleStr(ruleID),
		TenantName:    tenantName,
	}
	if err := cl.RulePost(rule); err != nil {
		log.Errorf("Unable to create allow rule %#v. Error: %v", rule, err)
		return err
	}

	return nil
}

func addPolicy(tenantName, policyName string) error {
	policy := &contivClient.Policy{
		PolicyName: policyName,
		TenantName: tenantName,
	}
	if err := cl.PolicyPost(policy); err != nil {
		log.Debugf("Unable to create policy rule. Error: %v", err)
		return err
	}

	return nil
}

func addApp(tenantName string, p *project.Project) error {

	log.Debugf("Add App '%s':'%s' ", tenantName, p.Name)
	app := &contivClient.AppProfile{
		AppProfileName: p.Name,
		TenantName: tenantName,
		NetworkName: getNetworkNameFromProject(p),
	}

	for _, svcName := range p.Configs.Keys() {
		epgKey := getSvcName(p, svcName)
		app.EndpointGroups = append(app.EndpointGroups, epgKey)
		log.Debugf("Adding epg to App:%s ", epgKey)
	}

	if err := cl.AppProfilePost(app); err != nil {
		log.Debugf("Unable to post app to netmaster. Error: %v", err)
		return err
	}

	return nil
}

func deleteApp(tenantName string, p *project.Project) error {

	log.Debugf("Deleting App '%s':'%s' ", tenantName, p.Name)

	if err := cl.AppProfileDelete(tenantName, getNetworkNameFromProject(p), p.Name); err != nil {
		log.Debugf("Unable to post app delete to netmaster. Error: %v", err)
		return err
	}

	return nil
}

func addEpg(tenantName, networkName, epgName string, policies []string) error {
	epg := &contivClient.EndpointGroup{
		EndpointGroupID: 1,
		GroupName:       epgName,
		NetworkName:     networkName,
		Policies:        policies,
		TenantName:      tenantName,
	}
	if err := cl.EndpointGroupPost(epg); err != nil {
		log.Errorf("Unable to create endpoint group. Tenant '%s' Network '%s' Epg '%s'. Error %v",
			tenantName, networkName, epgName, err)
		return err
	}

	return nil
}

func addEpgs(p *project.Project) error {
	tenantName := getTenantNameFromProject(p)
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		networkName := getNetworkName(svc)
		epgName := getSvcName(p, svcName)

		if err := addEpg(tenantName, networkName, epgName, []string{}); err != nil {
			log.Errorf("Unable to add epg for service '%s'. Error %v", svcName, err)
			return err
		}
	}
	return nil
}

func applyDefaultPolicy(p *project.Project, polRecs map[string]policyCreateRec) error {
	tenantName := getTenantNameFromProject(p)
	for _, svcName := range p.Configs.Keys() {
		svc, _ := p.Configs.Get(svcName)
		networkName := getNetworkName(svc)
		toEpgName := getSvcName(p, svcName)

		if pR, ok := polRecs[svcName]; ok {
			if pR.policyApplied {
				continue
			}
		}

		// add 'in' policy for the service tier
		ruleID := 1
		policyName := getInPolicyStr(p.Name, svcName)
		policies := []string{}

		log.Debugf("Applying deny all in policy for service '%s' ", svcName)
		if err := addPolicy(tenantName, policyName); err != nil {
			log.Errorf("Unable to add policy. Error %v ", err)
			return err
		}
		policies = append(policies, policyName)

		if err := addDenyAllRule(tenantName, networkName, "", policyName, ruleID); err != nil {
			log.Errorf("Unable to add deny rule. Error %v ", err)
			return err
		}

		// add 'out' policy for the service tier
		ruleID = 1
		policyName = getOutPolicyStr(p.Name, svcName)
		if err := addPolicy(tenantName, policyName); err != nil {
			log.Errorf("Unable to add policy. Error %v", err)
		}
		policies = append(policies, policyName)
		if err := addOutAcceptAllRule(tenantName, networkName, "", policyName, ruleID); err != nil {
			log.Errorf("Unable to add deny rule. Error %v ", err)
			return err
		}

		// add epg with in and out policies
		if err := addEpg(tenantName, networkName, toEpgName, policies); err != nil {
			log.Errorf("Unable to add epg. Error %v", err)
			return err
		}
	}

	return nil
}

func getPolicyRec(name string, polRecs map[string]policyCreateRec) policyCreateRec {
	rec, ok := polRecs[name]

	if ok {
		return rec
	}

	rec = policyCreateRec{nextRuleId: 1, policyApplied: false}
	polRecs[name] = rec
	return rec
}

func applyExposePolicy(p *project.Project, expMap map[string][]string, polRecs map[string]policyCreateRec) error {

	tenantName := getTenantNameFromProject(p)
	for toSvcName, spList := range expMap {
		svc, _ := p.Configs.Get(toSvcName)
		networkName := getNetworkName(svc)
		policyRec := getPolicyRec(toSvcName, polRecs)
		ruleID := policyRec.nextRuleId
		policyName := getInPolicyStr(p.Name, toSvcName)
		// create the policy, if necessary
		if !policyRec.policyApplied && (len(spList) > 0) {
			policies := []string{}
			if err := addPolicy(tenantName, policyName); err != nil {
				log.Errorf("Unable to add policy. Error %v ", err)
				return err
			}
			toEpgName := getSvcName(p, toSvcName)
			policies = append(policies, policyName)
			if err := addEpg(tenantName, networkName, toEpgName, policies); err != nil {
				log.Errorf("Unable to add epg. Error %v", err)
				return err
			}
		}

		for _, portID := range spList {
			pNum, err := strconv.Atoi(portID)
			if err != nil {
				log.Errorf("Unable to get port number. Error %v ", err)
			}

			if err = addInAcceptRule(tenantName, networkName, "", policyName, "tcp", pNum, ruleID); err != nil {
				log.Errorf("Unable to add allow rule. Error %v ", err)
				return err
			} else {
				log.Debugf("Exposed %v : port %v", policyName, portID)
			}
			ruleID++
		}
		policyRec.nextRuleId = ruleID
		polRecs[toSvcName] = policyRec
	}

	return nil
}

func getPolicyName(userId string, svc *config.ServiceConfig) (string, error) {
	var err error

	policyName := ""

	policyLabel := ops.LabelOpsGetNetworkIsolationPolicy()
	if policyLabel == "" {
		policyLabel = NET_ISOLATION_POLICY_LABEL
	}

	if labels := svc.Labels.MapParts(); labels != nil {
		if value, ok := labels[policyLabel]; ok {
			policyName = value
		}
	}

	if policyName == "" {
		policyName, err = ops.UserOpsGetDefaultNetworkPolicy(userId)
		if err != nil {
			log.Errorf("Unable to find find default policy: %s", err)
			return policyName, err
		}
		log.Infof("Using default policy '%s'...", policyName)
	}

	if err = ops.UserOpsCheckNetworkPolicy(userId, policyName); err != nil {
		log.Errorf("User '%s' not allowed to use policy '%s'", userId, policyName)
		return "", err
	}

	return policyName, nil
}

func getServicePorts(svcName string, svc *config.ServiceConfig) ([]nat.Port, error) {

	userId, err := getSelfId()
	if err != nil {
		log.Errorf("Unable to identify self: %s", err)
		return []nat.Port{}, err
	}

	policyName, err := getPolicyName(userId, svc)
	if err != nil {
		log.Errorf("Error obtaining policy : %s ", err)
		return []nat.Port{}, err
	}

	policyPorts, err := ops.GetRules(policyName)
	if err != nil {
		log.Errorf("Unable to get rules for policy '%s': %s", policyName, err)
		return []nat.Port{}, err
	}

	log.Infof("User '%s': applying '%s' to service '%s'", userId, policyName, svcName)

	natPorts := []nat.Port{}
	for _, policyPort := range policyPorts {
		// borrow port information from the app
		if policyPort.Proto() == "app" {
			natPorts1, err := getImageInfo(svc.Image)
			if err != nil {
				log.Errorf("Unable to auto fetch port/protocol information. Error %v", err)
				return []nat.Port{}, err
			}
			natPorts = append(natPorts, natPorts1...)
		} else {
			natPorts = append(natPorts, policyPort)
		}
	}

	return natPorts, nil
}

func applyInPolicy(p *project.Project, fromSvcName, toSvcName string, polRecs map[string]policyCreateRec) error {
	svc,_ := p.Configs.Get(toSvcName)

	policyRec := getPolicyRec(toSvcName, polRecs)
	tenantName := getTenantNameFromProject(p)
	networkName := getNetworkName(svc)
	toEpgName := getSvcName(p, toSvcName)

	policyName := getInPolicyStr(p.Name, toSvcName)
	fromEpgName := getFromEpgName(p, fromSvcName)

	ruleID := policyRec.nextRuleId
	policies := []string{}

	natPorts, err := getServicePorts(toSvcName, svc)
	if err != nil {
		return err
	}

	for _, natPort := range natPorts {
		if natPort.Proto() == "all" {
			log.Infof("Allowing all traffic to service '%s'", toSvcName)
			return nil
		}
	}

	log.Debugf("Creating network objects to service '%s': Tenant: %s Network %s", toSvcName, tenantName, networkName)

	if err := addPolicy(tenantName, policyName); err != nil {
		log.Errorf("Unable to add policy. Error %v ", err)
		return err
	}
	policies = append(policies, policyName)

	if err := addDenyAllRule(tenantName, networkName, "", policyName, ruleID); err != nil {
		return err
	}
	ruleID++

	for _, natPort := range natPorts {
		pNum, _ := strconv.Atoi(natPort.Port())
		if err := addInAcceptRule(tenantName, networkName, fromEpgName, policyName, natPort.Proto(), pNum, ruleID); err != nil {
			log.Errorf("Unable to add allow rule. Error %v ", err)
			return err
		}
		ruleID++
	}

	if err := addEpg(tenantName, networkName, toEpgName, policies); err != nil {
		log.Errorf("Unable to add epg. Error %v", err)
		return err
	}

	policyRec.nextRuleId = ruleID
	policyRec.policyApplied = true
	polRecs[toSvcName] = policyRec
	return nil
}

func removePolicy(p *project.Project, svcName, dir string) error {
	log.Debugf("Deleting policies for service '%s' ", svcName)
	tenantName := getTenantNameFromProject(p)
	policyName := getInPolicyStr(p.Name, svcName)
	if dir == "out" {
		policyName = getOutPolicyStr(p.Name, svcName)
	}

	if err := cl.PolicyDelete(tenantName, policyName); err != nil {
		log.Debugf("Unable to delete '%s' policy. Error: %v", policyName, err)
	}

	return nil
}

func removeEpg(p *project.Project, svcName string) error {
	svc,_ := p.Configs.Get(svcName)

	log.Debugf("Deleting Epg for service '%s' ", svcName)
	tenantName := getTenantNameFromProject(p)
	networkName := getNetworkName(svc)
	epgName := getSvcName(p, svcName)

	if err := cl.EndpointGroupDelete(tenantName, networkName, epgName); err != nil {
		log.Debugf("Unable to delete '%s' epg. Error: %v", epgName, err)
	}

	return nil
}
