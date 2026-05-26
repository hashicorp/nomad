/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { and, not } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { formatBytes, formatHertz } from 'nomad-ui/utils/units';

export default class AllocationStat extends Component {
  get metric() {
    return this.args.metric || 'memory'; // Either memory or cpu
  }

  get statClass() {
    return this.metric === 'cpu' ? 'is-info' : 'is-danger';
  }

  get cpu() {
    const cpu = this.args.statsTracker?.cpu;
    return cpu?.[cpu.length - 1];
  }

  get memory() {
    const memory = this.args.statsTracker?.memory;
    return memory?.[memory.length - 1];
  }

  get stat() {
    const metric = this.metric;
    if (metric === 'cpu' || metric === 'memory') {
      return this[metric];
    }

    return undefined;
  }

  get formattedStat() {
    if (!this.stat) return undefined;
    if (this.metric === 'memory') return formatBytes(this.stat.used);
    if (this.metric === 'cpu') return formatHertz(this.stat.used, 'MHz');
    return undefined;
  }

  get formattedReserved() {
    if (this.metric === 'memory')
      return formatBytes(this.args.statsTracker?.reservedMemory, 'MiB');
    if (this.metric === 'cpu')
      return formatHertz(this.args.statsTracker?.reservedCPU, 'MHz');
    return undefined;
  }

  <template>
    {{#if @allocation.isRunning}}
      {{#if (and (not this.stat) @isLoading)}}
        &hellip;
      {{else if @error}}
        <span
          class="tooltip is-small text-center"
          role="tooltip"
          aria-label="Couldn't collect stats"
        >
          <HdsIcon
            @name="alert-triangle-fill"
            @color="warning"
            class="icon-vertical-bump-down"
          />
        </span>
      {{else}}
        <div
          class="inline-chart tooltip"
          role="tooltip"
          aria-label="{{this.formattedStat}} / {{this.formattedReserved}}"
        >
          <progress
            class="progress is-small {{this.statClass}}"
            value="{{this.stat.percent}}"
            max="1"
          >
            {{this.stat.percent}}
          </progress>
        </div>
      {{/if}}
    {{/if}}
  </template>
}
