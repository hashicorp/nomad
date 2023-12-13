/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import Component from '@glimmer/component';

export default class NamespaceFilter extends Component {
  @service store;

  @action
  async fetchNamespaces() {
    return this.store.findAll('namespace');
  }

  @action
  formatAndSetNamespaces() {
    // Triggered on the promise in fetchNamespaces resolving
    const namespaces = this.store
      .peekAll('namespace')
      .map(({ name }) => ({ key: name, label: name }));

    if (namespaces.length <= 1) return null;

    this.args.fns.setNamespaceOptions(namespaces);
  }
}
