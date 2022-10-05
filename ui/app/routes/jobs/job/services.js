import Route from '@ember/routing/route';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { collect } from '@ember/object/computed';
import {
  watchRecord,
  watchRelationship,
} from 'nomad-ui/utils/properties/watch';

export default class JobsJobServicesRoute extends Route.extend(WithWatchers) {
  async model() {
    const job = this.modelFor('jobs.job');

    if (job) {
      await job.get('services');
      return job;
    }
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchServices', this.watchServices.perform(model));
      controller.set('watchJob', this.watchJob.perform(model));
    }
  }

  @watchRelationship('services', true) watchServices;
  @watchRecord('job') watchJob;

  @collect('watchServices', 'watchJob') watchers;
}
