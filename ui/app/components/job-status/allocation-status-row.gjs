/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import windowResize from 'nomad-ui/modifiers/window-resize';
import JobStatusAllocationStatusBlock from 'nomad-ui/components/job-status/allocation-status-block';
import JobStatusIndividualAllocation from 'nomad-ui/components/job-status/individual-allocation';

const ALLOC_BLOCK_WIDTH = 32;
const ALLOC_BLOCK_GAP = 10;
const COMPACT_INTER_SUMMARY_GAP = 7;

export default class JobStatusAllocationStatusRow extends Component {
  @tracked width = 0;

  get allocationGroups() {
    return Object.entries(this.args.allocBlocks || {}).flatMap(
      ([status, allocsByStatus]) =>
        Object.entries(allocsByStatus || {}).flatMap(
          ([health, allocsByHealth]) =>
            Object.entries(allocsByHealth || {}).map(
              ([canary, allocsByCanary]) => ({
                status,
                health,
                canary,
                allocs: allocsByCanary || [],
              }),
            ),
        ),
    );
  }

  get allocBlockSlots() {
    return this.allocationGroups.reduce(
      (totalSlots, group) => totalSlots + group.allocs.length,
      0,
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

  get numberOfSummariesShown() {
    return this.allocationGroups.filter((group) => group.allocs.length > 0)
      .length;
  }

  get summaryGroups() {
    return this.allocationGroups
      .filter((group) => group.allocs.length > 0)
      .map((group) => ({
        ...group,
        width: this.calcWidth(group.allocs.length),
      }));
  }

  get individualAllocs() {
    return this.allocationGroups.flatMap((group) =>
      group.allocs.map((allocation) => ({
        allocation,
        status: group.status,
        health: group.health,
        canary: group.canary,
      })),
    );
  }

  get compactTally() {
    if (!this.args.compact) {
      return null;
    }

    if (this.args.allocationTallyMode === 'complete') {
      return `${this.args.completeAllocs}/${this.args.groupCountSum}`;
    }

    return `${this.args.runningAllocs}/${this.args.groupCountSum}`;
  }

  calcWidth(count) {
    if (this.args.compact) {
      const totalGaps =
        (this.numberOfSummariesShown - 1) * COMPACT_INTER_SUMMARY_GAP;
      const usableWidth = this.width - totalGaps;
      return (count / this.allocBlockSlots) * usableWidth;
    }

    return (count / this.allocBlockSlots) * this.width;
  }

  reflow = (element) => {
    this.width = element.clientWidth;
  };

  captureElement = (element) => {
    this.width = element.clientWidth;
  };

  <template>
    <div class="allocation-status-row {{if @compact 'compact'}}" ...attributes>
      {{#if this.showSummaries}}
        <div
          class="alloc-status-summaries"
          {{didInsert this.captureElement}}
          {{windowResize this.reflow}}
        >
          {{#each this.summaryGroups as |group|}}
            <JobStatusAllocationStatusBlock
              @status={{group.status}}
              @health={{group.health}}
              @canary={{group.canary}}
              @steady={{@steady}}
              @count={{group.allocs.length}}
              @width={{group.width}}
              @compact={{@compact}}
              @allocs={{group.allocs}}
            />
          {{/each}}
        </div>
      {{else}}
        <div
          class="ungrouped-allocs"
          {{didInsert this.captureElement}}
          {{windowResize this.reflow}}
        >
          {{#each this.individualAllocs as |item|}}
            <JobStatusIndividualAllocation
              @allocation={{item.allocation}}
              @status={{item.status}}
              @health={{item.health}}
              @canary={{item.canary}}
              @steady={{@steady}}
            />
          {{/each}}
        </div>
      {{/if}}
      {{#if @compact}}
        {{this.compactTally}}
      {{/if}}
    </div>
  </template>
}
