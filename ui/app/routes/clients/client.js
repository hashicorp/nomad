import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default Route.extend({
  store: service(),

  model() {
    return this._super(...arguments).catch(notifyError(this));
  },

  afterModel(model) {
    if (model && model.get('isPartial')) {
      return model.reload().then(node => node.get('allocations'));
    }
    return model && model.get('allocations');
  },
});
