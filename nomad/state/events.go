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
		switch change.Table {
		case "acl_token":
			before, ok := change.Before.(*structs.ACLToken)
			if !ok {
				return structs.Event{}, false
			}

			return structs.Event{
				Topic:   structs.TopicACLToken,
				Key:     before.AccessorID,
				Payload: structs.NewACLTokenEvent(before),
			}, true
		case "acl_policy":
			before, ok := change.Before.(*structs.ACLPolicy)
			if !ok {
				return structs.Event{}, false
			}
			return structs.Event{
				Topic: structs.TopicACLPolicy,
				Key:   before.Name,
				Payload: &structs.ACLPolicyEvent{
					ACLPolicy: before,
				},
			}, true
		case TableACLRoles:
			before, ok := change.Before.(*structs.ACLRole)
			if !ok {
				return structs.Event{}, false
			}
			return structs.Event{
				Topic:      structs.TopicACLRole,
				Key:        before.ID,
				FilterKeys: []string{before.Name},
				Payload: &structs.ACLRoleStreamEvent{
					ACLRole: before,
				},
			}, true
		case TableACLAuthMethods:
			before, ok := change.Before.(*structs.ACLAuthMethod)
			if !ok {
				return structs.Event{}, false
			}
			return structs.Event{
				Topic: structs.TopicACLAuthMethod,
				Key:   before.Name,
				Payload: &structs.ACLAuthMethodEvent{
					AuthMethod: before,
				},
			}, true
		case TableACLBindingRules:
			before, ok := change.Before.(*structs.ACLBindingRule)
			if !ok {
				return structs.Event{}, false
			}
			return structs.Event{
				Topic:      structs.TopicACLBindingRule,
				Key:        before.ID,
				FilterKeys: []string{before.AuthMethod},
				Payload: &structs.ACLBindingRuleEvent{
					ACLBindingRule: before,
				},
			}, true
		case "nodes":
			before, ok := change.Before.(*structs.Node)
			if !ok {
				return structs.Event{}, false
			}

			before = before.Sanitize()
			return structs.Event{
				Topic: structs.TopicNode,
				Key:   before.ID,
				Payload: &structs.NodeStreamEvent{
					Node: before,
				},
			}, true
		case TableNodePools:
			before, ok := change.Before.(*structs.NodePool)
			if !ok {
				return structs.Event{}, false
			}
			return structs.Event{
				Topic: structs.TopicNodePool,
				Key:   before.Name,
				Payload: &structs.NodePoolEvent{
					NodePool: before,
				},
			}, true
		case TableServiceRegistrations:
			before, ok := change.Before.(*structs.ServiceRegistration)
			if !ok {
				return structs.Event{}, false
			}
			return structs.Event{
				Topic: structs.TopicService,
				Key:   before.ID,
				FilterKeys: []string{
					before.JobID,
					before.ServiceName,
				},
				Namespace: before.Namespace,
				Payload: &structs.ServiceRegistrationStreamEvent{
					Service: before,
				},
			}, true
		}
		return structs.Event{}, false
	}

	switch change.Table {
	case "acl_token":
		after, ok := change.After.(*structs.ACLToken)
		if !ok {
			return structs.Event{}, false
		}

		return structs.Event{
			Topic:   structs.TopicACLToken,
			Key:     after.AccessorID,
			Payload: structs.NewACLTokenEvent(after),
		}, true
	case "acl_policy":
		after, ok := change.After.(*structs.ACLPolicy)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic: structs.TopicACLPolicy,
			Key:   after.Name,
			Payload: &structs.ACLPolicyEvent{
				ACLPolicy: after,
			},
		}, true
	case TableACLRoles:
		after, ok := change.After.(*structs.ACLRole)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic:      structs.TopicACLRole,
			Key:        after.ID,
			FilterKeys: []string{after.Name},
			Payload: &structs.ACLRoleStreamEvent{
				ACLRole: after,
			},
		}, true
	case TableACLAuthMethods:
		after, ok := change.After.(*structs.ACLAuthMethod)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic: structs.TopicACLAuthMethod,
			Key:   after.Name,
			Payload: &structs.ACLAuthMethodEvent{
				AuthMethod: after,
			},
		}, true
	case TableACLBindingRules:
		after, ok := change.After.(*structs.ACLBindingRule)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic:      structs.TopicACLBindingRule,
			Key:        after.ID,
			FilterKeys: []string{after.AuthMethod},
			Payload: &structs.ACLBindingRuleEvent{
				ACLBindingRule: after,
			},
		}, true
	case "evals":
		after, ok := change.After.(*structs.Evaluation)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic: structs.TopicEvaluation,
			Key:   after.ID,
			FilterKeys: []string{
				after.JobID,
				after.DeploymentID,
			},
			Namespace: after.Namespace,
			Payload: &structs.EvaluationEvent{
				Evaluation: after,
			},
		}, true
	case "allocs":
		after, ok := change.After.(*structs.Allocation)
		if !ok {
			return structs.Event{}, false
		}
		alloc := after.Copy()

		filterKeys := []string{
			alloc.JobID,
			alloc.DeploymentID,
		}

		// remove job info to help keep size of alloc event down
		alloc.Job = nil

		return structs.Event{
			Topic:      structs.TopicAllocation,
			Key:        after.ID,
			FilterKeys: filterKeys,
			Namespace:  after.Namespace,
			Payload: &structs.AllocationEvent{
				Allocation: alloc,
			},
		}, true
	case "jobs":
		after, ok := change.After.(*structs.Job)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic:     structs.TopicJob,
			Key:       after.ID,
			Namespace: after.Namespace,
			Payload: &structs.JobEvent{
				Job: after,
			},
		}, true
	case "nodes":
		after, ok := change.After.(*structs.Node)
		if !ok {
			return structs.Event{}, false
		}

		after = after.Sanitize()
		return structs.Event{
			Topic: structs.TopicNode,
			Key:   after.ID,
			Payload: &structs.NodeStreamEvent{
				Node: after,
			},
		}, true
	case TableNodePools:
		after, ok := change.After.(*structs.NodePool)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic: structs.TopicNodePool,
			Key:   after.Name,
			Payload: &structs.NodePoolEvent{
				NodePool: after,
			},
		}, true
	case "deployment":
		after, ok := change.After.(*structs.Deployment)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic:      structs.TopicDeployment,
			Key:        after.ID,
			Namespace:  after.Namespace,
			FilterKeys: []string{after.JobID},
			Payload: &structs.DeploymentEvent{
				Deployment: after,
			},
		}, true
	case TableServiceRegistrations:
		after, ok := change.After.(*structs.ServiceRegistration)
		if !ok {
			return structs.Event{}, false
		}
		return structs.Event{
			Topic: structs.TopicService,
			Key:   after.ID,
			FilterKeys: []string{
				after.JobID,
				after.ServiceName,
			},
			Namespace: after.Namespace,
			Payload: &structs.ServiceRegistrationStreamEvent{
				Service: after,
			},
		}, true
	}

	return structs.Event{}, false
}
