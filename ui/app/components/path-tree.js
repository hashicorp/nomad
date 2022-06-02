// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class PathTreeComponent extends Component {
  @service router;

  /**
   * @returns {Array<Object.<string, Object>>}
   */
  get folders() {
    return Object.entries(this.args.branch.children).map(([name, data]) => {
      return { name, data };
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
