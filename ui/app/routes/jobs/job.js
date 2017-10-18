import Ember from 'ember';
import notifyError from 'nomad-ui/utils/notify-error';

const { Route, inject } = Ember;

export default Route.extend({
  store: inject.service(),

  model({ job_id }) {
    return this.get('store')
      .findRecord('job', job_id, { reload: true })
      .catch(notifyError(this));
  },
});
