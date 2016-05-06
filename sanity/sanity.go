package main


import (
	"io/ioutil"
	"os"
	"os/exec"
	"errors"
	"strings"

        log "github.com/Sirupsen/logrus"
)

// ***************** Utils  ********************

var composeFile string
func runCmd(cmd string) ([]string, error) {
	lines := []string{}

	output, err := exec.Command("/bin/bash", "-c", cmd).CombinedOutput()

	rawLines := strings.Split(string(output), "\n")
	for _, line := range rawLines {
		lines = append(lines, strings.TrimSpace(line))
	}

	return lines, err
}

func checkImages() error {
	for _,image := range []string{"jainvipin/web", "jainvipin/redis"} {
		_, err := runCmd("docker inspect " + image)
		if err != nil {
			log.Infof("pulling image: '%s'", image)
			_, err = runCmd("docker pull " + image)
			if err != nil {
				return errors.New("unable to pull image")
			}
		}

		vanillaImage := strings.Split(image, "/")[1]
		_, err = runCmd("docker inspect " + vanillaImage)
		if err != nil {
			log.Infof("retagging image: '%s'", image)
			_, err = runCmd("docker tag " + image + " " + vanillaImage)
			if err != nil {
				return errors.New("unable to retag image")
			}
		}
	}

	return nil
}

func writeTmpFile(yamlData []byte) error {
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		return err
	}

	composeFile = tmpfile.Name()
	if err = ioutil.WriteFile(composeFile, yamlData, 0644); err != nil {
		return err
	}

	return nil
}

func removeTmpFile() {
	os.Remove(composeFile)
}

func writeOpsFile(opsJsonData []byte) error {

    if err := ioutil.WriteFile("ops.json", opsJsonData, 0644); err != nil {
        return err
    }

	return nil
}

func removeOpsFile() {
	os.Remove("ops.json")
}


// ***************** Composition and Verification  ******************

func runComposition(projectName string) (string, error) {
	composeCmd := "contiv-compose -p " + projectName + " -f " + composeFile + " up -d"
	output, err := runCmd(composeCmd)
	return strings.Join(output, " "), err
}

func stopComposition(projectName string) error {
	composeCmd := "contiv-compose -p " + projectName + " -f " + composeFile + " stop"
	_, err := runCmd(composeCmd)
	return err
}

func verifyPolicy(projectName, portRange, allowedPort, disallowedPort string) error {
	fromContainer := projectName + "_web_1"
	toContainer := projectName + "_redis"
	verifyCmd := "docker exec " + fromContainer + " nc -zvw 1 " + toContainer + " " + portRange

	output, err := runCmd(verifyCmd)
	if err != nil {
		log.Errorf("Error running command '%s': %s", verifyCmd, output)
		return errors.New("Unable to run verify command")
	}

	timedOutStr := " (?) : Connection timed out"
	if strings.Contains(strings.Join(output, " "), allowedPort + timedOutStr) {
		log.Errorf("Failed to verify allowed ports '%s' in: %s", allowedPort, output)
		return errors.New("Unable to verify: allowed port not open")
	}

	if !strings.Contains(strings.Join(output, " "), disallowedPort + timedOutStr) {
		log.Errorf("Failed to verify allowed ports '%s' in: %s", allowedPort, output)
		return errors.New("Unable to verify: disallowed port is not timing out")
	}

	return nil
}

func checkCompose() error {
	if _, err := runCmd("which contiv-compose"); err != nil {
		lcBin := os.Getenv("GOPATH") + "/src/github.com/docker/libcompose/bundles/libcompose-cli"
		if _, err = runCmd("which " + lcBin); err != nil {
			return errors.New("contiv-compose or libcompose binary not found")
		}
		if _, err = runCmd("sudo cp " + lcBin + " /usr/bin/contiv-compose"); err != nil {
			return errors.New("Unable to move bundles/libcompose-cli to /usr/bin/contiv-compose")
		}
		if _, err := runCmd("which contiv-compose"); err != nil {
			return errors.New("$PATH does not include /usr/bin!!")
		}
	}
	return nil
}

