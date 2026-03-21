/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AbstractAbility from './abstract';
import { alias } from '@ember/object/computed';

export default class Namespace extends AbstractAbility {
  @alias('selfTokenIsManagement') canList;
  @alias('selfTokenIsManagement') canUpdate;
  @alias('selfTokenIsManagement') canWrite;
  @alias('selfTokenIsManagement') canDestroy;
}
