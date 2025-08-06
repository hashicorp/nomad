// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"strconv"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_UpsertServiceRegistrations(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// SubTest Marker: This ensures new service registrations are inserted as
	// expected with their correct indexes, along with an update to the index
	// table.
	services := mock.ServiceRegistrations()
	insertIndex := uint64(20)

	// Perform the initial upsert of service registrations.
	err := testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, insertIndex, services)
	must.NoError(t, err)

	// Check that the index for the table was modified as expected.
	initialIndex, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, insertIndex, initialIndex)

	// List all the service registrations in the table, so we can perform a
	// number of tests on the return array.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	// Count how many table entries we have, to ensure it is the expected
	// number.
	var count int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, insertIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, insertIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))
	}
	must.Eq(t, 2, count, must.Sprint("incorrect number of service registrations found"))

	// SubTest Marker: This section attempts to upsert the exact same service
	// registrations without any modification. In this case, the index table
	// should not be updated, indicating no write actually happened due to
	// equality checking.
	reInsertIndex := uint64(30)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, reInsertIndex, services))
	reInsertActualIndex, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, insertIndex, reInsertActualIndex, must.Sprint("index should not have changed"))

	// SubTest Marker: This section modifies a single one of the previously
	// inserted service registrations and performs an upsert. This ensures the
	// index table is modified correctly and that each service registration is
	// updated, or not, as expected.
	service1Update := services[0].Copy()
	service1Update.Tags = []string{"modified"}
	services1Update := []*structs.ServiceRegistration{service1Update}

	update1Index := uint64(40)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, update1Index, services1Update))

	// Check that the index for the table was modified as expected.
	updateActualIndex, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, update1Index, updateActualIndex, must.Sprint("index should have changed"))

	// Get the service registrations from the table.
	iter, err = testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	// Iterate all the stored registrations and assert they are as expected.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		serviceReg := raw.(*structs.ServiceRegistration)

		var expectedModifyIndex uint64

		switch serviceReg.ID {
		case service1Update.ID:
			expectedModifyIndex = update1Index
		case services[1].ID:
			expectedModifyIndex = insertIndex
		default:
			t.Errorf("unknown service registration found: %s", serviceReg.ID)
			continue
		}
		must.Eq(t, insertIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, expectedModifyIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))
	}

	// SubTest Marker: Here we modify the second registration but send an
	// upsert request that includes this and the already modified registration.
	service2Update := services[1].Copy()
	service2Update.Tags = []string{"modified"}
	services2Update := []*structs.ServiceRegistration{service1Update, service2Update}

	update2Index := uint64(50)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, update2Index, services2Update))

	// Check that the index for the table was modified as expected.
	update2ActualIndex, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, update2Index, update2ActualIndex, must.Sprint("index should have changed"))

	// Get the service registrations from the table.
	iter, err = testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	// Iterate all the stored registrations and assert they are as expected.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		serviceReg := raw.(*structs.ServiceRegistration)

		var (
			expectedModifyIndex uint64
			expectedServiceReg  *structs.ServiceRegistration
		)

		switch serviceReg.ID {
		case service2Update.ID:
			expectedModifyIndex = update2Index
			expectedServiceReg = service2Update
		case service1Update.ID:
			expectedModifyIndex = update1Index
			expectedServiceReg = service1Update
		default:
			t.Errorf("unknown service registration found: %s", serviceReg.ID)
			continue
		}

		must.Eq(t, insertIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, expectedModifyIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))
		must.True(t, expectedServiceReg.Equal(serviceReg))
	}
}

func TestStateStore_DeleteServiceRegistrationByID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services that we will use and modify throughout.
	services := mock.ServiceRegistrations()

	// SubTest Marker: This section attempts to delete a service registration
	// by an ID that does not exist. This is easy to perform here as the state
	// is empty.
	initialIndex := uint64(10)
	err := testState.DeleteServiceRegistrationByID(
		structs.MsgTypeTestSetup, initialIndex, services[0].Namespace, services[0].ID)
	must.EqError(t, err, "service registration not found")

	actualInitialIndex, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, uint64(0), actualInitialIndex, must.Sprint("index should not have changed"))

	// SubTest Marker: This section upserts two registrations, deletes one,
	// then ensure the remaining is left as expected.
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	// Perform the delete.
	delete1Index := uint64(20)
	must.NoError(t, testState.DeleteServiceRegistrationByID(
		structs.MsgTypeTestSetup, delete1Index, services[0].Namespace, services[0].ID))

	// Check that the index for the table was modified as expected.
	actualDelete1Index, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, delete1Index, actualDelete1Index, must.Sprint("index should have changed"))

	ws := memdb.NewWatchSet()

	// Get the service registrations from the table.
	iter, err := testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	var delete1Count int

	// Iterate all the stored registrations and assert we have the expected
	// number.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		delete1Count++
	}
	must.Eq(t, 1, delete1Count, must.Sprint("unexpected number of registrations in table"))

	// SubTest Marker: Delete the remaining registration and ensure all indexes
	// are updated as expected and the table is empty.
	delete2Index := uint64(30)
	must.NoError(t, testState.DeleteServiceRegistrationByID(
		structs.MsgTypeTestSetup, delete2Index, services[1].Namespace, services[1].ID))

	// Check that the index for the table was modified as expected.
	actualDelete2Index, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, delete2Index, actualDelete2Index, must.Sprint("index should have changed"))

	// Get the service registrations from the table.
	iter, err = testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	var delete2Count int

	// Iterate all the stored registrations and assert we have the expected
	// number.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		delete2Count++
	}
	must.Eq(t, 0, delete2Count, must.Sprint("unexpected number of registrations in table"))
}

