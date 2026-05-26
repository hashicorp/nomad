/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';

export default class DasResourceTotals extends Component {
  get diffs() {
    return new ResourcesDiffs(
      this.args.model,
      1,
      this.args.recommendations,
      this.args.excludedRecommendations,
    );
  }

  get cpuClass() {
    return classForDelta(this.diffs.cpu.delta);
  }

  get memoryClass() {
    return classForDelta(this.diffs.memory.delta);
  }

  <template>
    <table class="diffs-table" ...attributes>
      <tbody>
        <tr data-test-current>
          <th>Current</th>
          <td data-test-cpu>{{@model.reservedCPU}} MHz</td>
          <td data-test-memory>{{@model.reservedMemory}} MiB</td>
          <th class="diff">Difference</th>
          <td
            class="diff"
            data-test-cpu-unit-diff
          >{{this.diffs.cpu.signedDiff}}</td>
          <td class="diff" data-test-memory-unit-diff>
            {{this.diffs.memory.signedDiff}}
          </td>
        </tr>
        <tr data-test-recommended>
          <th>Recommended</th>
          <td
            data-test-cpu
            class={{this.cpuClass}}
          >{{this.diffs.cpu.recommended}}
            MHz</td>
          <td data-test-memory class={{this.memoryClass}}>
            {{this.diffs.memory.recommended}}
            MiB
          </td>
          <th class="diff">% Difference</th>
          <td class="diff" data-test-cpu-percent-diff>
            {{this.diffs.cpu.percentDiff}}
          </td>
          <td class="diff" data-test-memory-percent-diff>
            {{this.diffs.memory.percentDiff}}
          </td>
        </tr>
      </tbody>
    </table>
  </template>
}

function classForDelta(delta) {
  if (delta > 0) {
    return 'increase';
  } else if (delta < 0) {
    return 'decrease';
  }

  return '';
}
