import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchQuery } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  startWatchers(controller) {
    controller.set('modelWatch', this.watch.perform({ type: 'csi' }));
  },

  watch: watchQuery('volume'),
  watchers: collect('watch'),
});
