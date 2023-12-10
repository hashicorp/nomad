/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class TaskEventSerializer extends ApplicationSerializer {
  attrs = {
    message: 'DisplayMessage',
  };

  separateNanos = ['Time'];
}
