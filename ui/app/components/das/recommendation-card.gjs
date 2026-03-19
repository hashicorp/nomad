/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { htmlSafe } from '@ember/template';
import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import { and, not, eq } from 'ember-truth-helpers';
import includes from 'ember-composable-helpers/helpers/includes';
import { didCancel, task, timeout } from 'ember-concurrency';
import { macroCondition, isTesting } from '@embroider/macros';
import CopyButton from 'nomad-ui/components/copy-button';
import Toggle from 'nomad-ui/components/toggle';
import DasAccepted from 'nomad-ui/components/das/accepted';
import DasDismissed from 'nomad-ui/components/das/dismissed';
import DasDiffsTable from 'nomad-ui/components/das/diffs-table';
import DasError from 'nomad-ui/components/das/error';
import DasRecommendationChart from 'nomad-ui/components/das/recommendation-chart';
import DasTaskRow from 'nomad-ui/components/das/task-row';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';

export default class DasRecommendationCardComponent extends Component {
  @service router;

  @tracked allCpuToggleActive = true;
  @tracked allMemoryToggleActive = true;

  @tracked activeTaskToggleRowIndex = 0;

  element = null;

  @tracked cardHeight;
  @tracked interstitialComponent;
  @tracked error;

  @tracked proceedPromiseResolve;

  get activeTaskToggleRow() {
    return this.taskToggleRows[this.activeTaskToggleRowIndex];
  }

  get activeTask() {
    return this.activeTaskToggleRow.task;
  }

  get narrative() {
    const summary = this.args.summary;
    const taskGroup = summary.taskGroup;

    const diffs = new ResourcesDiffs(
      taskGroup,
      taskGroup.count,
      this.args.summary.recommendations,
      this.args.summary.excludedRecommendations,
    );

    const cpuDelta = diffs.cpu.delta;
    const memoryDelta = diffs.memory.delta;

    const aggregate = taskGroup.count > 1;
    const aggregateString = aggregate ? ' an aggregate' : '';

    if (cpuDelta || memoryDelta) {
      const deltasSameDirection =
        (cpuDelta < 0 && memoryDelta < 0) || (cpuDelta > 0 && memoryDelta > 0);

      let narrative = 'Applying the selected recommendations will';

      if (deltasSameDirection) {
        narrative += ` ${verbForDelta(cpuDelta)} ${aggregateString}`;
      }

      if (cpuDelta) {
        if (!deltasSameDirection) {
          narrative += ` ${verbForDelta(cpuDelta)} ${aggregateString}`;
        }

        narrative += ` <strong>${diffs.cpu.absoluteAggregateDiff} of CPU</strong>`;
      }

      if (cpuDelta && memoryDelta) {
        narrative += ' and';
      }

      if (memoryDelta) {
        if (!deltasSameDirection) {
          narrative += ` ${verbForDelta(memoryDelta)} ${aggregateString}`;
        }

        narrative += ` <strong>${diffs.memory.absoluteAggregateDiff} of memory</strong>`;
      }

      if (taskGroup.count === 1) {
        narrative += '.';
      } else {
        narrative += ` across <strong>${taskGroup.count} allocations</strong>.`;
      }

      return htmlSafe(narrative);
    } else {
      return '';
    }
  }

  get taskToggleRows() {
    const taskNameToTaskToggles = {};

    return this.args.summary.recommendations.reduce(
      (taskToggleRows, recommendation) => {
        let taskToggleRow = taskNameToTaskToggles[recommendation.task.name];

        if (!taskToggleRow) {
          taskToggleRow = {
            recommendations: [],
            task: recommendation.task,
          };

          taskNameToTaskToggles[recommendation.task.name] = taskToggleRow;
          taskToggleRows.push(taskToggleRow);
        }

        const isCpu = recommendation.resource === 'CPU';
        const rowResourceProperty = isCpu ? 'cpu' : 'memory';

        taskToggleRow[rowResourceProperty] = {
          recommendation,
          isActive:
            !this.args.summary.excludedRecommendations.includes(recommendation),
        };

        if (isCpu) {
          taskToggleRow.recommendations.unshift(recommendation);
        } else {
          taskToggleRow.recommendations.push(recommendation);
        }

        return taskToggleRows;
      },
      [],
    );
  }

  get showToggleAllToggles() {
    return this.taskToggleRows.length > 1;
  }

  get allCpuToggleDisabled() {
    return !this.args.summary.recommendations.filterBy('resource', 'CPU')
      .length;
  }

  get allMemoryToggleDisabled() {
    return !this.args.summary.recommendations.filterBy('resource', 'MemoryMB')
      .length;
  }

