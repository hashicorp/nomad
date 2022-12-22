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

  async model() {
    const jobTemplateVariables = await this.store.query('variable', {
      prefix: 'nomad/job-templates',
    });
    const recordsToQuery = jobTemplateVariables.map((template) =>
      this.store.findRecord('variable', template.id)
    );

    return Promise.all(recordsToQuery);
  }
}
