/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { or } from '@ember/object/computed';
import classic from 'ember-classic-decorator';

@classic
export default class PlacementFailure extends Component {
  // Either provide a taskGroup or a failedTGAlloc
  taskGroup = null;
  failedTGAlloc = null;

  @or('taskGroup.placementFailures', 'failedTGAlloc') placementFailures;
}