func TestStateStore_DeleteServiceRegistrationByNodeID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services that we will use and modify throughout.
	services := mock.ServiceRegistrations()

	// SubTest Marker: This section attempts to delete a service registration
	// by a nodeID that does not exist. This is easy to perform here as the
	// state is empty.
	initialIndex := uint64(10)
	must.NoError(t, testState.DeleteServiceRegistrationByNodeID(structs.MsgTypeTestSetup, initialIndex, services[0].NodeID))

	actualInitialIndex, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, uint64(0), actualInitialIndex, must.Sprint("index should not have changed"))

	// SubTest Marker: This section upserts two registrations then deletes one
	// by using the nodeID.
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	// Perform the delete.
	delete1Index := uint64(20)
	must.NoError(t, testState.DeleteServiceRegistrationByNodeID(
		structs.MsgTypeTestSetup, delete1Index, services[0].NodeID))

	// Check that the index for the table was modified as expected.
	actualDelete1Index, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, delete1Index, actualDelete1Index, must.Sprint("index should have changed"))

	ws := memdb.NewWatchSet()

	// Get the service registrations from the table.
	iter, err := testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	var delete1Count int

	// Iterate all the stored registrations and assert we have the expected
	// number.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		delete1Count++
	}
	must.Eq(t, 1, delete1Count, must.Sprint("unexpected number of registrations in table"))

	// SubTest Marker: Add multiple service registrations for a single nodeID
	// then delete these via the nodeID.
	delete2NodeID := services[1].NodeID
	var delete2NodeServices []*structs.ServiceRegistration

	for i := 0; i < 4; i++ {
		iString := strconv.Itoa(i)
		delete2NodeServices = append(delete2NodeServices, &structs.ServiceRegistration{
			ID:          "_nomad-task-ca60e901-675a-0ab2-2e57-2f3b05fdc540-group-api-countdash-api-http-" + iString,
			ServiceName: "countdash-api-" + iString,
			Namespace:   "platform",
			NodeID:      delete2NodeID,
			Datacenter:  "dc2",
			JobID:       "countdash-api-" + iString,
			AllocID:     "ca60e901-675a-0ab2-2e57-2f3b05fdc54" + iString,
			Tags:        []string{"bar"},
			Address:     "192.168.200.200",
			Port:        27500 + i,
		})
	}

	// Upsert the new service registrations.
	delete2UpsertIndex := uint64(30)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, delete2UpsertIndex, delete2NodeServices))

	delete2Index := uint64(40)
	must.NoError(t, testState.DeleteServiceRegistrationByNodeID(
		structs.MsgTypeTestSetup, delete2Index, delete2NodeID))

	// Check that the index for the table was modified as expected.
	actualDelete2Index, err := testState.Index(TableServiceRegistrations)
	must.NoError(t, err)
	must.Eq(t, delete2Index, actualDelete2Index, must.Sprint("index should have changed"))

	// Get the service registrations from the table.
	iter, err = testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	var delete2Count int

	// Iterate all the stored registrations and assert we have the expected
	// number.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		delete2Count++
	}
	must.Eq(t, 0, delete2Count, must.Sprint("unexpected number of registrations in table"))
}

func TestStateStore_GetServiceRegistrations(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services and upsert them.
	services := mock.ServiceRegistrations()
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	// Read the service registrations and check the objects.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetServiceRegistrations(ws)
	must.NoError(t, err)

	var count int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		serviceReg := raw.(*structs.ServiceRegistration)

		must.Eq(t, initialIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, initialIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))

		switch serviceReg.ID {
		case services[0].ID:
			must.Eq(t, services[0], serviceReg)
		case services[1].ID:
			must.Eq(t, services[1], serviceReg)
		default:
			t.Errorf("unknown service registration found: %s", serviceReg.ID)
		}
	}
	must.Eq(t, 2, count)
}

