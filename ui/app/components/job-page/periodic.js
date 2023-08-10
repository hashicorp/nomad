/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import Component from '@glimmer/component';

export default class Periodic extends Component {
  @action
  forceLaunch(setError) {
    this.args.job.forcePeriodic().catch((err) => {
      setError(err);
    });
  }
}
