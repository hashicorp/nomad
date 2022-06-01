import Route from '@ember/routing/route';

export default class VariablesPathRoute extends Route {
  model({ absolutePath }) {
    return this.modelFor('variables').pathTree.findPath(absolutePath);
  }
}
