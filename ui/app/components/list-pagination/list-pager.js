/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

@classic
@tagName('')
export default class ListPager extends Component {
  @service router;
  @action
  gotoRoute() {
    this.router.transitionTo(this.router.currentRouteName, {
      queryParams: { page: this.page },
    });
  }
}
