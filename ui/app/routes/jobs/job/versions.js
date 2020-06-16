import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class VersionsRoute extends Route.extend(WithWatchers) {
  model() {
    const job = this.modelFor('jobs.job');
    return job && job.get('versions').then(() => job);
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.watchVersions.perform(model));
    }
  }

  @watchRelationship('versions') watchVersions;
  @collect('watchVersions') watchers;
}
