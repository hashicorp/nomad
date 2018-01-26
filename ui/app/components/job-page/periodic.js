import AbstractJobPage from './abstract';
import { inject as service } from '@ember/service';

export default AbstractJobPage.extend({
  store: service(),
  actions: {
    forceLaunch() {
      this.get('job')
        .forcePeriodic()
        .then(() => {
          this.get('store').findAll('job');
        });
    },
  },
});
