import Controller from '@ember/controller';

export default class VariablesVariableController extends Controller {
  get breadcrumb() {
    return {
      title: 'Variable',
      label: this.model.path,
      args: [`variables.variable`, this.model.path],
    };
  }
}
