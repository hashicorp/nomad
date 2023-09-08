/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import {
  classNames,
  classNameBindings,
  tagName,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('label')
@classNames('toggle')
@classNameBindings('isDisabled:is-disabled', 'isActive:is-active')
@attributeBindings('data-test-label')
export default class Toggle extends Component {
  'data-test-label' = true;

  isActive = false;
  isDisabled = false;
  onToggle() {}
}
