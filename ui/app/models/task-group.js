import { computed } from '@ember/object';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner, fragmentArray, fragment } from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';
import classic from 'ember-classic-decorator';

const maybe = arr => arr || [];

@classic
export default class TaskGroup extends Fragment {
  @fragmentOwner() job;

  @attr('string') name;
  @attr('number') count;

  @fragmentArray('task') tasks;

  @fragmentArray('service') services;

  @fragmentArray('volume-definition') volumes;

  @fragment('group-scaling') scaling;

  @computed('tasks.@each.driver')
  get drivers() {
    return this.tasks.mapBy('driver').uniq();
  }

  @computed('job.allocations.@each.taskGroup')
  get allocations() {
    return maybe(this.get('job.allocations')).filterBy('taskGroupName', this.name);
  }

  @sumAggregation('tasks', 'reservedCPU') reservedCPU;
  @sumAggregation('tasks', 'reservedMemory') reservedMemory;
  @sumAggregation('tasks', 'reservedDisk') reservedDisk;

  @attr('number') reservedEphemeralDisk;

  @computed('job.latestFailureEvaluation.failedTGAllocs.[]')
  get placementFailures() {
    const placementFailures = this.get('job.latestFailureEvaluation.failedTGAllocs');
    return placementFailures && placementFailures.findBy('name', this.name);
  }

  @computed('summary.{queuedAllocs,startingAllocs}')
  get queuedOrStartingAllocs() {
    return this.get('summary.queuedAllocs') + this.get('summary.startingAllocs');
  }

  @computed('job.taskGroupSummaries.[]')
  get summary() {
    return maybe(this.get('job.taskGroupSummaries')).findBy('name', this.name);
  }

  @computed('job.scaleState.taskGroupScales.[]')
  get scaleState() {
    return maybe(this.get('job.scaleState.taskGroupScales')).findBy('name', this.name);
  }

  scale(count, message) {
    return this.job.scale(this.name, count, message);
  }
}
