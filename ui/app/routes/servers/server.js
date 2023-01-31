import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class ServerRoute extends Route {
  async model() {
    try {
      return super.model(...arguments);
    } catch (e) {
      notifyError.call(this, e);
    }
  }
}
