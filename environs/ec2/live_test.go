package ec2_test

import (
	"fmt"
	. "launchpad.net/gocheck"
	"launchpad.net/juju/go/environs"
	"launchpad.net/juju/go/environs/jujutest"
)

// integrationConfig holds the environments configuration
// for running the amazon EC2 integration tests.
//
// This is missing keys for security reasons; set the following environment variables
// to make the integration testing work:
//  access-key: $AWS_ACCESS_KEY_ID
//  admin-secret: $AWS_SECRET_ACCESS_KEY
var integrationConfig = []byte(`
environments:
  sample:
    type: ec2
`)

func registerIntegrationTests() {
	envs, err := environs.ReadEnvironsBytes(integrationConfig)
	if err != nil {
		panic(fmt.Errorf("cannot parse integration tests config data: %v", err))
	}
	for _, name := range envs.Names() {
		Suite(&jujutest.LiveTests{
			Environs: envs,
			Name:     name,
		})
	}
}