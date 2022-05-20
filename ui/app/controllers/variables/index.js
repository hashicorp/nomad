import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
export default class VariablesIndexController extends Controller {
  @service router;

  isForbidden = false;

  @action
  goToVariable(variable) {
    this.router.transitionTo('variables.variable', variable.path);
  }
}
