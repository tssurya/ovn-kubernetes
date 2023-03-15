package libovsdbops

import (
	"fmt"

	libovsdbclient "github.com/ovn-org/libovsdb/client"

	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/sbdb"
)

// GetPortBinding looks up a port binding from the cache
func GetPortBinding(sbClient libovsdbclient.Client, portBinding *sbdb.PortBinding) (*sbdb.PortBinding, error) {
	found := []*sbdb.PortBinding{}
	opModel := operationModel{
		Model:          portBinding,
		ModelPredicate: func(item *sbdb.PortBinding) bool { return item.LogicalPort == portBinding.LogicalPort },
		ExistingResult: &found,
		ErrNotFound:    true,
		BulkOp:         false,
	}

	m := newModelClient(sbClient)
	err := m.Lookup(opModel)
	if err != nil {
		return nil, err
	}

	return found[0], nil
}

func UpdatePortBindingSetChassis(sbClient libovsdbclient.Client, portBinding *sbdb.PortBinding, chassis *sbdb.Chassis) error {
	remoteChassis, err := GetChassis(sbClient, chassis)
	if err != nil {
		return fmt.Errorf("failed to get chassis id %s(%s), error: %v", chassis.Name, chassis.Hostname, err)
	}

	portBinding, err = GetPortBinding(sbClient, portBinding)
	if err != nil {
		return err
	}

	portBinding.Chassis = &remoteChassis.UUID

	opModel := operationModel{
		Model:          portBinding,
		OnModelUpdates: []interface{}{&portBinding.Chassis},
		ErrNotFound:    true,
		BulkOp:         false,
	}

	m := newModelClient(sbClient)
	_, err = m.CreateOrUpdate(opModel)
	return err
}

// CreatePortBinding creates port binding.  Used only for testing.
// ovn-northd will create port bindings in real deployments for every
// logical port.
func CreatePortBinding(sbClient libovsdbclient.Client, portBinding *sbdb.PortBinding) error {
	opModel := operationModel{
		Model:          portBinding,
		ModelPredicate: func(item *sbdb.PortBinding) bool { return item.LogicalPort == portBinding.LogicalPort },
		OnModelUpdates: onModelUpdatesAllNonDefault(),
		ErrNotFound:    false,
		BulkOp:         false,
	}

	m := newModelClient(sbClient)
	_, err := m.CreateOrUpdate(opModel)
	return err
}
