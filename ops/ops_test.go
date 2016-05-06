package ops

import (
	"io/ioutil"
	"testing"
)

const (
	tmpFile = "/tmp/data1122"
)

func writeTmpData(t *testing.T, jsonData []byte) {
    err := ioutil.WriteFile(tmpFile, jsonData, 0644)
		if err != nil {
			t.Fatalf("error writing to tmp file %#v", err)
		}
}

func TestUserPolicy(t *testing.T) {
    jsonData := []byte(`
			{
			"UserPolicy" : [
					{ "User":"admin", 
                      "DefaultTenant" : "foo",
					  "Networks": "all" },
					{ "User":"vagrant", 
					  "DefaultTenant" : "bar",
					  "Networks": "test,dev",
					  "DefaultNetwork" : "dev" }
				]
			}
		`)

	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err != nil {
		t.Fatalf("error loading ops with file %s \n", err)
	}

	if err := UserOpsCheckNetwork("admin", "public"); err != nil {
		t.Fatalf("error validating unspecified network")
	}

	if err := UserOpsCheckNetwork("vagrant", "test"); err != nil {
		t.Fatalf("error validating specified network")
	}

	if err := UserOpsCheckNetwork("vagrant", "public"); err == nil {
		t.Fatalf("error validating disallowed network")
	}

    jsonData = []byte(`
			{
			"UserPolicy" : [
					{ "User":"admin",
					  "NetworkPolicies": "all",
					  "DefaultNetworkPolicy":"allowall"},
					{ "User":"vagrant", 
					  "NetworkPolicies": "app1,app2,trustapp",
					  "DefaultNetworkPolicy":"trustapp" } ],
			"NetworkPolicy" : [
					{ "Name":"trustapp", "Rules": ["permit app"] },
					{ "Name":"allowall", "Rules": ["permit all"] },
					{ "Name":"app1", "Rules": ["permit tcp/4444"] },
					{ "Name":"app2", "Rules": ["permit icmp", "permit tcp/8080"] },
					{ "Name":"app3", "Rules": ["permit udp/5000", "permit tcp/5000"] } ]
			}
		`)

	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err != nil {
		t.Fatalf("error loading ops with file %s \n", err)
	}

	if err := UserOpsCheckNetworkPolicy("admin", "app3"); err != nil {
		t.Fatalf("error validating allow 'all' policy privileges")
	}

	if err := UserOpsCheckNetworkPolicy("vagrant", "app1"); err != nil {
		t.Fatalf("error validating allow specific policy privileges")
	}

	if err := UserOpsCheckNetworkPolicy("vagrant", "app3"); err == nil {
		t.Fatalf("error validating disallowed policy privileges")
	}

	if defPolicy, err := UserOpsGetDefaultNetworkPolicy("admin"); err != nil {
		t.Fatalf("error fetching default policy for user")
	} else if defPolicy != "allowall" {
		t.Fatalf("got invalid default policy for the user")
	}

	if defPolicy, err := UserOpsGetDefaultNetworkPolicy("vagrant"); err != nil {
		t.Fatalf("error fetching default policy for user")
	} else if defPolicy != "trustapp" {
		t.Fatalf("got invalid default policy for the user")
	}

    jsonData = []byte(`
			{
			"UserPolicy" : [
				{ "User":"fake",
				  "Networks": "test, dev",
				  "DefaultNetwork": "none"
				} ]
			}
		`)
	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err == nil {
		t.Fatalf("successfully loaded config with invalid default network")
	}

    jsonData = []byte(`
			{
			"UserPolicy" : [
				{ "User":"fake",
				  "NetworkPolicies" : "one, two, three",
				  "DefaultNetworkPolicy" : "four" }
				]
			}
		`)
	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err == nil {
		t.Fatalf("successfully loaded config with invalid default network policy")
	}

}

func TestNetworkPolicy(t *testing.T) {
    jsonData := []byte(`
			{
			"NetworkPolicy" : [
					{ "Name":"RedisDefault", "Rules": ["permit tcp/6379", "permit tcp/6001"] },
					{ "Name":"WebDefault", "Rules": ["permit tcp/80", "permit icmp" ] },
					{ "Name":"TrustApp", "Rules": ["permit app"] }
				]
			}
		`)

	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err != nil {
		t.Fatalf("error loading ops with file %s \n", err)
	}

	natPorts, err := GetRules("UnknownPolicy")
	if err == nil {
		t.Fatalf("error validating unknown policy: %s", err)
	}

	natPorts, err = GetRules("RedisDefault")
	if err != nil {
		t.Fatalf("error fetching rules from db policy: %s", err)
	}
	if len(natPorts) != 2 {
		t.Errorf("natPorts = %#v", natPorts)
		t.Fatalf("error parsing the rules")
	}
	if natPorts[0].Proto() != "tcp" && natPorts[0].Port() != "6379" {
		t.Fatalf("error parsing the port/proto in rules")
	}
	if natPorts[1].Proto() != "tcp" && natPorts[1].Port() != "6001" {
		t.Fatalf("error parsing the port/proto in rules")
	}

	natPorts, err = GetRules("WebDefault")
	if err != nil {
		t.Fatalf("error fetching rules from web policy: %s", err)
	}
	if len(natPorts) != 2 {
		t.Errorf("natPorts = %#v", natPorts)
		t.Fatalf("error parsing the rules")
	}
	if natPorts[0].Proto() != "tcp" && natPorts[0].Port() != "80" {
		t.Fatalf("error parsing the port/proto in rules")
	}
	if natPorts[1].Proto() != "icmp" {
		t.Fatalf("error parsing the port/proto in rules")
	}

	natPorts, err = GetRules("TrustApp")
	if err != nil {
		t.Fatalf("Error validating unknown policy: %s", err)
	}
	if len(natPorts) != 1 {
		t.Errorf("natPorts = %#v", natPorts)
		t.Fatalf("Error parsing the trust app rules")
	}
	if natPorts[0].Proto() != "app" {
		t.Fatalf("error parsing app proto fields")
	}
}

func TestInvalidNetworkPolicy(t *testing.T) {
    jsonData := []byte(`
			{ "NetworkPolicy" : [{ "Name":"JunkPolicy", "Rules": ["permit pp/6379"] }] }
		`)

	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err != nil {
		t.Fatalf("Error loading ops file: %s", err)
	}
	if _, err := GetRules("JunkPolicy"); err == nil {
		t.Fatalf("Successfully loaded invalid protocol")
	}

    jsonData = []byte(`
		{ "NetworkPolicy" : [{ "Name":"JunkPolicy", "Rules": ["permit tcp/666666"] }] }
	`)

	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err != nil {
		t.Fatalf("Error loading ops file: %s", err)
	}
	if _, err := GetRules("JunkPolicy"); err == nil {
		t.Fatalf("Successfully loaded port beyond valid range")
	}

    jsonData = []byte(`
			{ "NetworkPolicy" : [{ "Name":"JunkPolicy", "Rules": ["deny tcp/8080"] }] }
		`)

	writeTmpData(t, jsonData)
	if err := loadOpsWithFile(tmpFile); err != nil {
		t.Fatalf("Error loading ops file: %s", err)
	}
	if _, err := GetRules("JunkPolicy"); err == nil {
		t.Fatalf("Successfully loaded deny rule port")
	}
}
