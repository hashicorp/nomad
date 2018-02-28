import Route from '@ember/routing/route';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { watchRecord } from 'nomad-ui/utils/properties/watch';

export default Route.extend(WithModelErrorHandling, {
  setupController(controller, model) {
    controller.set('watcher', this.get('watch').perform(model));
    return this._super(...arguments);
  },

  deactivate() {
    this.get('watch').cancelAll();
    return this._super(...arguments);
  },

  watch: watchRecord('allocation'),
});
