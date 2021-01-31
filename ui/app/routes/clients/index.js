import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class IndexRoute extends Route.extend(WithWatchers) {
  startWatchers(controller) {
    controller.set('watcher', this.watch.perform());
  }

  @watchAll('node') watch;
  @collect('watch') watchers;
}
