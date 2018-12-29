import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  system: service(),

  job: null,

  // Provide a value that is bound to a query param
  sortProperty: null,
  sortDescending: null,

  // Provide actions that require routing
  gotoTaskGroup() {},
  gotoJob() {},

  // Set to a { title, description } to surface an error
  errorMessage: null,

  actions: {
    clearErrorMessage() {
      this.set('errorMessage', null);
    },
    handleError(errorObject) {
      this.set('errorMessage', errorObject);
    },
  },
});
