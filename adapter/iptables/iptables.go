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
	// insert rules in the table
	Append(rule Rule) error

	// insert single rule into the position of the table
	InsertUnique(rule Rule, pos int) error

	// delete rule from the table
	Delete(rule Rule) error

	// check if the rules are right
	Exists(rule Rule) (bool, error)

	// list all the rules from the chain
	List(table string, chain string) ([]string, error)

	// clear the chain
	ClearChain(table string, chain string) error

	// create a new chain
	NewChain(table string, chain string) error
}

// iptable Iptutil to control the rules
type Iptutil struct {
	// coreos/go-iptables added  contains filtered or unexported fields
	ipt *iptables.IPTables

	// rules need to append, and it should be a queue to store the respec
	ruleSet []Rule
}

// init operator to control ipatbles
func New() (IptOp, error) {
	// init ipatbles operator for ipv4 rules
	// TODO: add IPV6 support
	ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4), iptables.Timeout(5))

	if err != nil {
		klog.Errorf("Failed to create iptables operator : %v", err)
		return nil, err
	}
	klog.Infof("create iptables operator success")

	return &Iptutil{ipt}, nil
}

func (iptutil *Iptutil) Append(rule Rule) error {
	// check if the rule is in the table
	ok, err := iptutil.Exists(rule)
	// if there is no such rule then add it
	if err == nil && !ok {
		err = iptutil.ipt.AppendUnique(rule)
		klog.Infof("this rule is not exist, start to insert it")
	}
	if err != nil {
		klog.Errorf("error on iptables.AppendUnique in table %v on chain %v for rule %v, err ", rule.table, rule.chain, rule.spec, err)
		return err
	}
	klog.Infof("iptables.AppendUnique succeeded in table %v on chain %v for rule %v", rule.table, rule.chain, rule.spec)

	return nil
}

func (iptutil *Iptutil) InsertUnique(rule Rule, pos int) error {
	exists, err := iptutil.Exists(rule)
	// if there is no such rule then add it in the special position
	if err == nil && !exists {
		err = iptutil.ipt.Insert(rule.table, rule.chain, pos, rule.spec)
		klog.Infof("this rule is not exist, start to insert it on the position %v", pos)
	}
	if err != nil {
		klog.Errorf("error on iptables.AppendUnique in table %v on chain %v  at position %v for rule %v, err ", rule.table, rule.chain, pos, rule.spec, err)
		return err
	}
	klog.Infof("iptables.AppendUnique succeeded in table %v on chain %v at position %v for rule %v", rule.table, rule.chain, pos, rule.spec)
	return nil
}

func (iptutil *Iptutil) Delete(rule Rule) error {
	// delete the rule
	err := iptutil.ipt.DeleteIfExists(rule.table, rule.chain, rule.spec)
	if err != nil {
		klog.Errorf("error on iptables.Delete in table %v on chain %v for rule %v, err ", rule.table, rule.chain, rule.spec, err)
		return err
	}
	klog.Infof("iptables.Delete succeeded in table %v on chain %v for rule %v", rule.table, rule.chain, rule.spec)
	return nil
}

func (iptutil *Iptutil) Exists(rule Rule) (bool, error) {
	// check if the rule exist in the table
	exists, err := iptutil.ipt.Exists(rule.table, rule.chain, rule.spec)
	if err == nil && !exists {
		klog.Infof("rule is not exist in table %v on chain %v", rule.table, rule.chain, rule.spec)
		return false, nil
	}
	if err != nil {
		klog.Errorf("error on iptables.Exists in table %v on chain %v for rule %v, err ", rule.table, rule.chain, rule.spec, err)
		return false, err
	}
	klog.Infof("iptables rules exist in table %v on chain %v", rule.table, rule.chain)
	return true, nil
}

func (iptutil *Iptutil) List(table string, chain string) ([]string, error) {
	// get the rule from the table/chain
	rules, err := iptutil.ipt.List(table, chain)
	if err != nil {
		klog.Errorf("error on iptables.List in table %v on chain %v  err ", table, chain, err)
		return nil, err
	}
	klog.Infof("iptables.List succeeded, in table %v on chain %v get rules %v", table, chain, rules)
	return rules, nil
}

func (iptutil *Iptutil) ClearChain(table string, chain string) error {
	// clear the chain
	err := iptutil.ipt.ClearAndDeleteChain(table, chain)
	if err != nil {
		klog.Errorf("error on iptables.ClearAndDeleteChain in table %v on chain %v  err ", table, chain, err)
		return nil, err
	}
	klog.Infof("iptables.ClearAndDeleteChain succeeded, in table %v on chain %v ", table, chain)
	return nil
}

func (iptutil *Iptutil) NewChain(table string, chain string) error {
	// check if the chain exist
	exists, err := iptutil.ipt.ChainExists(table, chain)
	if err == nil && !exists {
		err = iptutil.ipt.NewChain(table, chain)
		klog.Infof("there is no such chain existv, now start to Newchain")
	}
	if err != nil {
		klog.Errorf("error on iptables.NewChain in table %v for chain %v  err ", table, chain, err)
		return err
	}
	klog.Infof("iptables.NewChain succeeded, in table %v for chain %v ", table, chain)
	return nil
}
