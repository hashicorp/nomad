// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import compactPath from '../utils/compact-path';
export default class VariablePathsComponent extends Component {
  @service router;

  /**
   * @returns {Array<Object.<string, Object>>}
   */
  get folders() {
    return Object.entries(this.args.branch.children).map(([name]) => {
      return compactPath(this.args.branch.children[name], name);
    });
  }

  get files() {
    return this.args.branch.files;
  }

  @action
  async handleFolderClick(path) {
    this.router.transitionTo('variables.path', path);
  }

  @action
  async handleFileClick(path) {
    this.router.transitionTo('variables.variable', path);
  }
}
