import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
export default class VariablesVariableEditController extends Controller {
  @service store;
  queryParams = ['path'];
  get existingVariables() {
    return this.store.peekAll('variable');
  }
}
