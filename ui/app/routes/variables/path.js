import Route from '@ember/routing/route';
export default class VariablesPathRoute extends Route {
  model({ absolutePath }) {
    const treeAtPath =
      this.modelFor('variables').pathTree.findPath(absolutePath);
    if (treeAtPath) {
      return { treeAtPath, absolutePath };
    } else {
      return { absolutePath };
    }
  }
}
