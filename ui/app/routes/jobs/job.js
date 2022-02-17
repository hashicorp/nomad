import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import classic from 'ember-classic-decorator';

@classic
export default class JobRoute extends Route {
  @service can;
  @service store;
  @service token;

  serialize(model) {
    return { job_name: model.get('idWithNamespace') };
  }

  model(params) {
    const [name, namespace = 'default'] = params.job_name.split('@');

    const fullId = JSON.stringify([name, namespace]);

    return this.store
      .findRecord('job', fullId, { reload: true })
      .then((job) => {
        const relatedModelsQueries = [
          job.get('allocations'),
          job.get('evaluations'),
          this.store.query('job', { namespace }),
          this.store.findAll('namespace'),
        ];

        if (this.can.can('accept recommendation')) {
          relatedModelsQueries.push(job.get('recommendationSummaries'));
        }

        // Optimizing future node look ups by preemptively loading everything
        if (job.get('hasClientStatus') && this.can.can('read client')) {
          relatedModelsQueries.push(this.store.findAll('node'));
        }

        return RSVP.all(relatedModelsQueries).then(() => job);
      })
      .catch(notifyError(this));
  }
}