  get cannotAccept() {
    return (
      this.args.summary.excludedRecommendations.length ==
      this.args.summary.recommendations.length
    );
  }

  get copyButtonLink() {
    const path = this.router.urlFor(
      'optimize.summary',
      this.args.summary.slug,
      {
        queryParams: { namespace: this.args.summary.jobNamespace },
      },
    );
    const { origin } = window.location;

    return `${origin}${path}`;
  }

  @(task(function* () {
    this.interstitialComponent = 'accepted';
    yield timeout(macroCondition(isTesting()) ? 0 : 2000);

    this.args.proceed.perform();
    this.resetInterstitial();
  }).drop())
  onApplied;

  @(task(function* () {
    const { manuallyDismissed } = yield new Promise((resolve) => {
      this.proceedPromiseResolve = resolve;
      this.interstitialComponent = 'dismissed';
    });

    if (!manuallyDismissed) {
      yield timeout(macroCondition(isTesting()) ? 0 : 2000);
    }

    this.args.proceed.perform();
    this.resetInterstitial();
  }).drop())
  onDismissed;

  @(task(function* (error) {
    yield new Promise((resolve) => {
      this.proceedPromiseResolve = resolve;
      this.interstitialComponent = 'error';
      this.error = error.toString();
    });

    this.args.proceed.perform();
    this.resetInterstitial();
  }).drop())
  onError;

  get interstitialStyle() {
    return htmlSafe(`height: ${this.cardHeight}px`);
  }

  toggleAllRecommendationsForResource = (resource) => {
    let enabled;

    if (resource === 'CPU') {
      this.allCpuToggleActive = !this.allCpuToggleActive;
      enabled = this.allCpuToggleActive;
    } else {
      this.allMemoryToggleActive = !this.allMemoryToggleActive;
      enabled = this.allMemoryToggleActive;
    }

    this.args.summary.toggleAllRecommendationsForResource(resource, enabled);
  };

  accept = () => {
    this.storeCardHeight();
    this.args.summary
      .save()
      .then(
        () => this.onApplied.perform(),
        (e) => this.onError.perform(e),
      )
      .catch((e) => {
        if (!didCancel(e)) {
          throw e;
        }
      });
  };

  dismiss = async () => {
    this.storeCardHeight();
    const recommendations = await this.args.summary.recommendations;

    this.args.summary.excludedRecommendations.pushObjects(recommendations);

    this.args.summary
      .save()
      .then(
        () => this.onDismissed.perform(),
        (e) => this.onError.perform(e),
      )
      .catch((e) => {
        if (!didCancel(e)) {
          throw e;
        }
      });
  };

  setActiveTaskToggleRowIndex = (index) => {
    this.activeTaskToggleRowIndex = index;
  };

  resetInterstitial() {
    if (!this.args.skipReset) {
      this.interstitialComponent = undefined;
      this.error = undefined;
    }
  }

  cardInserted = (element) => {
    this.element = element;
  };

  storeCardHeight() {
    this.cardHeight = this.element.clientHeight;
  }

