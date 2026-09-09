/**
 * Copyright IBM Corp. 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class GroupSelections extends Component {
  get selections() {
    return Object.entries(
      this.args.job.effectiveGroupSelectionStatuses || {},
    ).map(([name, status]) => ({
      name,
      count: status.Count,
      placed: status.Count - status.Unplaced,
      unplaced: status.Unplaced,
    }));
  }
}
