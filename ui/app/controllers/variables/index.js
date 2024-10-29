/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
// eslint-disable-next-line no-unused-vars
import VariableModel from '../../models/variable';

const ALL_NAMESPACE_WILDCARD = '*';

export default class VariablesIndexController extends Controller {
  @service router;
  @service store;

  isForbidden = false;

  /**
   * Trigger can either be the pointer event itself, or if the keyboard shorcut was used, the html element corresponding to the variable.
   * @param {VariableModel} variable
   * @param {PointerEvent|HTMLElement} trigger
   */
  @action
  goToVariable(variable, trigger) {
    // Don't navigate if the user clicked on a link; this will happen with modifier keys like cmd/ctrl on the link itself
    if (
      trigger instanceof PointerEvent &&
      /** @type {HTMLElement} */ (trigger.target).tagName === 'A'
    ) {
      return;
    }
    this.router.transitionTo('variables.variable', variable.path);
  }

  @action goToNewVariable() {
    this.router.transitionTo('variables.new');
  }

  @controller variables;

  @action
  setNamespace(namespace) {
    this.variables.setNamespace(namespace);
  }

  get namespaceSelection() {
    return this.variables.qpNamespace;
  }

  get hasVariables() {
    return this.model.variables.length;
  }

  get root() {
    return this.model.root;
  }

  get namespaceOptions() {
    const namespaces = this.store
      .peekAll('namespace')
      .map(({ name }) => ({ key: name, label: name }));

    if (namespaces.length <= 1) return null;

    // Create default namespace selection
    namespaces.unshift({
      key: ALL_NAMESPACE_WILDCARD,
      label: 'All (*)',
    });

    return namespaces;
  }
}
