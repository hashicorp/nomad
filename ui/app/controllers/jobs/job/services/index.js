/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import SortableFactory from 'nomad-ui/mixins/sortable-factory';
import { computed } from '@ember/object';

export default class JobsJobServicesIndexController extends Controller.extend(
  WithNamespaceResetting,
  SortableFactory(['name', 'level']),
) {
  get job() {
    return this.model;
  }

  get taskGroups() {
    return this.job.taskGroups;
  }

  queryParams = [
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  sortProperty = 'name';
  sortDescending = false;

  get listToSort() {
    return this.services;
  }

  get sortedServices() {
    return this.listSorted;
  }

  @computed('taskGroups.@each.tasks')
  get tasks() {
    return this.taskGroups.map((group) => group.tasks.toArray()).flat();
  }

  @computed('tasks.@each.services')
  get taskServices() {
    return this.tasks
      .map((t) => (t.services || []).toArray())
      .flat()
      .compact()
      .map((service) => {
        service.level = 'task';
        return service;
      });
  }

  @computed('model.taskGroup.services.@each.name', 'taskGroups')
  get groupServices() {
    return this.taskGroups
      .map((g) => (g.services || []).toArray())
      .flat()
      .compact()
      .map((service) => {
        service.level = 'group';
        return service;
      });
  }

  get serviceFragments() {
    return [...this.taskServices, ...this.groupServices];
  }

  // Services, grouped by name, with aggregatable allocations.
  @computed(
    'job.services.@each.{name,allocation}',
    'job.services.length',
    'serviceFragments',
  )
  get services() {
    return this.serviceFragments.map((fragment) => {
      fragment.instances = this.job.services.filter(
        (s) => s.name === fragment.name && s.derivedLevel === fragment.level,
      );
      return fragment;
    });
  }
}
