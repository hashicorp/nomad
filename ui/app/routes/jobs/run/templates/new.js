import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class RunRoute extends Route {
  @service can;
  @service router;
  @service store;
  @service system;

  beforeModel(transition) {
    if (
      this.can.cannot('run job', null, {
        namespace: transition.to.queryParams.namespace,
      })
    ) {
      this.router.transitionTo('jobs');
    }
  }

  async model() {
    try {
      // When variables are created with a namespace attribute, it is verified against
      // available namespaces to prevent redirecting to a non-existent namespace.
      await Promise.all([
        this.store.query('variable', {
          prefix: 'nomad/job-templates',
          namespace: '*',
        }),
        this.store.findAll('namespace'),
      ]);

      return this.store.createRecord('variable');
    } catch (e) {
      notifyForbidden(this)(e);
    }
  }

  resetController(controller, isExiting) {
    if (
      isExiting &&
      controller.model.isNew &&
      !controller.model.isDestroyed &&
      !controller.model.isDestroying
    ) {
      controller.model?.unloadRecord();
    }
    controller.set('templateName', null);
    controller.set('templateNamespace', 'default');
  }
}
