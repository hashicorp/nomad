/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { inject as service } from '@ember/service';

@tagName('')
export default class ServerSubnav extends Component {
  @service keyboard;
}
