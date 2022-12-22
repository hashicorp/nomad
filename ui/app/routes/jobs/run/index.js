import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import classic from 'ember-classic-decorator';

@classic
export default class RunRoute extends Route {
  @service can;
  @service store;
  @service system;

  beforeModel(transition) {
    if (
      this.can.cannot('run job', null, {
        namespace: transition.to.queryParams.namespace,
      })
    ) {
      this.transitionTo('jobs');
    }
  }

  async model() {
    const transition = arguments[1];

    if (transition.from?.name === 'jobs.run.templates.index') {
      const plainId = transition.to.queryParams.plainId;
      return this.store.peekAll('job').findBy('plainId', plainId);
    }

    // When jobs are created with a namespace attribute, it is verified against
    // available namespaces to prevent redirecting to a non-existent namespace.
    return this.store.findAll('namespace').then(() => {
      return this.store.createRecord('job');
    });
  }

  resetController(controller, isExiting, transition) {
    if (isExiting) {
      if (transition.to.name === 'jobs.run.templates.index') {
        return;
      }
      controller.model.deleteRecord();
    }
  }
}
