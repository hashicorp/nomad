import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class RunTemplatesRoute extends Route {
  @service can;
  @service store;

  beforeModel() {
    if (
      this.can.cannot('write variable', null, {
        namespace: '*',
        path: '*',
      })
    ) {
      this.transitionTo('jobs');
    }
  }

  model() {
    return this.store.query('variable', {
      prefix: 'nomad/job-templates',
      filter: 'Template is not empty"',
    });
  }
}
