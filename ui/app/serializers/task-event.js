/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
