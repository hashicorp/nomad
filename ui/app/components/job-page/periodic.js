import AbstractJobPage from './abstract';
import { inject as service } from '@ember/service';

export default AbstractJobPage.extend({
  store: service(),

  errorMessage: null,

  actions: {
    forceLaunch() {
      this.get('job')
        .forcePeriodic()
        .catch(() => {
          this.set('errorMessage', {
            title: 'Could Not Force Launch',
            description: 'Your ACL token does not grant permission to submit jobs.',
          });
        });
    },
    clearErrorMessage() {
      this.set('errorMessage', null);
    },
  },
});
