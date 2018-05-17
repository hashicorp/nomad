import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyError from 'nomad-ui/utils/notify-error';

export default Route.extend(WithWatchers, {
  startWatchers(controller, model) {
    controller.set('watcher', this.get('watch').perform(model));
  },

  model() {
    // Preload the job for the allocation since it's required for the breadcrumb trail
    return this._super(...arguments)
      .then(allocation => allocation.get('job').then(() => allocation))
      .catch(notifyError(this));
  },

  watch: watchRecord('allocation'),

  watchers: collect('watch'),
});
