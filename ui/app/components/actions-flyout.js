/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default class ActionsFlyoutComponent extends Component {
  @service nomadActions;

  @alias('nomadActions.flyoutActive') isOpen;
}
