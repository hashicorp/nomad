/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { service } from '@ember/service';
import Route from '@ember/routing/route';

export default class VolumesRoute extends Route.extend() {
  @service system;
  @service store;
}
