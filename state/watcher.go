package state

import (
	"launchpad.net/goyaml"
	"launchpad.net/juju/go/state/watcher"
	"launchpad.net/tomb"
)

// ConfigWatcher observes changes to any configuration node.
type ConfigWatcher struct {
	st         *State
	path       string
	tomb       tomb.Tomb
	watcher    *watcher.ContentWatcher
	changeChan chan *ConfigNode
}

// newConfigWatcher creates and starts a new config watcher for
// the given path.
func newConfigWatcher(st *State, path string) *ConfigWatcher {
	w := &ConfigWatcher{
		st:         st,
		path:       path,
		changeChan: make(chan *ConfigNode),
		watcher:    watcher.NewContentWatcher(st.zk, path),
	}
	go w.loop()
	return w
}

// Changes returns a channel that will receive the new
// *ConfigNode when a change is detected. Note that multiple
// changes may be observed as a single event in the channel.
func (w *ConfigWatcher) Changes() <-chan *ConfigNode {
	return w.changeChan
}

// Stop stops the watch and returns any error encountered
// while watching. This method should always be called
// before discarding the watcher.
func (w *ConfigWatcher) Stop() error {
	w.tomb.Kill(nil)
	if err := w.watcher.Stop(); err != nil {
		w.tomb.Wait()
		return err
	}
	return w.tomb.Wait()
}

// loop is the backend for watching the configuration node.
func (w *ConfigWatcher) loop() {
	defer w.tomb.Done()
	defer close(w.changeChan)

	for {
		select {
		case <-w.tomb.Dying():
			return
		case change, ok := <-w.watcher.Changes():
			if !ok {
				return
			}
			// A non-existent node is treated as an empty node.
			configNode, err := parseConfigNode(w.st.zk, w.path, change.Content)
			if err != nil {
				w.tomb.Kill(err)
				return
			}
			select {
			case <-w.watcher.Dying():
				return
			case <-w.tomb.Dying():
				return
			case w.changeChan <- configNode:
			}
		}
	}
}

// NeedsUpgradeWatcher observes changes to a unit's upgrade flag.
type NeedsUpgradeWatcher struct {
	st         *State
	path       string
	tomb       tomb.Tomb
	watcher    *watcher.ContentWatcher
	changeChan chan NeedsUpgrade
}

// newNeedsUpgradeWatcher creates and starts a new resolved flag node 
// watcher for the given path.
func newNeedsUpgradeWatcher(st *State, path string) *NeedsUpgradeWatcher {
	w := &NeedsUpgradeWatcher{
		st:         st,
		path:       path,
		changeChan: make(chan NeedsUpgrade),
		watcher:    watcher.NewContentWatcher(st.zk, path),
	}
	go w.loop()
	return w
}

// Changes returns a channel that will receive notifications
// about upgrades for the unit. Note that multiple changes
// may be observed as a single event in the channel.
func (w *NeedsUpgradeWatcher) Changes() <-chan NeedsUpgrade {
	return w.changeChan
}

// Stop stops the watch and returns any error encountered
// while watching. This method should always be called
// before discarding the watcher.
func (w *NeedsUpgradeWatcher) Stop() error {
	w.tomb.Kill(nil)
	if err := w.watcher.Stop(); err != nil {
		w.tomb.Wait()
		return err
	}
	return w.tomb.Wait()
}

// loop is the backend for watching the resolved flag node.
func (w *NeedsUpgradeWatcher) loop() {
	defer w.tomb.Done()
	defer close(w.changeChan)

	for {
		select {
		case <-w.tomb.Dying():
			return
		case change, ok := <-w.watcher.Changes():
			if !ok {
				return
			}
			var needsUpgrade NeedsUpgrade
			if change.Exists {
				needsUpgrade.Upgrade = true
				var setting needsUpgradeNode
				if err := goyaml.Unmarshal([]byte(change.Content), &setting); err != nil {
					w.tomb.Kill(err)
					return
				}
				needsUpgrade.Force = setting.Force
			}
			select {
			case <-w.watcher.Dying():
				return
			case <-w.tomb.Dying():
				return
			case w.changeChan <- needsUpgrade:
			}
		}
	}
}

// ResolvedWatcher observes changes to a unit's resolved
// mode. See SetResolved for details.
type ResolvedWatcher struct {
	st         *State
	path       string
	tomb       tomb.Tomb
	watcher    *watcher.ContentWatcher
	changeChan chan ResolvedMode
}

// newResolvedWatcher returns a new ResolvedWatcher watching path.
func newResolvedWatcher(st *State, path string) *ResolvedWatcher {
	w := &ResolvedWatcher{
		st:         st,
		path:       path,
		changeChan: make(chan ResolvedMode),
		watcher:    watcher.NewContentWatcher(st.zk, path),
	}
	go w.loop()
	return w
}

// Changes returns a channel that will receive the new
// resolved mode when a change is detected. Note that multiple
// changes may be observed as a single event in the channel.
func (w *ResolvedWatcher) Changes() <-chan ResolvedMode {
	return w.changeChan
}

// Stop stops the watch and returns any error encountered
// while watching. This method should always be called
// before discarding the watcher.
func (w *ResolvedWatcher) Stop() error {
	w.tomb.Kill(nil)
	if err := w.watcher.Stop(); err != nil {
		w.tomb.Wait()
		return err
	}
	return w.tomb.Wait()
}

