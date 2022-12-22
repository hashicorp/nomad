import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class RunTemplatesRoute extends Route {
  @service can;
  @service store;

  beforeModel(transition) {
    const hasPermissions = this.can.can('write variable', null, {
      namespace: '*',
      path: '*',
    });

    // We create a job with no id in jobs.run that is populated by this form.
    // A user cannot start at this route.
    const wasJobModelCreated = transition.from?.name === 'jobs.run.index';
    if (!hasPermissions || !wasJobModelCreated) {
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
