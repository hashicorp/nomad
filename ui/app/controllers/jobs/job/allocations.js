/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import intersection from 'lodash.intersection';
import Sortable from 'nomad-ui/mixins/sortable';
import Searchable from 'nomad-ui/mixins/searchable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import {
  serialize,
  deserializedQueryParam as selection,
} from 'nomad-ui/utils/qp-serialize';
import classic from 'ember-classic-decorator';

@classic
export default class AllocationsController extends Controller.extend(
  Sortable,
  Searchable,
  WithNamespaceResetting
) {
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

  @alias('model') job;

  @computed
  get searchProps() {
    return ['shortId', 'name', 'taskGroupName'];
  }

  @computed('model.allocations.[]')
  get allocations() {
    return this.get('model.allocations') || [];
  }

  @computed(
    'allocations.[]',
    'selectionStatus',
    'selectionClient',
    'selectionTaskGroup',
    'selectionVersion',
    'selectionScheduling'
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
        !selectionClient.includes(alloc.get('node.shortId'))
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

  @alias('filteredAllocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @selection('qpStatus') selectionStatus;
  @selection('qpClient') selectionClient;
  @selection('qpTaskGroup') selectionTaskGroup;
  @selection('qpVersion') selectionVersion;
  @selection('qpScheduling') selectionScheduling;

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation.id);
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
      new Set(this.model.allocations.mapBy('node.shortId'))
    ).compact();

    // Update query param when the list of clients changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpClient',
        serialize(intersection(clients, this.selectionClient))
      );
    });

    return clients.sort().map((c) => ({ key: c, label: c }));
  }

  @computed('model.allocations.[]', 'selectionTaskGroup')
  get optionsTaskGroups() {
    const taskGroups = Array.from(
      new Set(this.model.allocations.mapBy('taskGroupName'))
    ).compact();

    // Update query param when the list of task groups changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpTaskGroup',
        serialize(intersection(taskGroups, this.selectionTaskGroup))
      );
    });

    return taskGroups.sort().map((tg) => ({ key: tg, label: tg }));
  }

  @computed('model.allocations.[]', 'selectionVersion')
  get optionsVersions() {
    const versions = Array.from(
      new Set(this.model.allocations.mapBy('jobVersion'))
    ).compact();

    // Update query param when the list of versions changes.
    scheduleOnce('actions', () => {
      // eslint-disable-next-line ember/no-side-effects
      this.set(
        'qpVersion',
        serialize(intersection(versions, this.selectionVersion))
      );
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

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }

  @action
  setActiveTaskQueryParam(task) {
    if (task) {
      this.set('activeTask', `${task.allocation.id}-${task.name}`);
    } else {
      this.set('activeTask', null);
    }
  }
}
