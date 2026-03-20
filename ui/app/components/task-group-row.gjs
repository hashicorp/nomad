/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { array } from '@ember/helper';
import { tracked } from '@glimmer/tracking';
import { debounce, join } from '@ember/runloop';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { or } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import cannot from 'ember-can/helpers/cannot';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import AllocationStatusBar from 'nomad-ui/components/allocation-status-bar';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default class TaskGroupRow extends Component {
  @service abilities;

  @tracked count = 0;
  debounce = 500;

  constructor() {
    super(...arguments);
    this.syncCount();
  }

  syncCount = () => {
    this.count = Number(this.args.taskGroup?.count ?? 0);
  };

  handleClick = (event) => {
    lazyClick([this.args.onClick, event]);
  };

  get runningDeployment() {
    return this.args.taskGroup?.job?.runningDeployment;
  }

  get namespace() {
    const job = this.args.taskGroup?.job;

    const namespaceId =
      (typeof job?.get === 'function' ? job.get('namespaceId') : undefined) ||
      job?.namespaceId;
    if (namespaceId) {
      return namespaceId;
    }

    const jobId =
      (typeof job?.get === 'function' ? job.get('id') : undefined) || job?.id;
    if (jobId) {
      try {
        const [, parsedNamespace] = JSON.parse(jobId);
        return parsedNamespace || 'default';
      } catch {
        // Fall through to final default.
      }
    }

    if (typeof job?.namespace === 'string') {
      return job.namespace;
    }

    return 'default';
  }

  get tooltipText() {
    if (this.abilities.cannot('scale job', null, { namespace: this.namespace }))
      return "You aren't allowed to scale task groups";
    if (this.runningDeployment)
      return 'You cannot scale task groups during a deployment';
    return undefined;
  }

  get isMinimum() {
    const scaling = this.args.taskGroup.scaling;
    if (!scaling || scaling.min == null) return false;
    return this.count <= scaling.min;
  }

  get isMaximum() {
    const scaling = this.args.taskGroup.scaling;
    if (!scaling || scaling.max == null) return false;
    return this.count >= scaling.max;
  }

  countUp = () => {
    join(this, () => {
      const scaling = this.args.taskGroup.scaling;
      if (!scaling || scaling.max == null || this.count < scaling.max) {
        const nextCount = this.count + 1;
        this.count = nextCount;
        if (typeof this.args.taskGroup?.set === 'function') {
          this.args.taskGroup.set('count', nextCount);
        } else if (this.args.taskGroup) {
          this.args.taskGroup.count = nextCount;
        }
        this.scale(nextCount);
      }
    });
  };

  countDown = () => {
    join(this, () => {
      const scaling = this.args.taskGroup.scaling;
      if (!scaling || scaling.min == null || this.count > scaling.min) {
        const nextCount = this.count - 1;
        this.count = nextCount;
        if (typeof this.args.taskGroup?.set === 'function') {
          this.args.taskGroup.set('count', nextCount);
        } else if (this.args.taskGroup) {
          this.args.taskGroup.count = nextCount;
        }
        this.scale(nextCount);
      }
    });
  };

  scale(count) {
    debounce(this, this.sendCountAction, count, this.debounce);
  }

  sendCountAction = (count) => {
    return this.args.taskGroup.scale(count);
  };

  <template>
    <tr
      class="task-group-row is-interactive"
      data-test-task-group
      {{on "click" this.handleClick}}
      {{didUpdate this.syncCount @taskGroup.count}}
    >
      <td data-test-task-group-name={{@taskGroup.name}}>
        <LinkTo
          @route="jobs.job.task-group"
          @models={{array @taskGroup.job @taskGroup}}
          class="is-primary"
        >
          {{@taskGroup.name}}
        </LinkTo>
      </td>
      <td data-test-task-group-count class="nowrap">
        {{this.count}}
        {{#if @taskGroup.scaling}}
          <div
            data-test-scale-controls
            class="button-bar is-shadowless is-text bumper-left
              {{if
                (or
                  this.runningDeployment
                  (cannot 'scale job' namespace=this.namespace)
                )
                'tooltip multiline'
              }}"
            aria-label={{this.tooltipText}}
          >
            <button
              data-test-scale="decrement"
              aria-label="decrement"
              class="button is-xsmall is-light"
              disabled={{or
                this.isMinimum
                this.runningDeployment
                (cannot "scale job" namespace=this.namespace)
              }}
              {{on "click" this.countDown}}
              type="button"
            >
              <HdsIcon @name="minus" @isInline={{true}} />
            </button>
            <button
              data-test-scale-controls-increment
              data-test-scale="increment"
              aria-label="increment"
              class="button is-xsmall is-light"
              disabled={{or
                this.isMaximum
                this.runningDeployment
                (cannot "scale job" namespace=this.namespace)
              }}
              {{on "click" this.countUp}}
              type="button"
            >
              <HdsIcon @name="plus" @isInline={{true}} />
            </button>
          </div>
        {{/if}}
      </td>
      <td data-test-task-group-allocs>
        <div class="inline-chart"><AllocationStatusBar
            @allocationContainer={{@taskGroup.summary}}
            @isNarrow={{true}}
          /></div>
      </td>
      <td data-test-task-group-volume>{{if
          @taskGroup.volumes.length
          "Yes"
        }}</td>
      <td data-test-task-group-cpu>{{formatScheduledHertz
          @taskGroup.reservedCPU
        }}</td>
      <td data-test-task-group-mem>{{formatScheduledBytes
          @taskGroup.reservedMemory
          start="MiB"
        }}</td>
      <td data-test-task-group-disk>{{formatScheduledBytes
          @taskGroup.reservedEphemeralDisk
          start="MiB"
        }}</td>
    </tr>
  </template>
}
