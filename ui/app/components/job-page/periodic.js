import AbstractJobPage from './abstract';
import { inject as service } from '@ember/service';

export default AbstractJobPage.extend({
  store: service(),

  errorMessage: '',

  actions: {
    forceLaunch() {
      this.get('job')
        .forcePeriodic()
        .catch(error => {
          this.set('errorMessage', `Could not force launch: ${error}`);
        });
    },
    clearErrorMessage() {
      this.set('errorMessage', '');
    },
  },
});