func checkNetplugin() error {
	if _, err := runCmd("which netplugin && which netmaster && which netctl"); err != nil {
		return errors.New("Unable to find netplugin binaries")
	}

	if _, err := runCmd("netctl net ls"); err != nil {
		log.Infof("Netplugin not running - attempting to start")
		preWdir, _ := os.Getwd()
		netpluginDir := os.Getenv("GOPATH") + "/src/github.com/contiv/netplugin"
		if err := os.Chdir(netpluginDir); err != nil {
			log.Errorf("Error chdir to netplugin path: %s", netpluginDir)
			return errors.New("Unable to go to netplugin path")
		}
		defer os.Chdir(preWdir)

		if output, err := runCmd("make host-restart"); err != nil {
			log.Errorf("Error starting netplugin binaries: %s", output)
			return errors.New("Unable to start netplugin; please make sure it is running")
		}
	}

	tenants := []string{ "blue" }
	for _, tenant := range tenants {
		if output, err := runCmd("netctl tenant create " + tenant); err != nil {
			if !strings.Contains(strings.Join(output, " "), "Cant change tenant parameters") {
				return errors.New("error crating tenant")
			}
		}
	}

	nets := []string{
				"10.11.1.0/24 dev",
				"10.22.1.0/24 test",
				"10.33.1.0/24 production",
				"10.11.2.0/24 -t blue dev",
				"10.22.2.0/24 -t blue test",
				"10.33.2.0/24 -t blue production",
			}
	for _, net := range nets {
		if output, err := runCmd("netctl net create -s " + net); err != nil {
			if !strings.Contains(strings.Join(output, " "), "Cant change network parameters") {
				return errors.New("error creating network")
			}
		}
	}

	return nil
}

// ***************** Test Cases ******************

func basicTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
        redis:  
          image: redis
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "basic"
	if output, err := runComposition(projectName); err != nil {
		log.Errorf("Error running composition: %s", output)
		return err
	}
	defer stopComposition(projectName)

	if err := verifyPolicy(projectName, "6379-6380", "6379", "6380"); err != nil {
		return err
	}

	log.Infof("  Pass")

	return nil
}

func overridePolicyTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
          net: test
        redis:  
          image: redis
          net: test
          labels:
           io.contiv.policy: "RedisDefault"
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "override_policy"
	if output, err := runComposition(projectName); err != nil {
		if !strings.Contains(output, "not allowed on network") {
			log.Fatalf("Unable to verify invalid network: %s", output)
		}
	}
	defer stopComposition(projectName)

	if err := verifyPolicy(projectName, "6377-6380", "6378", "6380"); err != nil {
		return err
	}

	log.Infof("  Pass")

	return nil
}

func nonDefaultNetworkTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
          net: test
        redis:  
          image: redis
          net: test
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "test_net"
	if output, err := runComposition(projectName); err != nil {
		log.Errorf("Error running composition: %s", output)
		return err
	}
	defer stopComposition(projectName)

	if err := verifyPolicy(projectName, "6379-6380", "6379", "6380"); err != nil {
		return err
	}

	log.Infof("  Pass")

	return nil
}

func nonDefaultTenantTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
          labels:
           io.contiv.tenant: "blue"
        redis:  
          image: redis
          labels:
           io.contiv.tenant: "blue"
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "blue_tenant"
	if output, err := runComposition(projectName); err != nil {
		log.Errorf("Error running composition: %s", output)
		return err
	}
	defer stopComposition(projectName)

	if err := verifyPolicy(projectName, "6379-6380", "6379", "6380"); err != nil {
		return err
	}

	log.Infof("  Pass")

	return nil
}

func disallowedNetworkTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
          net: production
        redis:  
          image: redis
          net: production
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "invalid_net"
	if output, err := runComposition(projectName); err != nil {
		if !strings.Contains(output, "not allowed on network") {
			log.Fatalf("Unable to run composition with invalid network: %s", output)
		}
	}

	log.Infof("  Pass")

	return nil
}

func disallowedPolicyTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
        redis:  
          image: redis
          labels:
           io.contiv.policy: "AllPriviliges"
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "invalid_policy"
	if output, err := runComposition(projectName); err != nil {
		if !strings.Contains(output, "not allowed to use policy") {
			log.Fatalf("Unable to run composition with invalid policy: %s", output)
		}
	}

	log.Infof("  Pass")

	return nil
}

func inconsistentNetworkInfoTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
          net: dev
        redis:  
          image: redis
          net: test
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "inconsistent_tenant"
	if _, err := runComposition(projectName); err == nil {
		log.Fatalf("Successfully ran inconsistent network config")
	}

	log.Infof("  Pass")

	return nil
}

func inconsistentTenantInfoTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
          labels:
           io.contiv.tenant: "blue"
        redis:  
          image: redis
          labels:
           io.contiv.tenant: "green"
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "inconsistent_tenant"
	if _, err := runComposition(projectName); err == nil {
		log.Fatalf("Successfully ran inconsistent tenant config")
	}

	log.Infof("  Pass")

	return nil
}

func customPolicyLabelTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
        redis:  
          image: redis
          labels:
           net.isolation.policy: "RedisDefault"
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "custom_policy_label"
	if output, err := runComposition(projectName); err != nil {
		if !strings.Contains(output, "not allowed on network") {
			log.Fatalf("Unable to verify invalid network: %s", output)
		}
	}
	defer stopComposition(projectName)

	if err := verifyPolicy(projectName, "6377-6380", "6378", "6380"); err != nil {
		return err
	}

	log.Infof("  Pass")

	return nil
}

