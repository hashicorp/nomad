import Controller from '@ember/controller';

export default class VariablesVariableController extends Controller {
  get breadcrumb() {
    return {
      label: this.model.path,
      args: [`variables.variable`, this.model.path],
    };
  }
}
