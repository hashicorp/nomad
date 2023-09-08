/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action, computed } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class ServerController extends Controller {
  activeTab = 'tags';

  @computed('model.tags')
  get sortedTags() {
    const tags = this.get('model.tags') || {};
    return Object.keys(tags)
      .map((name) => ({
        name,
        value: tags[name],
      }))
      .sortBy('name');
  }

  @action
  setTab(tab) {
    this.set('activeTab', tab);
  }
}
