/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationAdapter from './application';
import classic from 'ember-classic-decorator';

@classic
export default class AgentAdapter extends ApplicationAdapter {
  pathForType = () => 'agent/members';

  urlForFindRecord() {
    const [, ...args] = arguments;
    return this.urlForFindAll(...args);
  }
}
