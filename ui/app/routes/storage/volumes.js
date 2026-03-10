/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import classic from 'ember-classic-decorator';

@classic
export default class VolumesRoute extends Route.extend() {
  @service system;
  @service store;
}
