/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-incorrect-calls-with-inline-anonymous-functions */
import { inject as service } from '@ember/service';
import { alias, readOnly } from '@ember/object/computed';
import Controller from '@ember/controller';
import { action, computed, get } from '@ember/object';
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
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

@classic
export default class TaskGroupController extends Controller.extend(
  Sortable,
  Searchable,
  WithNamespaceResetting
) {
  @service userSettings;
  @service can;

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
    'activeTask',
  ];

  currentPage = 1;
  @readOnly('userSettings.pageSize') pageSize;

  qpStatus = '';
  qpClient = '';
  sortProperty = 'modifyIndex';
  sortDescending = true;
  activeTask = null;

  @computed
  get searchProps() {
    return ['shortId', 'name'];
  }

  @localStorageProperty('nomadShowSubTasks', true) showSubTasks;

  @action
  toggleShowSubTasks(e) {
    e.preventDefault();
    this.set('showSubTasks', !this.get('showSubTasks'));
  }

  @computed('model.allocations.[]')
  get allocations() {
    return this.get('model.allocations') || [];
  }

  @computed('allocations.[]', 'selectionStatus', 'selectionClient')
  get filteredAllocations() {
    const { selectionStatus, selectionClient } = this;

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

      return true;
    });
  }

  @alias('filteredAllocations') listToSort;
  @alias('listSorted') listToSearch;
  @alias('listSearched') sortedAllocations;

  @selection('qpStatus') selectionStatus;
  @selection('qpClient') selectionClient;

  @computed('model.scaleState.events.@each.time', function () {
    const events = get(this, 'model.scaleState.events');
    if (events) {
      return events.sortBy('time').reverse();
    }
    return [];
  })
  sortedScaleEvents;

  @computed('sortedScaleEvents.@each.hasCount', function () {
    const countEventsCount = this.sortedScaleEvents.filterBy('hasCount').length;
    return (
      countEventsCount > 1 &&
      countEventsCount >= this.sortedScaleEvents.length / 2
    );
  })
  shouldShowScaleEventTimeline;

  @computed('model.job.{namespace,runningDeployment}')
  get tooltipText() {
    if (
      this.can.cannot('scale job', null, {
        namespace: this.model.job.namespace.get('name'),
      })
    )
      return "You aren't allowed to scale task groups";
    if (this.model.job.runningDeployment)
      return 'You cannot scale task groups during a deployment';
    return undefined;
  }

  @action
  gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation.id);
  }

  @action
  scaleTaskGroup(count) {
    return this.model.scale(count);
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

    return clients.sort().map((dc) => ({ key: dc, label: dc }));
  }

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
  }

  get taskGroup() {
    return this.model;
  }

  get breadcrumb() {
    const { job, name } = this.taskGroup;
    return {
      title: 'Task Group',
      label: name,
      args: ['jobs.job.task-group', job, name],
    };
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
