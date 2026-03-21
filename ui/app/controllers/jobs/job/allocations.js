/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { service } from '@ember/service';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';

export default class AllocationsController extends Controller.extend(
  SortableFactory([
    'modifyIndex',
    'name',
    'shortId',
    'taskGroupName',
    'clientStatus',
    'jobVersion',
  ]),
  Searchable,
  WithNamespaceResetting,
) {
  @service router;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      searchTerm: 'search',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    {
      qpStatus: 'status',
    },
    {
      qpClient: 'client',
    },
    {
      qpTaskGroup: 'taskGroup',
    },
    {
      qpVersion: 'version',
    },
    {
      qpScheduling: 'scheduling',
    },
    'activeTask',
  ];

  qpStatus = '';
  qpClient = '';
  qpTaskGroup = '';
  qpVersion = '';
  qpScheduling = '';
  currentPage = 1;
  pageSize = 25;
  activeTask = null;

  sortProperty = 'modifyIndex';
  sortDescending = true;

  get job() {
    return this.model;
  }

  @computed
  get searchProps() {
    return ['shortId', 'name', 'taskGroupName'];
  }

  @computed('model.allocations.[]')
  get allocations() {
    return this.model.allocations || [];
  }

  @computed(
    'allocations.[]',
    'selectionStatus',
    'selectionClient',
    'selectionTaskGroup',
    'selectionVersion',
    'selectionScheduling',
  )
  get filteredAllocations() {
    const {
      selectionStatus,
      selectionClient,
      selectionTaskGroup,
      selectionVersion,
      selectionScheduling,
    } = this;

    return this.allocations.filter((alloc) => {
      if (
        selectionStatus.length &&
        !selectionStatus.includes(alloc.clientStatus)
      ) {
        return false;
      }
      if (
        selectionClient.length &&
        !selectionClient.includes(this.clientKeyForAllocation(alloc))
      ) {
        return false;
      }
      if (
        selectionTaskGroup.length &&
        !selectionTaskGroup.includes(alloc.taskGroupName)
      ) {
        return false;
      }
      if (
        selectionVersion.length &&
        !selectionVersion.includes(alloc.jobVersion)
      ) {
        return false;
      }

      if (selectionScheduling.length) {
        if (
          selectionScheduling.includes('will-not-reschedule') &&
          !alloc.willNotReschedule
        ) {
          return false;
        }
        if (
          selectionScheduling.includes('will-not-restart') &&
          !alloc.willNotRestart
        ) {
          return false;
        }
        if (
          selectionScheduling.includes('has-been-rescheduled') &&
          !alloc.hasBeenRescheduled
        ) {
          return false;
        }
        if (
          selectionScheduling.includes('has-been-restarted') &&
          !alloc.hasBeenRestarted
        ) {
          return false;
        }
        return true;
      }

      return true;
    });
  }

  get listToSort() {
    return this.filteredAllocations;
  }

  get listToSearch() {
    return this.listSorted;
  }

  get sortedAllocations() {
    return this.listSearched;
  }

  @selection('qpStatus') selectionStatus;
  @selection('qpClient') selectionClient;
  @selection('qpTaskGroup') selectionTaskGroup;
  @selection('qpVersion') selectionVersion;
  @selection('qpScheduling') selectionScheduling;

  clientKeyForAllocation(allocation) {
    return allocation?.nodeID?.split('-')?.[0] || null;
  }

  @action
  gotoAllocation(allocation) {
    this.router.transitionTo('allocations.allocation', allocation.id);
  }

  get optionsAllocationStatus() {
    return [
      { key: 'pending', label: 'Pending' },
      { key: 'running', label: 'Running' },
      { key: 'complete', label: 'Complete' },
      { key: 'failed', label: 'Failed' },
      { key: 'lost', label: 'Lost' },
      { key: 'unknown', label: 'Unknown' },
    ];
  }

  @computed('model.allocations.[]', 'selectionClient')
  get optionsClients() {
    const clients = Array.from(
      new Set(
        this.model.allocations
          .map((allocation) => this.clientKeyForAllocation(allocation))
          .filter(Boolean),
      ),
    ).compact();

    // Update query param when the list of clients changes.
    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      this.qpClient = serialize(intersection(clients, this.selectionClient));
    });

    return clients.sort().map((c) => ({ key: c, label: c }));
  }

  @computed('model.allocations.[]', 'selectionTaskGroup')
  get optionsTaskGroups() {
    const taskGroups = Array.from(
      new Set(this.model.allocations.mapBy('taskGroupName')),
    ).compact();

    // Update query param when the list of task groups changes.
    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      this.qpTaskGroup = serialize(
        intersection(taskGroups, this.selectionTaskGroup),
      );
    });

    return taskGroups.sort().map((tg) => ({ key: tg, label: tg }));
  }

  @computed('model.allocations.[]', 'selectionVersion')
  get optionsVersions() {
    const versions = Array.from(
      new Set(this.model.allocations.mapBy('jobVersion')),
    ).compact();

    // Update query param when the list of versions changes.
    scheduleOnce('actions', this, () => {
      // eslint-disable-next-line ember/no-side-effects
      this.qpVersion = serialize(intersection(versions, this.selectionVersion));
    });

    return versions.sort((a, b) => a - b).map((v) => ({ key: v, label: v }));
  }

  @computed('model.allocations.[]', 'selectionScheduling')
  get optionsScheduling() {
    return [
      {
        key: 'has-been-rescheduled',
        label: 'Failed & Has Been Rescheduled',
      },
      {
        key: 'will-not-reschedule',
        label: "Failed & Won't Reschedule",
      },
      {
        key: 'has-been-restarted',
        label: 'Has Been Restarted',
      },
      {
        key: 'will-not-restart',
        label: "Won't Restart",
      },
    ];
  }

  @action
  setFacetQueryParam(queryParam, selection) {
    this[queryParam] = serialize(selection);
  }

  @action
  setActiveTaskQueryParam(task) {
    if (task) {
      this.activeTask = `${task.allocation.id}-${task.name}`;
    } else {
      this.activeTask = null;
    }
  }

  @action
  updateSearchTerm(searchTerm) {
    this.searchTerm = searchTerm;
    this.resetPagination();
  }
}
