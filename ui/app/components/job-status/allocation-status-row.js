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
      this.allocBlockSlots * (ALLOC_BLOCK_WIDTH + ALLOC_BLOCK_GAP) -
        ALLOC_BLOCK_GAP >
      this.width
    );
  }

  calcPerc(count) {
    return (count / this.allocBlockSlots) * this.width;
  }

  @action reflow(element) {
    this.width = element.clientWidth;
  }

  @action
  captureElement(element) {
    this.width = element.clientWidth;
  }
}
