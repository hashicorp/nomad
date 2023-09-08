/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

const ALL_NAMESPACE_WILDCARD = '*';

export default class VariablesIndexController extends Controller {
  @service router;
  @service store;

  isForbidden = false;

  @action
  goToVariable(variable) {
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
