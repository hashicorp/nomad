import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class ClientRoute extends Route {
  @service store;

  async model() {
    try {
      return super.model(...arguments);
    } catch (e) {
      notifyError(this)(e);
    }
  }

  afterModel(model) {
    if (model && model.get('isPartial')) {
      return model.reload().then((node) => node.get('allocations'));
    }
    return model && model.get('allocations');
  }
}
