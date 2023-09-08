/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { equal, or } from '@ember/object/computed';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class AllocationSubnav extends Component {
  @service router;
  @service keyboard;

  @equal('router.currentRouteName', 'allocations.allocation.fs')
  fsIsActive;

  @equal('router.currentRouteName', 'allocations.allocation.fs-root')
  fsRootIsActive;

  @or('fsIsActive', 'fsRootIsActive') filesLinkActive;
}