func TestStateStore_GetServiceRegistrationsByNamespace(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services and upsert them.
	services := mock.ServiceRegistrations()
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	// Look up services using the namespace of the first service.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetServiceRegistrationsByNamespace(ws, services[0].Namespace)
	must.NoError(t, err)

	var count1 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count1++
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, initialIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, initialIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))
		must.Eq(t, services[0].Namespace, serviceReg.Namespace)
	}
	must.Eq(t, 1, count1)

	// Look up services using the namespace of the second service.
	iter, err = testState.GetServiceRegistrationsByNamespace(ws, services[1].Namespace)
	must.NoError(t, err)

	var count2 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count2++
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, initialIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, initialIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))
		must.Eq(t, services[1].Namespace, serviceReg.Namespace)
	}
	must.Eq(t, 1, count2)

	// Look up services using a namespace that shouldn't contain any
	// registrations.
	iter, err = testState.GetServiceRegistrationsByNamespace(ws, "pony-club")
	must.NoError(t, err)

	var count3 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count3++
	}
	must.Eq(t, 0, count3)
}

func TestStateStore_GetServiceRegistrationByName(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services and upsert them.
	services := mock.ServiceRegistrations()
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	// Try reading a service by a name that shouldn't exist.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetServiceRegistrationByName(ws, "default", "pony-glitter-api")
	must.NoError(t, err)

	var count1 int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count1++
	}
	must.Eq(t, 0, count1)

	// Read one of the known service registrations.
	expectedReg := services[1].Copy()

	iter, err = testState.GetServiceRegistrationByName(ws, expectedReg.Namespace, expectedReg.ServiceName)
	must.NoError(t, err)

	var count2 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count2++
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, expectedReg.ServiceName, serviceReg.ServiceName)
		must.Eq(t, expectedReg.Namespace, serviceReg.Namespace)
	}
	must.Eq(t, 1, count2)

	// Create a bunch of additional services whose name and namespace match
	// that of expectedReg.
	var newServices []*structs.ServiceRegistration

	for i := 0; i < 4; i++ {
		iString := strconv.Itoa(i)
		newServices = append(newServices, &structs.ServiceRegistration{
			ID:          "_nomad-task-ca60e901-675a-0ab2-2e57-2f3b05fdc540-group-api-countdash-api-http-" + iString,
			ServiceName: expectedReg.ServiceName,
			Namespace:   expectedReg.Namespace,
			NodeID:      "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
			Datacenter:  "dc1",
			JobID:       expectedReg.JobID,
			AllocID:     "ca60e901-675a-0ab2-2e57-2f3b05fdc54" + iString,
			Tags:        []string{"bar"},
			Address:     "192.168.200.200",
			Port:        27500 + i,
		})
	}

	updateIndex := uint64(20)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, updateIndex, newServices))

	iter, err = testState.GetServiceRegistrationByName(ws, expectedReg.Namespace, expectedReg.ServiceName)
	must.NoError(t, err)

	var count3 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count3++
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, expectedReg.ServiceName, serviceReg.ServiceName)
		must.Eq(t, expectedReg.Namespace, serviceReg.Namespace)
	}
	must.Eq(t, 5, count3)
}

func TestStateStore_GetServiceRegistrationByID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services and upsert them.
	services := mock.ServiceRegistrations()
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	ws := memdb.NewWatchSet()

	// Try reading a service by an ID that shouldn't exist.
	serviceReg, err := testState.GetServiceRegistrationByID(ws, "default", "pony-glitter-sparkles")
	must.NoError(t, err)
	must.Nil(t, serviceReg)

	// Read the two services that we should find.
	serviceReg, err = testState.GetServiceRegistrationByID(ws, services[0].Namespace, services[0].ID)
	must.NoError(t, err)
	must.Eq(t, services[0], serviceReg)

	serviceReg, err = testState.GetServiceRegistrationByID(ws, services[1].Namespace, services[1].ID)
	must.NoError(t, err)
	must.Eq(t, services[1], serviceReg)
}

func TestStateStore_GetServiceRegistrationsByAllocID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services and upsert them.
	services := mock.ServiceRegistrations()
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	ws := memdb.NewWatchSet()

	// Try reading services by an allocation that doesn't have any
	// registrations.
	iter, err := testState.GetServiceRegistrationsByAllocID(ws, "4eed3c6d-6bf1-60d6-040a-e347accae6c4")
	must.NoError(t, err)

	var count1 int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count1++
	}
	must.Eq(t, 0, count1)

	// Read the two allocations that we should find.
	iter, err = testState.GetServiceRegistrationsByAllocID(ws, services[0].AllocID)
	must.NoError(t, err)

	var count2 int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count2++
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, services[0].AllocID, serviceReg.AllocID)
	}
	must.Eq(t, 1, count2)

	iter, err = testState.GetServiceRegistrationsByAllocID(ws, services[1].AllocID)
	must.NoError(t, err)

	var count3 int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count3++
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, services[1].AllocID, serviceReg.AllocID)
	}
	must.Eq(t, 1, count3)
}

