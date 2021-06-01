import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class DispatchRoute extends Route {
  @service can;

  breadcrumbs = [
    {
      label: 'Dispatch',
      args: ['jobs.run'],
    },
  ];

  beforeModel() {
    if (this.can.cannot('dispatch job')) {
      this.transitionTo('jobs.job');
    }
  }

  model() {
    const job = this.modelFor('jobs.job');
    if (!job) return this.transitionTo('jobs.job');

    return job.fetchRawDefinition().then(definition => ({
      rawJob: job,
      definition,
    }));
  }
}
