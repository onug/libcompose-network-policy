
package ops

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strconv"
	"strings"
	"github.com/docker/go-connections/nat"
	log "github.com/Sirupsen/logrus"
)

type UserPolicyInfo struct {
	User string
	DefaultTenant string
	Networks string
	DefaultNetwork string
	NetworkPolicies string
	DefaultNetworkPolicy string
}

type NetworkPolicyInfo struct {
	Name string
	Rules []string
}

type LabelMapInfo struct {
	Tenant string
	NetworkIsolationPolicy string
}

type opsPolicy struct {
	LabelMap LabelMapInfo
	UserPolicy []UserPolicyInfo
	NetworkPolicy []NetworkPolicyInfo
}

var ops opsPolicy

func LoadOps() error {
	opsFile := "./ops.json"
	return loadOpsWithFile(opsFile)
}

func loadOpsWithFile(fileName string) error {

	composeBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatalf("error reading the config file: %s", err)
	}

	ops = opsPolicy{}
	if err := json.Unmarshal(composeBytes, &ops); err != nil {
		log.Errorf("error unmarshaling json %#v \n", err)
		return err
	}

	for _, policy := range ops.UserPolicy {
		if policy.DefaultNetwork == "" {
			continue
		}
		if err := UserOpsCheckNetwork(policy.User, policy.DefaultNetwork); err != nil {
			log.Errorf("Default network '%s' not present allowed networks '%s'",
				policy.DefaultNetwork, policy.Networks)
			return err
		}
	}

	for _, policy := range ops.UserPolicy {
		if policy.DefaultNetworkPolicy == "" {
			continue
		}
		if err := UserOpsCheckNetworkPolicy(policy.User, policy.DefaultNetworkPolicy); err != nil {
			log.Errorf("Default policy not present in the allowed policies")
			return err
		}
	}

	return nil
}

func LabelOpsGetTenant() string {
	return ops.LabelMap.Tenant
}

func LabelOpsGetNetworkIsolationPolicy() string {
	return ops.LabelMap.NetworkIsolationPolicy
}

func UserOpsCheckNetwork(userName, network string) error {
	for _, policy := range ops.UserPolicy {
		if policy.User != userName {
			continue
		}
		allowedNetworks := strings.Split(policy.Networks, ",")
		for _, allowedNetwork := range allowedNetworks {
			if allowedNetwork == network || allowedNetwork == "all" {
				return nil
			}
		}
	}

	return errors.New("Deny disallowed network")
}

func UserOpsGetDefaultNetworkPolicy(userName string) (string, error) {
	for _, policy := range ops.UserPolicy {
		if policy.User != userName {
			continue
		}
		if policy.DefaultNetworkPolicy != "" {
			return policy.DefaultNetworkPolicy, nil
		}
	}

	return "", errors.New("Default Policy Not Found")
}

func UserOpsGetDefaultNetwork(userName string) (string, error) {
	for _, policy := range ops.UserPolicy {
		if policy.User != userName {
			continue
		}
		if policy.DefaultNetwork != "" {
			return policy.DefaultNetwork, nil
		}
	}

	return "", errors.New("Default Network Not Found")
}

func UserOpsGetDefaultTenant(userName string) (string, error) {
	for _, policy := range ops.UserPolicy {
		if policy.User != userName {
			continue
		}
		if policy.DefaultTenant != "" {
			return policy.DefaultTenant, nil
		}
	}

	return "", errors.New("Default Tenant Not Found")
}

func UserOpsCheckNetworkPolicy(userName, networkPolicy string) error {
	for _, policy := range ops.UserPolicy {
		if policy.User != userName {
			continue
		}
		allowedPolicies := strings.Split(policy.NetworkPolicies, ",")
		for _, allowedPolicy := range allowedPolicies {
			if allowedPolicy == networkPolicy || allowedPolicy == "all" {
				return nil
			}
		}
	}

	return errors.New("Deny disallowed policy")
}

func GetRules(policyName string) ([]nat.Port, error) {
	portList := []nat.Port{}

	for _, policy := range ops.NetworkPolicy {
		if policy.Name != policyName {
			continue
		}
		for _, rule := range policy.Rules {
			var natPort nat.Port
			var err error

			clauses := strings.Split(rule, " ")
			if len(clauses) <= 0 {
				return portList, errors.New("none found")
			}
			switch clauses[0] {
				case "permit":
					if len(clauses) <= 1 {
						return portList, errors.New("Incomplete permit clause")
					}
					protoPort := strings.Split(clauses[1], "/")
					if len(protoPort) == 0 {
						return portList, errors.New("Empty proto/port in permit clause")
					}
					switch protoPort[0] {
						case "tcp", "udp":
							if len(protoPort) <= 1 {
								return portList, errors.New("Invalid permit clause: port or protocol missing")
							}
							pNum, _ := strconv.Atoi(protoPort[1])
							if pNum < 0 || pNum > 65535 {
								return portList, errors.New("Invalid port in permit clause")
							}
							natPort, err = nat.NewPort(protoPort[0], protoPort[1]);
							if err != nil {
								return portList, err
							}
						case "icmp":
							natPort, err = nat.NewPort(protoPort[0], "0");
							if err != nil {
								return portList, err
							}
						case "app":
							natPort, err = nat.NewPort("app", "0")
							if err != nil {
								return portList, err
							}
						case "all":
							natPort, err = nat.NewPort("all", "0")
							if err != nil {
								return portList, err
							}
						default:
							return portList, errors.New("Invalid proto in permit clause")
					}

				case "deny":
					return portList, errors.New("Not supported")

				default:
					return portList, errors.New("Invalid clause")
			}
			portList = append(portList, natPort)
		}
	}

	if len(portList) == 0 {
		return portList, errors.New("Unrecognized policy")
	}

	return portList, nil
}
