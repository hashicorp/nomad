import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import classic from 'ember-classic-decorator';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

@classic
export default class RunRoute extends Route {
  @service can;
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
      this.transitionTo('jobs');
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
          _newDefinition: templateRecord.keyValues?.find((el) => {
            return el.key === 'template';
          })?.value,
        });
      }

      return this.store.createRecord('job');
    } catch (e) {
      notifyForbidden(this)(e);
    }
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.model.deleteRecord();
    }
  }
}
