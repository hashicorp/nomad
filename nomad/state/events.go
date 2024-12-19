// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

var MsgTypeEvents = map[structs.MessageType]string{
	structs.NodeRegisterRequestType:                      structs.TypeNodeRegistration,
	structs.NodeDeregisterRequestType:                    structs.TypeNodeDeregistration,
	structs.UpsertNodeEventsType:                         structs.TypeNodeEvent,
	structs.NodePoolUpsertRequestType:                    structs.TypeNodePoolUpserted,
	structs.NodePoolDeleteRequestType:                    structs.TypeNodePoolDeleted,
	structs.EvalUpdateRequestType:                        structs.TypeEvalUpdated,
	structs.AllocClientUpdateRequestType:                 structs.TypeAllocationUpdated,
	structs.JobRegisterRequestType:                       structs.TypeJobRegistered,
	structs.NodeUpdateStatusRequestType:                  structs.TypeNodeEvent,
	structs.JobDeregisterRequestType:                     structs.TypeJobDeregistered,
	structs.JobBatchDeregisterRequestType:                structs.TypeJobBatchDeregistered,
	structs.AllocUpdateDesiredTransitionRequestType:      structs.TypeAllocationUpdateDesiredStatus,
	structs.NodeUpdateEligibilityRequestType:             structs.TypeNodeDrain,
	structs.NodeUpdateDrainRequestType:                   structs.TypeNodeDrain,
	structs.BatchNodeUpdateDrainRequestType:              structs.TypeNodeDrain,
	structs.DeploymentStatusUpdateRequestType:            structs.TypeDeploymentUpdate,
	structs.DeploymentPromoteRequestType:                 structs.TypeDeploymentPromotion,
	structs.DeploymentAllocHealthRequestType:             structs.TypeDeploymentAllocHealth,
	structs.ApplyPlanResultsRequestType:                  structs.TypePlanResult,
	structs.ACLTokenDeleteRequestType:                    structs.TypeACLTokenDeleted,
	structs.ACLTokenUpsertRequestType:                    structs.TypeACLTokenUpserted,
	structs.ACLPolicyDeleteRequestType:                   structs.TypeACLPolicyDeleted,
	structs.ACLPolicyUpsertRequestType:                   structs.TypeACLPolicyUpserted,
	structs.ACLRolesDeleteByIDRequestType:                structs.TypeACLRoleDeleted,
	structs.ACLRolesUpsertRequestType:                    structs.TypeACLRoleUpserted,
	structs.ACLAuthMethodsUpsertRequestType:              structs.TypeACLAuthMethodUpserted,
	structs.ACLAuthMethodsDeleteRequestType:              structs.TypeACLAuthMethodDeleted,
	structs.ACLBindingRulesUpsertRequestType:             structs.TypeACLBindingRuleUpserted,
	structs.ACLBindingRulesDeleteRequestType:             structs.TypeACLBindingRuleDeleted,
	structs.ServiceRegistrationUpsertRequestType:         structs.TypeServiceRegistration,
	structs.ServiceRegistrationDeleteByIDRequestType:     structs.TypeServiceDeregistration,
	structs.ServiceRegistrationDeleteByNodeIDRequestType: structs.TypeServiceDeregistration,
	structs.HostVolumeRegisterRequestType:                structs.TypeHostVolumeRegistered,
	structs.HostVolumeDeleteRequestType:                  structs.TypeHostVolumeDeleted,
	structs.CSIVolumeRegisterRequestType:                 structs.TypeCSIVolumeRegistered,
	structs.CSIVolumeDeregisterRequestType:               structs.TypeCSIVolumeDeregistered,
	structs.CSIVolumeClaimRequestType:                    structs.TypeCSIVolumeClaim,
}

func eventsFromChanges(tx ReadTxn, changes Changes) *structs.Events {
	eventType, ok := MsgTypeEvents[changes.MsgType]
	if !ok {
		return nil
	}

	var events []structs.Event
	for _, change := range changes.Changes {
		if event, ok := eventFromChange(change); ok {
			event.Type = eventType
			event.Index = changes.Index
			events = append(events, event)
		}
	}

	return &structs.Events{Index: changes.Index, Events: events}
}

func eventFromChange(change memdb.Change) (structs.Event, bool) {
	if change.Deleted() {
		// we don't emit events for all Eventers on delete, so we need to make
		// sure we're only emitting events for the tables we want
		switch change.Table {
		case TableACLAuthMethods,
			TableACLBindingRules,
			TableACLPolicies,
			TableACLRoles,
			TableACLTokens,
			TableCSIPlugins,
			TableCSIVolumes,
			TableHostVolumes,
			TableJobs,
			TableNodePools,
			TableNodes,
			TableServiceRegistrations:
			before, ok := change.Before.(structs.Eventer)
			if !ok {
				return structs.Event{}, false
			}
			return before.Event(), true
		default:
			return structs.Event{}, false
		}
	}

	// we don't emit events for all Eventers on update (ex. the Job and
	// JobVersion table have the same Job object), so we need to make sure
	// we're only emitting events for the tables we want
	switch change.Table {
	case TableACLAuthMethods,
		TableACLBindingRules,
		TableACLPolicies,
		TableACLRoles,
		TableACLTokens,
		TableAllocs,
		TableCSIPlugins,
		TableCSIVolumes,
		TableDeployments,
		TableEvals,
		TableHostVolumes,
		TableJobs,
		TableNodePools,
		TableNodes,
		TableServiceRegistrations:
		after, ok := change.After.(structs.Eventer)
		if !ok {
			return structs.Event{}, false
		}
		return after.Event(), true
	default:
		return structs.Event{}, false
	}
}
