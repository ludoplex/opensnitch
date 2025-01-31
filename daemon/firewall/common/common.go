package common

import (
	"sync"
	"time"

	"github.com/evilsocket/opensnitch/daemon/log"
)

// default arguments for various functions
var (
	EnableRule     = true
	DoLogErrors    = true
	ForcedDelRules = true
	ReloadRules    = true
	RestoreChains  = true
	BackupChains   = true
	ReloadConf     = true
)

type (
	callback     func()
	callbackBool func() bool

	// Common holds common fields and functionality of both firewalls,
	// iptables and nftables.
	Common struct {
		RulesChecker *time.Ticker
		stopChecker  chan bool
		QueueNum     uint16
		Running      bool
		Intercepting bool
		FwEnabled    bool
		sync.RWMutex
	}
)

// SetQueueNum sets the queue number used by the firewall.
// It's the queue where all intercepted connections will be sent.
func (c *Common) SetQueueNum(qNum *int) {
	c.Lock()
	defer c.Unlock()

	if qNum != nil {
		c.QueueNum = uint16(*qNum)
	}

}

// IsRunning returns if the firewall is running or not.
func (c *Common) IsRunning() bool {
	c.RLock()
	defer c.RUnlock()

	return c != nil && c.Running
}

// IsFirewallEnabled returns if the firewall is running or not.
func (c *Common) IsFirewallEnabled() bool {
	c.RLock()
	defer c.RUnlock()

	return c != nil && c.FwEnabled
}

// IsIntercepting returns if the firewall is running or not.
func (c *Common) IsIntercepting() bool {
	c.RLock()
	defer c.RUnlock()

	return c != nil && c.Intercepting
}

// NewRulesChecker starts monitoring interception rules.
// We expect to have 2 rules loaded: one to intercept DNS responses and another one
// to intercept network traffic.
func (c *Common) NewRulesChecker(areRulesLoaded callbackBool, reloadRules callback) {
	c.Lock()
	defer c.Unlock()

	if c.RulesChecker != nil {
		c.RulesChecker.Stop()
		select {
		case c.stopChecker <- true:
		case <-time.After(5 * time.Millisecond):
			log.Error("NewRulesChecker: timed out stopping monitor rules")
		}
	}
	c.stopChecker = make(chan bool, 1)
	c.RulesChecker = time.NewTicker(time.Second * 10)

	go startCheckingRules(c.stopChecker, c.RulesChecker, areRulesLoaded, reloadRules)
}

// StartCheckingRules monitors if our rules are loaded.
// If the rules to intercept traffic are not loaded, we'll try to insert them again.
func startCheckingRules(exitChan <-chan bool, rulesChecker *time.Ticker, areRulesLoaded callbackBool, reloadRules callback) {
	for {
		select {
		case <-exitChan:
			goto Exit
		case _, active := <-rulesChecker.C:
			if !active {
				goto Exit
			}

			if areRulesLoaded() == false {
				reloadRules()
			}
		}
	}

Exit:
	log.Info("exit checking firewall rules")
}

// StopCheckingRules stops checking if firewall rules are loaded.
func (c *Common) StopCheckingRules() {
	c.Lock()
	defer c.Unlock()

	if c.RulesChecker != nil {
		select {
		case c.stopChecker <- true:
			close(c.stopChecker)
		case <-time.After(5 * time.Millisecond):
			// We should not arrive here
			log.Error("StopCheckingRules: timed out stopping monitor rules")
		}

		c.RulesChecker.Stop()
		c.RulesChecker = nil
	}
}

func (c *Common) reloadCallback(callback func()) {
	callback()
}
