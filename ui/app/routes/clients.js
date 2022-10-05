import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import classic from 'ember-classic-decorator';

@classic
export default class ClientsRoute extends Route.extend(WithForbiddenState) {
  @service store;
  @service system;

  beforeModel() {
    return this.get('system.leader');
  }

  async model() {
    try {
      return {
        nodes: await this.store.findAll('node'),
        agents: await this.store.findAll('agent'),
      };
    } catch (e) {
      notifyForbidden(this)(e);
    }
  }
}
