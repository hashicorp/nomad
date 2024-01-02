/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import compactPath from '../utils/compact-path';
export default class VariablePathsComponent extends Component {
  @service router;
  @service can;

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
  async handleFileClick({ path, variable: { id, namespace } }) {
    if (this.can.can('read variable', null, { path, namespace })) {
      this.router.transitionTo('variables.variable', id);
    }
  }
}
