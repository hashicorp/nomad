/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

const ALL_NAMESPACE_WILDCARD = '*';

export default class VariablesPathController extends Controller {
  @service router;

  get absolutePath() {
    return this.model?.absolutePath || '';
  }
  get breadcrumbs() {
    if (this.absolutePath) {
      let crumbs = [];
      this.absolutePath.split('/').reduce((m, n) => {
        crumbs.push({
          label: n,
          args: [`variables.path`, m + n],
        });
        return m + n + '/';
      }, []);
      return crumbs;
    } else {
      return [];
    }
  }

  @action goToNewVariable() {
    this.router.transitionTo('variables.new', {
      queryParams: { path: `${this.absolutePath}/` },
    });
  }

  @controller variables;

  @action
  setNamespace(namespace) {
    this.variables.setNamespace(namespace);
  }

  get namespaceSelection() {
    return this.variables.qpNamespace;
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
