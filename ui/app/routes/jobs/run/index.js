import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import classic from 'ember-classic-decorator';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

@classic
export default class JobsRunIndexRoute extends Route {
  @service can;
  @service flashMessages;
  @service router;
  @service store;
  @service system;

  queryParams = {
    template: {
      refreshModel: true,
    },
  };

  beforeModel(transition) {
    if (
      this.can.cannot('run job', null, {
        namespace: transition.to.queryParams.namespace,
      })
    ) {
      this.router.transitionTo('jobs');
    }
  }

  async model({ template }) {
    try {
      // When jobs are created with a namespace attribute, it is verified against
      // available namespaces to prevent redirecting to a non-existent namespace.
      await this.store.findAll('namespace');

      // If template is set in URL, create the job model and add the definition
      if (template) {
        const templateRecord = await this.store.findRecord(
          'variable',
          template
        );

        return this.store.createRecord('job', {
          _newDefinition: templateRecord.items.template,
        });
      }

      return this.store.createRecord('job');
    } catch (e) {
      this.handle404(e);
    }
  }

  handle404(e) {
    const error404 = e.errors.find((err) => err.status === 404);
    if (error404) {
      this.flashMessages.add({
        title: `Error loading ${this.template}`,
        message: error404.detail,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });

      return;
    }
    notifyForbidden(this)(e);
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.model?.deleteRecord();
      controller.set('template', null);
    }
  }
}
