import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import classic from 'ember-classic-decorator';

@classic
export default class ServersRoute extends Route.extend(WithForbiddenState) {
  @service store;
  @service system;

  beforeModel() {
    return this.get('system.leader');
  }

  async model() {
    try {
      const [nodes, agents] = await Promise.all([
        this.store.findAll('node'),
        this.store.findAll('agent'),
      ]);

      return {
        nodes,
        agents,
      };
    } catch (e) {
      notifyForbidden(this)(e);
    }
  }
}
