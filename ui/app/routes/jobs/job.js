import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import { task } from 'ember-concurrency';
import wait from 'nomad-ui/utils/wait';

export default Route.extend({
  store: service(),
  token: service(),
  watchList: service(),

  serialize(model) {
    return { job_name: model.get('plainId') };
  },

  model(params, transition) {
    const namespace = transition.queryParams.namespace || this.get('system.activeNamespace.id');
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);
    return this.get('store')
      .findRecord('job', fullId, { reload: true })
      .then(job => {
        return RSVP.all([job.get('allocations'), job.get('evaluations')]).then(() => job);
      })
      .catch(notifyError(this));
  },

  setupController(controller, model) {
    controller.set('modelTask', this.get('watch').perform(model.get('id')));
    controller.set('summaryTask', this.get('watchRelationship').perform(model, 'summary'));
    controller.set('evaluationsTask', this.get('watchRelationship').perform(model, 'evaluations'));
    controller.set('deploymentsTask', this.get('watchRelationship').perform(model, 'deployments'));
  },

  watch: task(function*(jobId) {
    while (true) {
      try {
        yield RSVP.all([
          this.store.findRecord('job', jobId, { reload: true, adapterOptions: { watch: true } }),
          wait(2000),
        ]);
      } catch (e) {
        yield e;
        break;
      }
    }
  }),

  watchRelationship: task(function*(job, relationshipName) {
    while (true) {
      try {
        yield RSVP.all([
          this.store
            .adapterFor(job.get('modelName'))
            .reloadRelationship(job, relationshipName, true),
          wait(2000),
        ]);
      } catch (e) {
        yield e;
        break;
      }
    }
  }),
});
