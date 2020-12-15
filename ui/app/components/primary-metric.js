import Ember from 'ember';
import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { task, timeout } from 'ember-concurrency';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('primary-metric')
export default class PrimaryMetric extends Component {
  @service token;
  @service('stats-trackers-registry') statsTrackersRegistry;

  // One of Node, Allocation, or TaskState
  resource = null;

  // cpu or memory
  metric = null;

  'data-test-primary-metric' = true;

  // An instance of a StatsTracker. An alternative interface to resource
  @computed('trackedResource', 'type')
  get tracker() {
    const resource = this.trackedResource;
    return this.statsTrackersRegistry.getTracker(resource);
  }

  @computed('resource')
  get type() {
    const resource = this.resource;
    return resource && resource.constructor.modelName;
  }

  @computed('resource.allocation', 'type')
  get trackedResource() {
    // TaskStates use the allocation stats tracker
    return this.type === 'task-state' ? this.get('resource.allocation') : this.resource;
  }

  @computed('metric')
  get metricLabel() {
    const metric = this.metric;
    const mappings = {
      cpu: 'CPU',
      memory: 'Memory',
    };
    return mappings[metric] || metric;
  }

  @computed('metric', 'resource.name', 'tracker.tasks', 'type')
  get data() {
    if (!this.tracker) return [];

    const metric = this.metric;
    if (this.type === 'task-state') {
      // handle getting the right task out of the tracker
      const task = this.get('tracker.tasks').findBy('task', this.get('resource.name'));
      return task && task[metric];
    }

    return this.get(`tracker.${metric}`);
  }

  @computed('metric', 'resource.name', 'tracker.tasks', 'type')
  get reservedAmount() {
    const metricProperty = this.metric === 'cpu' ? 'reservedCPU' : 'reservedMemory';

    if (this.type === 'task-state') {
      const task = this.get('tracker.tasks').findBy('task', this.get('resource.name'));
      return task[metricProperty];
    }

    return this.get(`tracker.${metricProperty}`);
  }

  @computed('metric')
  get chartClass() {
    const metric = this.metric;
    const mappings = {
      cpu: 'is-info',
      memory: 'is-danger',
    };

    return mappings[metric] || 'is-primary';
  }

  @task(function*() {
    do {
      this.get('tracker.poll').perform();
      yield timeout(100);
    } while (!Ember.testing);
  })
  poller;

  didReceiveAttrs() {
    if (this.tracker) {
      this.poller.perform();
    }
  }

  willDestroy() {
    this.poller.cancelAll();
    this.get('tracker.signalPause').perform();
  }
}
