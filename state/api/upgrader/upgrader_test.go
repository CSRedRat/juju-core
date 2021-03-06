// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package upgrader_test

import (
	stdtesting "testing"

	. "launchpad.net/gocheck"

	"launchpad.net/juju-core/errors"
	"launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/api/upgrader"
	statetesting "launchpad.net/juju-core/state/testing"
	coretesting "launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/testing/checkers"
	"launchpad.net/juju-core/version"
)

func TestAll(t *stdtesting.T) {
	coretesting.MgoTestPackage(t)
}

type upgraderSuite struct {
	testing.JujuConnSuite

	stateAPI *api.State

	// These are raw State objects. Use them for setup and assertions, but
	// should never be touched by the API calls themselves
	rawMachine *state.Machine
	rawCharm   *state.Charm
	rawService *state.Service
	rawUnit    *state.Unit

	upgrader *upgrader.Upgrader
}

var _ = Suite(&upgraderSuite{})

func (s *upgraderSuite) SetUpTest(c *C) {
	s.JujuConnSuite.SetUpTest(c)

	// Create a machine to work with
	var err error
	s.rawMachine, err = s.State.AddMachine("series", state.JobHostUnits)
	c.Assert(err, IsNil)
	err = s.rawMachine.SetProvisioned("foo", "fake_nonce", nil)
	c.Assert(err, IsNil)
	err = s.rawMachine.SetPassword("test-password")
	c.Assert(err, IsNil)

	// Login as the machine agent of the created machine.
	s.stateAPI = s.OpenAPIAsMachine(c, s.rawMachine.Tag(), "test-password", "fake_nonce")
	c.Assert(s.stateAPI, NotNil)

	// Create the upgrader facade.
	s.upgrader, err = s.stateAPI.Upgrader()
	c.Assert(err, IsNil)
	c.Assert(s.upgrader, NotNil)
}

func (s *upgraderSuite) TearDownTest(c *C) {
	if s.stateAPI != nil {
		err := s.stateAPI.Close()
		c.Check(err, IsNil)
	}
	s.JujuConnSuite.TearDownTest(c)
}

// Note: This is really meant as a unit-test, this isn't a test that should
//       need all of the setup we have for this test suite
func (s *upgraderSuite) TestNew(c *C) {
	upgrader := upgrader.New(s.stateAPI)
	c.Assert(upgrader, NotNil)
}

func (s *upgraderSuite) TestSetToolsWrongMachine(c *C) {
	cur := version.Current
	err := s.upgrader.SetTools(params.AgentTools{
		Tag:    "42",
		Arch:   cur.Arch,
		Series: cur.Series,
		Major:  cur.Major,
		Minor:  cur.Minor,
		Patch:  cur.Patch,
		Build:  cur.Build,
	})
	c.Assert(err, ErrorMatches, "permission denied")
	c.Assert(params.ErrCode(err), Equals, params.CodeUnauthorized)
}

func (s *upgraderSuite) TestSetTools(c *C) {
	cur := version.Current
	tools, err := s.rawMachine.AgentTools()
	c.Assert(err, checkers.Satisfies, errors.IsNotFoundError)
	c.Assert(tools, IsNil)
	err = s.upgrader.SetTools(params.AgentTools{
		Tag:    s.rawMachine.Tag(),
		Arch:   cur.Arch,
		Series: cur.Series,
		URL:    "",
		Major:  cur.Major,
		Minor:  cur.Minor,
		Patch:  cur.Patch,
		Build:  cur.Build,
	})
	c.Assert(err, IsNil)
	s.rawMachine.Refresh()
	tools, err = s.rawMachine.AgentTools()
	c.Assert(err, IsNil)
	c.Assert(tools, NotNil)
	c.Check(tools.Binary, Equals, cur)
}

func (s *upgraderSuite) TestToolsWrongMachine(c *C) {
	tools, err := s.upgrader.Tools("42")
	c.Assert(err, ErrorMatches, "permission denied")
	c.Assert(params.ErrCode(err), Equals, params.CodeUnauthorized)
	c.Assert(tools, IsNil)
}

func (s *upgraderSuite) TestTools(c *C) {
	cur := version.Current
	curTools := &state.Tools{Binary: cur, URL: ""}
	if curTools.Minor > 0 {
		curTools.Minor -= 1
	}
	s.rawMachine.SetAgentTools(curTools)
	// Upgrader.Tools returns the *desired* set of tools, not the currently
	// running set. We want to upgraded to cur.Version
	tools, err := s.upgrader.Tools(s.rawMachine.Tag())
	c.Assert(err, IsNil)
	c.Assert(tools, NotNil)
	c.Check(tools.Tag, Equals, s.rawMachine.Tag())
	c.Check(tools.Major, Equals, cur.Major)
	c.Check(tools.Minor, Equals, cur.Minor)
	c.Check(tools.Patch, Equals, cur.Patch)
	c.Check(tools.Build, Equals, cur.Build)
	c.Check(tools.Arch, Equals, cur.Arch)
	c.Check(tools.Series, Equals, cur.Series)
	c.Check(tools.URL, Not(Equals), "")
}

func (s *upgraderSuite) TestWatchAPIVersion(c *C) {
	w, err := s.upgrader.WatchAPIVersion(s.rawMachine.Tag())
	c.Assert(err, IsNil)
	defer statetesting.AssertStop(c, w)
	wc := statetesting.NewNotifyWatcherC(c, s.BackingState, w)
	// Initial event
	wc.AssertOneChange()
	vers := version.MustParse("10.20.34")
	err = statetesting.SetAgentVersion(s.BackingState, vers)
	c.Assert(err, IsNil)
	// One change noticing the new version
	wc.AssertOneChange()
	// Setting the version to the same value doesn't trigger a change
	err = statetesting.SetAgentVersion(s.BackingState, vers)
	c.Assert(err, IsNil)
	wc.AssertNoChange()
	vers = version.MustParse("10.20.35")
	err = statetesting.SetAgentVersion(s.BackingState, vers)
	c.Assert(err, IsNil)
	wc.AssertOneChange()
	statetesting.AssertStop(c, w)
	wc.AssertClosed()
}