func customTenantLabelTest() error {
	yamlData := []byte(`
        web:
          image: web
          ports:
           - "5000:5000"
          links:
           - redis
          labels:
           tenant: "blue"
        redis:  
          image: redis
          labels:
           tenant: "blue"
       `)

	if err := writeTmpFile(yamlData); err != nil {
		log.Fatalf("error creating tmp file: %s", err)
	}
	defer removeTmpFile()

	projectName := "custom_tenant_label"
	if output, err := runComposition(projectName); err != nil {
		log.Errorf("Error running composition: %s", output)
		return err
	}
	defer stopComposition(projectName)

	if err := verifyPolicy(projectName, "6379-6380", "6379", "6380"); err != nil {
		return err
	}

	log.Infof("  Pass")

	return nil
}

// ***************** Main: load ops and run tests ****************

func main() {

	log.Infof("Checking Environment")
	if err := checkCompose(); err != nil {
		log.Fatalf("Env check failed: %s", err)
	}
	if err := checkNetplugin(); err != nil {
		log.Fatalf("Env check failed: %s", err)
	}

	workDir := "/tmp"
	if err := os.Chdir(workDir); err != nil {
		log.Fatalf("Unable to changet the workdir: %s", workDir)
	}

	log.Infof("Checking Images")
	if err := checkImages(); err != nil {
		log.Fatalf("Unable to download/fetch images: %s", err)
	}

	opsJsonData := []byte(`
		{
        "LabelMap" : {
                "Tenant" : "io.contiv.tenant",
                "NetworkIsolationPolicy" : "io.contiv.policy"
        },

        "UserPolicy" : [

                { "User":"admin",
                  "Networks": "all",
                  "DefaultNetwork": "dev",
                  "NetworkPolicies" : "all",
                  "DefaultNetworkPolicy": "AllPriviliges" },

                { "User":"vagrant",
                  "DefaultTenant": "default",
                  "Networks": "test,dev",
                  "DefaultNetwork": "dev",
                  "NetworkPolicies" : "TrustApp,RedisDefault,WebDefault",
                  "DefaultNetworkPolicy": "TrustApp" }
        ],

        "NetworkPolicy" : [
                { "Name":"AllPriviliges",
                  "Rules": ["permit all"]},

                { "Name":"RedisDefault",
                  "Rules": ["permit tcp/6379", "permit tcp/6378", "permit tcp/6377"] },

                { "Name":"WebDefault",
                  "Rules": ["permit tcp/80", "permit icmp" ] },

                { "Name":"TrustApp",
                  "Rules": ["permit app"] }
        ]
		}
	`)
	if err := writeOpsFile(opsJsonData); err != nil {
		log.Fatalf("error writing ops file")
	}
	defer removeOpsFile()

	testName := "basic test"
	log.Infof("Running test: %s", testName)
	if err := basicTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "override policy"
	log.Infof("Running test: %s", testName)
	if err := overridePolicyTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "disallowed network"
	log.Infof("Running test: %s", testName)
	if err := disallowedNetworkTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "disallowed policy"
	log.Infof("Running test: %s", testName)
	if err := disallowedPolicyTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "non default network"
	log.Infof("Running test: %s", testName)
	if err := nonDefaultNetworkTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "non default tenant"
	log.Infof("Running test: %s", testName)
	if err := nonDefaultTenantTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "inconsnstent networks"
	log.Infof("Running test: %s", testName)
	if err := inconsistentNetworkInfoTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "non default tenant"
	log.Infof("Running test: %s", testName)
	if err := inconsistentTenantInfoTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	opsJsonData = []byte(`
		{
        "LabelMap" : {
                "Tenant" : "tenant",
                "NetworkIsolationPolicy" : "net.isolation.policy"
        },

        "UserPolicy" : [
                { "User":"vagrant",
                  "DefaultTenant": "default",
                  "Networks": "test,dev",
                  "DefaultNetwork": "dev",
                  "NetworkPolicies" : "TrustApp,RedisDefault,WebDefault",
                  "DefaultNetworkPolicy": "TrustApp" }
        ],

        "NetworkPolicy" : [
                { "Name":"RedisDefault",
                  "Rules": ["permit tcp/6379", "permit tcp/6378", "permit tcp/6377"] },

                { "Name":"WebDefault",
                  "Rules": ["permit tcp/80", "permit icmp" ] },

                { "Name":"TrustApp",
                  "Rules": ["permit app"] }
        ]
		}
	`)
	removeOpsFile()
	if err := writeOpsFile(opsJsonData); err != nil {
		log.Fatalf("error writing ops file")
	}
	defer removeOpsFile()

	testName = "custom tenant label"
	log.Infof("Running test: %s", testName)
	if err := customTenantLabelTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

	testName = "custom policy label"
	log.Infof("Running test: %s", testName)
	if err := customPolicyLabelTest(); err != nil {
		log.Fatalf("Error in %s: %s", testName, err)
	}

}
