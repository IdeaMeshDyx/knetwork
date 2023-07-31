package iptables

import (
	"github.com/coreos/go-iptables/iptables"
	klog "k8s.io/klog/v2"
)

// rule represents an iptables rule.
type Rule struct {
	table string
	chain string
	spec  []string
}

// iptable Operation include all the operation to interact with iptables
type IptOp interface {
	// insert amount of rules in the table
	Append(rule Rule) error

	// insert single rule into the table
	InsertUnique(rule Rule, pos int) error

	// delete rule from the table
	Delete(rule Rule) error

	// check if the rules are right
	Exists(rule Rule) (bool, error)

	// list all the rules from the chain
	List(table string, chain string) ([]string, error)

	// clear the chain
	ClearChain(table string, chain string) error

	// delete the chain
	DeleteChain(table string, chain string) error

	// create a new chain
	NewChain(table string, chain string) error

	// list all the chains
	ListChains(table string) ([]string, error)
}

// iptable Adapter to control the rules
type Adapter struct {
	// coreos/go-iptables added  contains filtered or unexported fields
	*iptables.IPTables

	// rules need to append
	ruleSet []Rule
}

// init operator to control ipatbles
func New() (IptOp, error) {
	// init ipatbles operator for ipv4 rules
	ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4), iptables.Timeout(5))

	if err != nil {
		klog.Errorf("Failed to create IpOpt : %v", err)
		return nil, err
	}
	klog.Infof("create IpOpt success")

	return &Adapter{ipt}, nil
}

func (adapter *Adapter) Append(rule Rule) error {
	ok, err := adapter.Exists(rule)
	// if there is no such rule then add them
	if err == nil && !ok {
		err = adapter.Append(rule)
	}
	if err != nil {
		klog.ErrorS(err, "error on iptables.Append", "table", rule.table, "chain", rule.chain, "rulespec", rulespec, "exists", exists)
		return err
	}
	if klog.V(5).Enabled() {
		klog.V(5).InfoS("iptables.Append succeeded", "table", table, "chain", chain, "rulespec", rulespec, "exists", exists)
	}
	return nil
}

func (adapter *Adapter) InsertUnique(rule Rule, pos int) error {
	exists, err := adapter.Exists(table, chain, rulespec...)
	if err == nil && !exists {
		err = adapter.Insert(table, chain, pos, rulespec...)
	}
	if err != nil {
		klog.ErrorS(err, "error on iptables.Insert", "table", table, "chain", chain, "pos", pos, "rulespec", rulespec, "exists", exists)
		return err
	}
	if klog.V(5).Enabled() {
		klog.V(5).InfoS("iptables.Insert succeeded", "table", table, "chain", chain, "pos", pos, "rulespec", rulespec, "exists", exists)
	}
	return nil
}

func (adapter *Adapter) Delete(rule Rule) error {
	err := adapter.IPTables.DeleteIfExists(table, chain, rulespec...)
	if err != nil {
		klog.ErrorS(err, "error on iptables.Delete", "table", table, "chain", chain, "rulespec", rulespec)
		return err
	}
	if klog.V(5).Enabled() {
		klog.V(5).InfoS("iptables.Delete succeeded", "table", table, "chain", chain, "rulespec", rulespec)
	}
	return nil
}

func (adapter *Adapter) Exists(rule Rule) (bool, error) { return true, nil }

func (adapter *Adapter) List(table string, chain string) ([]string, error) { return nil, nil }

func (adapter *Adapter) ClearChain(table string, chain string) error { return nil }

func (adapter *Adapter) DeleteChain(table string, chain string) error { return nil }

func (adapter *Adapter) NewChain(table string, chain string) error { return nil }

func (adapter *Adapter) ListChains(table string) ([]string, error) { return nil, nil }
