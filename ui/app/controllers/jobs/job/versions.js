import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class VersionsController extends Controller.extend(WithNamespaceResetting) {
  error = null;

  @alias('model') job;

  onDismiss() {
    this.set('error', null);
  }

  @action
  handleError(errorObject) {
    this.set('error', errorObject);
  }
}
