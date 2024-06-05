/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

const ALLOC_BLOCK_WIDTH = 32;
const ALLOC_BLOCK_GAP = 10;
const COMPACT_INTER_SUMMARY_GAP = 7;

export default class JobStatusAllocationStatusRowComponent extends Component {
  @tracked width = 0;

  get allocBlockSlots() {
    return Object.values(this.args.allocBlocks)
      .flatMap((statusObj) => Object.values(statusObj))
      .flatMap((healthObj) => Object.values(healthObj))
      .reduce(
        (totalSlots, allocsByCanary) =>
          totalSlots + (allocsByCanary ? allocsByCanary.length : 0),
        0
      );
  }

  get showSummaries() {
    return (
      this.args.compact ||
      this.allocBlockSlots * (ALLOC_BLOCK_WIDTH + ALLOC_BLOCK_GAP) -
        ALLOC_BLOCK_GAP >
        this.width
    );
  }

  // When we calculate how much width to give to a row in our viz,
  // we want to also offset the gap BETWEEN summaries. The way that css grid
  // works, a gap only appears between 2 elements, not at the start or end of a row.
  // Thus, we need to calculate total gap space using the number of summaries shown.
  get numberOfSummariesShown() {
    return Object.values(this.args.allocBlocks)
      .flatMap((statusObj) => Object.values(statusObj))
      .flatMap((healthObj) => Object.values(healthObj))
      .filter((allocs) => allocs.length > 0).length;
  }

  calcPerc(count) {
    if (this.args.compact) {
      const totalGaps =
        (this.numberOfSummariesShown - 1) * COMPACT_INTER_SUMMARY_GAP;
      const usableWidth = this.width - totalGaps;
      return (count / this.allocBlockSlots) * usableWidth;
    } else {
      return (count / this.allocBlockSlots) * this.width;
    }
  }

  @action reflow(element) {
    this.width = element.clientWidth;
  }

  @action
  captureElement(element) {
    this.width = element.clientWidth;
  }
}
