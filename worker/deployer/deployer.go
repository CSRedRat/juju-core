package deployer

import (
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/watcher"
	"launchpad.net/juju-core/trivial"
	"launchpad.net/tomb"
)

// Deployer is responsible for deploying and recalling unit agents, according
// to changes in a set of state units; and for the final removal of its agents'
// units from state when they are no longer needed.
type Deployer struct {
	tomb       tomb.Tomb
	st         *state.State
	mgr        Manager
	entityName string
	deployed   map[string]bool
}

// Manager abstracts away the differences between different unit deployment
// strategies; where a Deployer is responsible for what to deploy, a Manager
// is responsible for how to deploy.
type Manager interface {

	// DeployUnit causes the agent for the specified unit to be started and run
	// continuously until further notice without further intervention. It will
	// return an error if the agent is already deployed.
	DeployUnit(unitName, initialPassword string) error

	// RecallUnit causes the agent for the specified unit to be stopped, and
	// the agent's data to be destroyed. It will return an error if the agent
	// was not deployed by the manager.
	RecallUnit(unitName string) error

	// DeployedUnits returns the names of all units deployed by the manager.
	DeployedUnits() ([]string, error)
}

// NewDeployer returns a Deployer that deploys and recalls unit agents via
// mgr, according to membership and lifecycle changes notified by w.
func NewDeployer(st *state.State, mgr Manager, entityName string, w *state.UnitsWatcher) *Deployer {
	d := &Deployer{
		st:         st,
		mgr:        mgr,
		entityName: entityName,
		deployed:   map[string]bool{},
	}
	go func() {
		defer d.tomb.Done()
		defer watcher.Stop(w, &d.tomb)
		d.tomb.Kill(d.loop(w))
	}()
	return d
}

func (d *Deployer) String() string {
	return "deployer for " + d.entityName
}

func (d *Deployer) Stop() error {
	d.tomb.Kill(nil)
	return d.tomb.Wait()
}

func (d *Deployer) Wait() error {
	return d.tomb.Wait()
}

// changed ensures that the named unit is deployed, recalled, or removed, as
// indicated by its state.
func (d *Deployer) changed(unitName string) error {
	// Determine unit life state, and whether we're responsible for it.
	log.Printf("worker/deployer: checking unit %q", unitName)
	var life state.Life
	responsible := false
	unit, err := d.st.Unit(unitName)
	if state.IsNotFound(err) {
		life = state.Dead
	} else if err != nil {
		return err
	} else {
		life = unit.Life()
		if deployerName, ok := unit.DeployerName(); ok {
			responsible = deployerName == d.entityName
		}
	}
	// Deployed units must be removed if they're Dead, or if the deployer
	// is no longer responsible for them.
	if d.deployed[unitName] {
		if life == state.Dead || !responsible {
			if err := d.recall(unitName); err != nil {
				return err
			}
		}
	}
	// The only units that should be deployed are those that (1) we are responsible
	// for and (2) are Alive -- if we're responsible for a Dying unit that is not
	// yet deployed, we should remove it immediately rather than undergo the hassle
	// of deploying a unit agent purely so it can set itself to Dead.
	if responsible && !d.deployed[unitName] {
		if life == state.Alive {
			return d.deploy(unit)
		} else if unit != nil {
			return d.remove(unit)
		}
	}
	return nil
}

// deploy will deploy the supplied unit with the deployer's manager. It will
// panic if it observes inconsistent internal state.
func (d *Deployer) deploy(unit *state.Unit) error {
	unitName := unit.Name()
	if d.deployed[unit.Name()] {
		panic("must not re-deploy a deployed unit")
	}
	log.Printf("worker/deployer: deploying unit %q", unit)
	initialPassword, err := trivial.RandomPassword()
	if err != nil {
		return err
	}
	if err := unit.SetPassword(initialPassword); err != nil {
		return err
	}
	if err := d.mgr.DeployUnit(unitName, initialPassword); err != nil {
		return err
	}
	d.deployed[unitName] = true
	return nil
}

// recall will recall the named unit with the deployer's manager. It will
// panic if it observes inconsistent internal state.
func (d *Deployer) recall(unitName string) error {
	if !d.deployed[unitName] {
		panic("must not recall a unit that is not deployed")
	}
	log.Printf("worker/deployer: recalling unit %q", unitName)
	if err := d.mgr.RecallUnit(unitName); err != nil {
		return err
	}
	delete(d.deployed, unitName)
	return nil
}

// remove will remove the supplied unit from state. It will panic if it
// observes inconsistent internal state.
func (d *Deployer) remove(unit *state.Unit) error {
	if d.deployed[unit.Name()] {
		panic("must not remove a deployed unit")
	} else if unit.Life() == state.Alive {
		panic("must not remove an Alive unit")
	}
	log.Printf("worker/deployer: removing unit %q", unit)
	if err := unit.EnsureDead(); err != nil {
		return err
	}
	service, err := unit.Service()
	if err != nil {
		return err
	}
	return service.RemoveUnit(unit)
}

func (d *Deployer) loop(w *state.UnitsWatcher) error {
	deployed, err := d.mgr.DeployedUnits()
	if err != nil {
		return err
	}
	for _, unitName := range deployed {
		d.deployed[unitName] = true
		if err := d.changed(unitName); err != nil {
			return err
		}
	}
	for {
		select {
		case <-d.tomb.Dying():
			return tomb.ErrDying
		case changes, ok := <-w.Changes():
			if !ok {
				return watcher.MustErr(w)
			}
			for _, unitName := range changes {
				if err := d.changed(unitName); err != nil {
					return err
				}
			}
		}
	}
	panic("unreachable")
}