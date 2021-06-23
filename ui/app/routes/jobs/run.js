import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import classic from 'ember-classic-decorator';

@classic
export default class RunRoute extends Route {
  @service can;
  @service store;
  @service system;

  breadcrumbs = [
    {
      label: 'Run',
      args: ['jobs.run'],
    },
  ];

  beforeModel() {
    if (this.can.cannot('run job')) {
      this.transitionTo('jobs');
    }
  }

  model() {
    // When jobs are created with a namespace attribute, it is verified against
    // available namespaces to prevent redirecting to a non-existent namespace.
    return this.store.findAll('namespace').then(() => {
      return this.store.createRecord('job');
    });
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.model.deleteRecord();
    }
  }
}
