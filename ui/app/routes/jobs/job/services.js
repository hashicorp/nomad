import Route from '@ember/routing/route';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { collect } from '@ember/object/computed';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';

export default class JobsJobServicesRoute extends Route.extend(WithWatchers) {
  model() {
    const job = this.modelFor('jobs.job');
    console.log('yob', job.get('services'));
    return job && job.get('services').then(() => job);
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchServices', this.watchServices.perform(model));
    }
  }

  @watchRelationship('services') watchServices;

  @collect('watchServices') watchers;
}
