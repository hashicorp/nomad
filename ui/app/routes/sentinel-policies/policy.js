/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class PolicyRoute extends Route {
  @service store;

  model() {
    return super.model(...arguments).catch(notifyError(this));
  }
}
