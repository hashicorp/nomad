/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import {
  classNames,
  classNameBindings,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('accordion-head')
@classNameBindings('isOpen::is-light', 'isExpandable::is-inactive')
@attributeBindings('data-test-accordion-head')
export default class AccordionHead extends Component {
  'data-test-accordion-head' = true;

  buttonLabel = 'toggle';
  isOpen = false;
  isExpandable = true;
  item = null;

  onClose() {}
  onOpen() {}
}
