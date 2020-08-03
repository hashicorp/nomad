import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class Abstract extends Component {
  @service system;

  job = null;

  // Provide a value that is bound to a query param
  sortProperty = null;
  sortDescending = null;

  // Provide actions that require routing
  gotoTaskGroup() {}
  gotoJob() {}

  // Set to a { title, description } to surface an error
  errorMessage = null;

  @action
  clearErrorMessage() {
    this.set('errorMessage', null);
  }

  @action
  handleError(errorObject) {
    this.set('errorMessage', errorObject);
  }
}