// loop is the backend for watching the resolved flag node.
func (w *ResolvedWatcher) loop() {
	defer w.tomb.Done()
	defer close(w.changeChan)

	for {
		select {
		case <-w.tomb.Dying():
			return
		case change, ok := <-w.watcher.Changes():
			if !ok {
				return
			}
			mode, err := parseResolvedMode(change.Content)
			if err != nil {
				w.tomb.Kill(err)
				return
			}
			select {
			case <-w.watcher.Dying():
				return
			case <-w.tomb.Dying():
				return
			case w.changeChan <- mode:
			}
		}
	}
}

// PortsWatcher observes changes to a unit's open ports.
// See OpenPort for details.
type PortsWatcher struct {
	st         *State
	path       string
	tomb       tomb.Tomb
	watcher    *watcher.ContentWatcher
	changeChan chan []Port
}

// newPortsWatcher creates and starts a new ports node 
// watcher for the given path.
func newPortsWatcher(st *State, path string) *PortsWatcher {
	w := &PortsWatcher{
		st:         st,
		path:       path,
		changeChan: make(chan []Port),
		watcher:    watcher.NewContentWatcher(st.zk, path),
	}
	go w.loop()
	return w
}

// Changes returns a channel that will receive the actual
// open ports when a change is detected. Note that multiple
// changes may be observed as a single event in the channel.
func (w *PortsWatcher) Changes() <-chan []Port {
	return w.changeChan
}

// Stop stops the watch and returns any error encountered
// while watching. This method should always be called
// before discarding the watcher.
func (w *PortsWatcher) Stop() error {
	w.tomb.Kill(nil)
	if err := w.watcher.Stop(); err != nil {
		w.tomb.Wait()
		return err
	}
	return w.tomb.Wait()
}

// loop is the backend for watching the ports node.
func (w *PortsWatcher) loop() {
	defer w.tomb.Done()
	defer close(w.changeChan)

	for {
		select {
		case <-w.tomb.Dying():
			return
		case change, ok := <-w.watcher.Changes():
			if !ok {
				return
			}
			var ports openPortsNode
			if err := goyaml.Unmarshal([]byte(change.Content), &ports); err != nil {
				w.tomb.Kill(err)
				return
			}
			select {
			case <-w.watcher.Dying():
				return
			case <-w.tomb.Dying():
				return
			case w.changeChan <- ports.Open:
			}
		}
	}
}

// MachinesWatcher notifies about machines being added or removed 
// from the environment.
type MachinesWatcher struct {
	st               *State
	path             string
	tomb             tomb.Tomb
	changeChan       chan *MachinesChange
	watcher          *watcher.ContentWatcher
	knownMachineKeys []string
}

// newMachinesWatcher creates and starts a new machine watcher.
func newMachinesWatcher(st *State) *MachinesWatcher {
	// start with an empty topology
	topology, _ := parseTopology("")
	w := &MachinesWatcher{
		st:               st,
		path:             zkTopologyPath,
		changeChan:       make(chan *MachinesChange),
		watcher:          watcher.NewContentWatcher(st.zk, zkTopologyPath),
		knownMachineKeys: topology.MachineKeys(),
	}
	go w.loop()
	return w
}

// Changes returns a channel that will receive the actual
// watcher.ChildrenChanges. Note that multiple changes may
// be observed as a single event in the channel.
func (w *MachinesWatcher) Changes() <-chan *MachinesChange {
	return w.changeChan
}

// Stop stops the watch and returns any error encountered
// while watching. This method should always be called
// before discarding the watcher.
func (w *MachinesWatcher) Stop() error {
	w.tomb.Kill(nil)
	if err := w.watcher.Stop(); err != nil {
		w.tomb.Wait()
		return err
	}
	return w.tomb.Wait()
}

// loop is the backend for watching the ports node.
func (w *MachinesWatcher) loop() {
	defer w.tomb.Done()
	defer close(w.changeChan)
	for {
		select {
		case <-w.tomb.Dying():
			return
		case change, ok := <-w.watcher.Changes():
			if !ok {
				return
			}
			topology, err := parseTopology(change.Content)
			if err != nil {
				w.tomb.Kill(err)
				return
			}
			currentMachineKeys := topology.MachineKeys()
			added, deleted := diff(currentMachineKeys, w.knownMachineKeys), diff(w.knownMachineKeys, currentMachineKeys)
			w.knownMachineKeys = currentMachineKeys
			if len(added) == 0 && len(deleted) == 0 {
				// nothing changed in zkMachinePath
				continue
			}
			// Why are we dealing with strings, not *Machines at this point ?
			// Because *Machine does not define equality, yet.
			mc := new(MachinesChange)
			for _, m := range added {
				mc.Added = append(mc.Added, &Machine{w.st, m})
			}
			for _, m := range deleted {
				mc.Deleted = append(mc.Deleted, &Machine{w.st, m})
			}
			select {
			case <-w.watcher.Dying():
				return
			case <-w.tomb.Dying():
				return
			case w.changeChan <- mc:
			}
		}
	}
}

// diff returns all the elements that exist in A but not B.
func diff(A, B []string) (missing []string) {
next:
	for _, a := range A {
		for _, b := range B {
			if a == b {
				continue next
			}
		}
		missing = append(missing, a)
	}
	return
}