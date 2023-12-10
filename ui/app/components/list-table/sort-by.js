/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import {
  classNames,
  attributeBindings,
  classNameBindings,
  tagName,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('th')
@attributeBindings('title')
@classNames('is-selectable')
@classNameBindings('isActive:is-active', 'sortDescending:desc:asc')
export default class SortBy extends Component {
  // The prop that the table is currently sorted by
  currentProp = '';

  // The prop this sorter controls
  prop = '';

  @computed('currentProp', 'prop')
  get isActive() {
    return this.currentProp === this.prop;
  }

  @computed('sortDescending', 'isActive')
  get shouldSortDescending() {
    return !this.isActive || !this.sortDescending;
  }
}
