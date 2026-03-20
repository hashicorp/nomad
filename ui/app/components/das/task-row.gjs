/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { fn, concat } from '@ember/helper';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import { and, not } from 'ember-truth-helpers';
import Toggle from 'nomad-ui/components/toggle';

export default class DasTaskRow extends Component {
  @tracked height;

  get half() {
    return this.height / 2;
  }

  get borderCoverHeight() {
    return this.height - 2;
  }

  calculateHeight = (element) => {
    this.height = element.clientHeight + 1;
  };

  <template>
    <tr
      class={{if @active "active"}}
      {{on "click" @onClick}}
      {{didInsert this.calculateHeight}}
      data-test-task-toggles
    >
      <td class="task-cell" data-test-name colspan="2">{{@task.name}}</td>
      <td class="toggle-cell">
        <label>
          <Toggle
            data-test-cpu-toggle
            @isActive={{@cpu.isActive}}
            @onToggle={{fn @toggleRecommendation @cpu.recommendation}}
            @isDisabled={{not @cpu.recommendation}}
            title={{concat "CPU for " @task.name}}
            aria-label={{concat "CPU for " @task.name}}
          />
        </label>
      </td>
      <td class="toggle-cell">
        <label>
          <Toggle
            data-test-memory-toggle
            @isActive={{@memory.isActive}}
            @onToggle={{fn @toggleRecommendation @memory.recommendation}}
            @isDisabled={{not @memory.recommendation}}
            title={{concat "Memory for " @task.name}}
            aria-label={{concat "Memory for " @task.name}}
          />
        </label>
        {{#if (and @active this.height)}}
          <svg width={{this.height}} height={{this.height}}>
            <rect
              class="border-cover"
              x="0"
              y="1"
              height={{this.borderCoverHeight}}
            />
            <polyline
              class="triangle"
              points="1 1 {{this.half}} {{this.half}} 1 {{this.height}}"
            />
          </svg>
        {{/if}}
      </td>
    </tr>
  </template>
}
