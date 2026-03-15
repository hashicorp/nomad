/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class DasRecommendationRow extends Component {
  @tracked cpu = {};
  @tracked memory = {};

  get taskGroup() {
    return this.args.summary.taskGroup;
  }

  get jobName() {
    return (
      this.taskGroup?.job?.name ||
      this.args.summary.job?.name ||
      this.args.summary.jobId
    );
  }

  get allocationCount() {
    return this.taskGroup?.count || 0;
  }

  @action
  storeDiffs() {
    // Prevent resource toggling from affecting the summary diffs

    if (!this.taskGroup) {
      this.cpu = {};
      this.memory = {};
      return;
    }

    const diffs = new ResourcesDiffs(
      this.taskGroup,
      1,
      this.args.summary.recommendations,
      this.args.summary.excludedRecommendations,
    );

    const aggregateDiffs = new ResourcesDiffs(
      this.taskGroup,
      this.taskGroup.count,
      this.args.summary.recommendations,
      this.args.summary.excludedRecommendations,
    );

    this.cpu = {
      delta: diffs.cpu.delta,
      signedDiff: diffs.cpu.signedDiff,
      percentDiff: diffs.cpu.percentDiff,
      signedAggregateDiff: aggregateDiffs.cpu.signedDiff,
    };

    this.memory = {
      delta: diffs.memory.delta,
      signedDiff: diffs.memory.signedDiff,
      percentDiff: diffs.memory.percentDiff,
      signedAggregateDiff: aggregateDiffs.memory.signedDiff,
    };
  }
}