  <template>
    {{! template-lint-disable no-duplicate-landmark-elements}}
    {{#if this.interstitialComponent}}
      <section class="das-interstitial" style={{this.interstitialStyle}}>
        {{#if (eq this.interstitialComponent "accepted")}}
          <DasAccepted />
        {{else if (eq this.interstitialComponent "dismissed")}}
          <DasDismissed @proceed={{this.proceedPromiseResolve}} />
        {{else if (eq this.interstitialComponent "error")}}
          <DasError
            @proceed={{this.proceedPromiseResolve}}
            @error={{this.error}}
          />
        {{/if}}
      </section>
    {{else if @summary.taskGroup}}
      <section
        ...attributes
        data-test-task-group-recommendations
        class="recommendation-card"
        {{didInsert this.cardInserted}}
      >

        <h2 class="top overview inner-container">Resource Recommendation</h2>

        <header class="overview inner-container">
          <h3 class="slug">
            <span
              class="job"
              data-test-job-name
            >{{@summary.taskGroup.job.name}}</span>
            <span
              class="group"
              data-test-task-group-name
            >{{@summary.taskGroup.name}}</span>
          </h3>
          <h4 class="namespace">
            <span class="namespace-label">Namespace:</span>
            <span data-test-namespace>{{@summary.jobNamespace}}</span>
          </h4>
        </header>

        <section class="diffs overview inner-container">
          <DasDiffsTable
            data-test-group-totals
            @model={{@summary.taskGroup}}
            @recommendations={{@summary.recommendations}}
            @excludedRecommendations={{@summary.excludedRecommendations}}
          />
        </section>

        <section class="narrative overview inner-container">
          <p data-test-narrative>{{this.narrative}}</p>
        </section>

        <section class="main overview inner-container task-toggles">
          <table data-test-toggles-table>
            <thead data-test-tasks-head>
              <tr>
                {{#if this.showToggleAllToggles}}
                  <th>Task</th>
                  <th class="toggle-all">Toggle All</th>
                  <th class="toggle-cell">
                    <label>
                      <Toggle
                        data-test-cpu-toggle
                        @isActive={{and
                          this.allCpuToggleActive
                          (not this.allCpuToggleDisabled)
                        }}
                        @isDisabled={{this.allCpuToggleDisabled}}
                        @onToggle={{fn
                          this.toggleAllRecommendationsForResource
                          "CPU"
                        }}
                        title="Toggle CPU recommendations for all tasks"
                      >
                        <div class="label-wrapper">CPU</div>
                      </Toggle>
                    </label>
                  </th>
                  <th class="toggle-cell">
                    <label>
                      <Toggle
                        data-test-memory-toggle
                        @isActive={{and
                          this.allMemoryToggleActive
                          (not this.allMemoryToggleDisabled)
                        }}
                        @isDisabled={{this.allMemoryToggleDisabled}}
                        @onToggle={{fn
                          this.toggleAllRecommendationsForResource
                          "MemoryMB"
                        }}
                        title="Toggle memory recommendations for all tasks"
                      >
                        <div class="label-wrapper">Mem</div>
                      </Toggle>
                    </label>
                  </th>
                {{else}}
                  <th colspan="2">Task</th>
                  <th class="toggle-cell">CPU</th>
                  <th class="toggle-cell">Mem</th>
                {{/if}}
              </tr>
            </thead>
            <tbody>
              {{#each
                this.taskToggleRows key="task.name"
                as |taskToggleRow index|
              }}
                <DasTaskRow
                  @task={{taskToggleRow.task}}
                  @active={{eq this.activeTaskToggleRowIndex index}}
                  @cpu={{taskToggleRow.cpu}}
                  @memory={{taskToggleRow.memory}}
                  @onClick={{fn this.setActiveTaskToggleRowIndex index}}
                  @toggleRecommendation={{@summary.toggleRecommendation}}
                />
              {{/each}}
            </tbody>
          </table>
        </section>

        <section class="actions overview inner-container">
          <button
            class="button is-primary"
            type="button"
            disabled={{this.cannotAccept}}
            data-test-accept
            {{on "click" this.accept}}
          >Accept</button>
          <button
            class="button is-light"
            type="button"
            data-test-dismiss
            {{on "click" this.dismiss}}
          >Dismiss</button>
        </section>

        <section class="active-task-group" data-test-active-task>
          <section class="top active-task inner-container">
            <CopyButton
              data-test-copy-button
              @clipboardText={{this.copyButtonLink}}
            >
              {{@summary.taskGroup.job.name}}
              /
              {{@summary.taskGroup.name}}
            </CopyButton>

            {{#if @onCollapse}}
              <button
                data-test-accordion-toggle
                class="button is-light is-compact pull-right accordion-toggle"
                {{on "click" @onCollapse}}
                type="button"
              >
                Collapse
              </button>
            {{/if}}
          </section>

          <header class="active-task inner-container">
            <h3 data-test-task-name>{{this.activeTask.name}} task</h3>
          </header>

          <section class="diffs active-task inner-container">
            <DasDiffsTable
              @model={{this.activeTask}}
              @recommendations={{this.activeTaskToggleRow.recommendations}}
              @excludedRecommendations={{@summary.excludedRecommendations}}
            />
          </section>

          <ul class="main active-task inner-container">
            {{#each
              this.activeTaskToggleRow.recommendations
              as |recommendation|
            }}
              <li data-test-recommendation>
                <DasRecommendationChart
                  data-test-chart-for={{recommendation.resource}}
                  @resource={{recommendation.resource}}
                  @currentValue={{recommendation.currentValue}}
                  @recommendedValue={{recommendation.value}}
                  @stats={{recommendation.stats}}
                  @disabled={{includes
                    recommendation
                    @summary.excludedRecommendations
                  }}
                />
              </li>
            {{/each}}
          </ul>
        </section>

      </section>
    {{/if}}
  </template>
}

function verbForDelta(delta) {
  if (delta > 0) {
    return 'add';
  } else {
    return 'save';
  }
}
