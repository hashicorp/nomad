/* eslint-disable ember/no-observers */
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { observes } from '@ember-decorators/object';
import { computed as overridable } from 'ember-overridable-computed';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';
import Sortable from 'nomad-ui/mixins/sortable';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';
import classic from 'ember-classic-decorator';
import { union } from '@ember/object/computed';

@classic
export default class IndexController extends Controller.extend(Sortable) {
  @service token;
  @service store;

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

  @alias('model.states') listToSort;
  @alias('listSorted') sortedStates;

  // Set in the route
  preempter = null;

  @overridable(function () {
    // { title, description }
    return null;
  })
  error;

  @computed('model.allocatedResources.ports.@each.label')
  get ports() {
    return (this.get('model.allocatedResources.ports') || []).sortBy('label');
  }

  @computed('model.states.@each.task')
  get tasks() {
    return this.get('model.states').mapBy('task') || [];
  }

  @computed('tasks.@each.services')
  get taskServices() {
    return this.get('tasks')
      .map((t) => ((t && t.get('services')) || []).toArray())
      .flat()
      .compact();
  }

  @computed('model.taskGroup.services.@each.name')
  get groupServices() {
    return (this.get('model.taskGroup.services') || []).sortBy('name');
  }

  @union('taskServices', 'groupServices') services;

  @computed('model.healthChecks.{}')
  get serviceHealthStatuses() {
    if (!this.model.healthChecks) return null;

    let result = new Map();
    Object.values(this.model.healthChecks)?.forEach((service) => {
      const isTask = !!service.Task;
      const groupName = service.Group.split('.')[1].split('[')[0];
      const currentServiceStatus = service.Status;

      const currentServiceName = isTask
        ? service.Task.concat(`-${service.Service}`)
        : groupName.concat(`-${service.Service}`);
      const serviceStatuses = result.get(currentServiceName);
      if (serviceStatuses) {
        if (serviceStatuses[currentServiceStatus]) {
          result.set(currentServiceName, {
            ...serviceStatuses,
            [currentServiceStatus]: serviceStatuses[currentServiceStatus]++,
          });
        } else {
          result.set(currentServiceName, {
            ...serviceStatuses,
            [currentServiceStatus]: 1,
          });
        }
      } else {
        result.set(currentServiceName, { [currentServiceStatus]: 1 });
      }
    });

    return result;
  }

  onDismiss() {
    this.set('error', null);
  }

  @watchRecord('allocation') watchNext;

  @observes('model.nextAllocation.clientStatus')
  observeWatchNext() {
    const nextAllocation = this.model.nextAllocation;
    if (nextAllocation && nextAllocation.content) {
      this.watchNext.perform(nextAllocation);
    } else {
      this.watchNext.cancelAll();
    }
  }

  @task(function* () {
    try {
      yield this.model.stop();
      // Eagerly update the allocation clientStatus to avoid flickering
      this.model.set('clientStatus', 'complete');
    } catch (err) {
      this.set('error', {
        title: 'Could Not Stop Allocation',
        description: messageForError(err, 'manage allocation lifecycle'),
      });
    }
  })
  stopAllocation;

  @task(function* () {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Allocation',
        description: messageForError(err, 'manage allocation lifecycle'),
      });
    }
  })
  restartAllocation;

  @action
  gotoTask(allocation, task) {
    this.transitionToRoute('allocations.allocation.task', task);
  }

  @action
  taskClick(allocation, task, event) {
    lazyClick([() => this.send('gotoTask', allocation, task), event]);
  }
}
