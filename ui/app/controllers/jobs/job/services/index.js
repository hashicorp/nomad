/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import Sortable from 'nomad-ui/mixins/sortable';
import { alias } from '@ember/object/computed';
import { computed } from '@ember/object';
import { union } from '@ember/object/computed';

export default class JobsJobServicesIndexController extends Controller.extend(
  WithNamespaceResetting,
  Sortable
) {
  @alias('model') job;
  @alias('job.taskGroups') taskGroups;

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

  @alias('services') listToSort;
  @alias('listSorted') sortedServices;

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

  @union('taskServices', 'groupServices') serviceFragments;

  // Services, grouped by name, with aggregatable allocations.
  @computed(
    'job.services.@each.{name,allocation}',
    'job.services.length',
    'serviceFragments'
  )
  get services() {
    return this.serviceFragments.map((fragment) => {
      fragment.instances = this.job.services.filter(
        (s) => s.name === fragment.name && s.derivedLevel === fragment.level
      );
      return fragment;
    });
  }
}
