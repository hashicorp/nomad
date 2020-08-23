import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchQuery } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class IndexRoute extends Route.extend(WithWatchers) {
  startWatchers(controller) {
    controller.set('modelWatch', this.watch.perform({ type: 'csi' }));
  }

  @watchQuery('volume') watch;
  @collect('watch') watchers;
}
