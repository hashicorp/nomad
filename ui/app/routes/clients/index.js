import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  startWatchers(controller) {
    controller.set('watcher', this.get('watch').perform());
  },

  watch: watchAll('node'),
  watchers: collect('watch'),
});
