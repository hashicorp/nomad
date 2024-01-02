/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { schedule } from '@ember/runloop';

export default class BucketService extends Service {
  @tracked crumbs = [];

  @action registerBreadcrumb(crumb) {
    schedule('actions', this, () => {
      this.crumbs = [...this.crumbs, crumb];
    });
  }

  @action deregisterBreadcrumb(crumb) {
    const newCrumbs = this.crumbs.filter((c) => c !== crumb);

    this.crumbs = newCrumbs;
  }
}
