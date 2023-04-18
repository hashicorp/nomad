import Route from '@ember/routing/route';

export default class VariablesVariableEditRoute extends Route {
  model() {
    return this.modelFor('variables.variable');
  }
}
