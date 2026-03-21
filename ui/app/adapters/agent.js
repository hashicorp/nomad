/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationAdapter from './application';

export default class AgentAdapter extends ApplicationAdapter {
  pathForType = () => 'agent/members';

  urlForFindRecord() {
    const [, ...args] = arguments;
    return this.urlForFindAll(...args);
  }
}
