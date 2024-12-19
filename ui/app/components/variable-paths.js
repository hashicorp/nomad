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
  async handleFolderClick(path, trigger) {
    // Don't navigate if the user clicked on a link; this will happen with modifier keys like cmd/ctrl on the link itself
    if (
      trigger instanceof PointerEvent &&
      /** @type {HTMLElement} */ (trigger.target).tagName === 'A'
    ) {
      return;
    }
    this.router.transitionTo('variables.path', path);
  }

  @action
  async handleFileClick({ path, variable: { id, namespace } }, trigger) {
    if (this.can.can('read variable', null, { path, namespace })) {
      // Don't navigate if the user clicked on a link; this will happen with modifier keys like cmd/ctrl on the link itself
      if (
        trigger instanceof PointerEvent &&
        /** @type {HTMLElement} */ (trigger.target).tagName === 'A'
      ) {
        return;
      }
      this.router.transitionTo('variables.variable', id);
    }
  }
}