func TestStateStore_GetServiceRegistrationsByJobID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services and upsert them.
	services := mock.ServiceRegistrations()
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	ws := memdb.NewWatchSet()

	// Perform a query against a job that shouldn't have any registrations.
	iter, err := testState.GetServiceRegistrationsByJobID(ws, "default", "tamagotchi")
	must.NoError(t, err)

	var count1 int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count1++
	}
	must.Eq(t, 0, count1)

	// Look up services using the namespace and jobID of the first service.
	iter, err = testState.GetServiceRegistrationsByJobID(ws, services[0].Namespace, services[0].JobID)
	must.NoError(t, err)

	var outputList1 []*structs.ServiceRegistration

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, initialIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, initialIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))
		outputList1 = append(outputList1, serviceReg)
	}
	must.SliceContainsAll(t, outputList1, []*structs.ServiceRegistration{services[0]})

	// Look up services using the namespace and jobID of the second service.
	iter, err = testState.GetServiceRegistrationsByJobID(ws, services[1].Namespace, services[1].JobID)
	must.NoError(t, err)

	var outputList2 []*structs.ServiceRegistration

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		serviceReg := raw.(*structs.ServiceRegistration)
		must.Eq(t, initialIndex, serviceReg.CreateIndex,
			must.Sprintf("incorrect create index for %s", serviceReg.ID))
		must.Eq(t, initialIndex, serviceReg.ModifyIndex,
			must.Sprintf("incorrect modify index for %s", serviceReg.ID))
		outputList2 = append(outputList2, serviceReg)
	}
	must.SliceContainsAll(t, outputList2, []*structs.ServiceRegistration{services[1]})
}

func TestStateStore_GetServiceRegistrationsByNodeID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test services and upsert them.
	services := mock.ServiceRegistrations()
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertServiceRegistrations(structs.MsgTypeTestSetup, initialIndex, services))

	ws := memdb.NewWatchSet()

	// Perform a query against a node that shouldn't have any registrations.
	serviceRegs, err := testState.GetServiceRegistrationsByNodeID(ws, "4eed3c6d-6bf1-60d6-040a-e347accae6c4")
	must.NoError(t, err)
	must.Len(t, 0, serviceRegs)

	// Read the two nodes that we should find entries for.
	serviceRegs, err = testState.GetServiceRegistrationsByNodeID(ws, services[0].NodeID)
	must.NoError(t, err)
	must.Len(t, 1, serviceRegs)

	serviceRegs, err = testState.GetServiceRegistrationsByNodeID(ws, services[1].NodeID)
	must.NoError(t, err)
	must.Len(t, 1, serviceRegs)
}

func TestAlloc_ServiceRegistrationLifecycle(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)
	index, _ := store.LatestIndex()

	alloc0 := mock.Alloc()
	alloc1 := mock.Alloc()

	services := mock.ServiceRegistrations()
	services[0].AllocID = alloc0.ID
	services[1].AllocID = alloc1.ID

	node := mock.Node()
	node.ID = services[0].NodeID
	alloc0.NodeID = node.ID

	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, node))

	index++
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index,
		[]*structs.Allocation{alloc0, alloc1}))

	index++
	must.NoError(t, store.UpsertServiceRegistrations(
		structs.MsgTypeTestSetup, index, services))

	// node gets marked lost, but this doesn't delete services
	node = node.Copy()
	node.Status = structs.NodeStatusDown
	services, err := store.GetServiceRegistrationsByNodeID(nil, node.ID)
	must.NoError(t, err)
	must.Len(t, 1, services)

	// client marks alloc complete, so we clear the service
	alloc0 = alloc0.Copy()
	alloc0.ClientStatus = structs.AllocClientStatusComplete
	index++
	must.NoError(t, store.UpdateAllocsFromClient(structs.MsgTypeTestSetup, index,
		[]*structs.Allocation{alloc0}))

	iter, err := store.GetServiceRegistrationsByAllocID(nil, alloc0.ID)
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	// scheduler/plan marks alloc lost, so we clear the service
	planAlloc := new(structs.Allocation)
	*planAlloc = *alloc1
	planAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	planAlloc.ClientStatus = structs.AllocClientStatusLost
	diff := structs.AllocationDiff(*planAlloc)

	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocUpdateRequest: structs.AllocUpdateRequest{
				AllocsStopped: []*structs.AllocationDiff{&diff},
			},
		}))

	iter, err = store.GetServiceRegistrationsByAllocID(nil, alloc1.ID)
	must.NoError(t, err)
	must.Nil(t, iter.Next())

}
