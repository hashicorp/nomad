import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class JobsRunTemplatesManageRoute extends Route {
  @service can;
  @service router;
  @service store;

  beforeModel() {
    const hasPermissions = this.can.can('write variable', null, {
      namespace: '*',
      path: '*',
    });

    if (!hasPermissions) {
      this.router.transitionTo('jobs');
    }
  }

  async model() {
    const jobTemplateVariables = await this.store.query('variable', {
      prefix: 'nomad/job-templates',
      namespace: '*',
    });
    const recordsToQuery = jobTemplateVariables.map((template) =>
      this.store.findRecord('variable', template.id)
    );

    return Promise.all(recordsToQuery);
  }
}
