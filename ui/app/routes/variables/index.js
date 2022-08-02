import Route from '@ember/routing/route';

export default class VariablesIndexRoute extends Route {
  model() {
    const { variables, pathTree } = this.modelFor('variables');
    return {
      variables,
      root: pathTree.paths.root,
    };
  }
}
