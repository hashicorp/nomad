/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';
import { htmlSafe } from '@ember/template';
import { didCancel, task, timeout } from 'ember-concurrency';
import Ember from 'ember';

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
      this.args.summary.excludedRecommendations
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
      []
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
      }
    );
    const { origin } = window.location;

    return `${origin}${path}`;
  }

  @action
  toggleAllRecommendationsForResource(resource) {
    let enabled;

    if (resource === 'CPU') {
      this.allCpuToggleActive = !this.allCpuToggleActive;
      enabled = this.allCpuToggleActive;
    } else {
      this.allMemoryToggleActive = !this.allMemoryToggleActive;
      enabled = this.allMemoryToggleActive;
    }

    this.args.summary.toggleAllRecommendationsForResource(resource, enabled);
  }

  @action
  accept() {
    this.storeCardHeight();
    this.args.summary
      .save()
      .then(
        () => this.onApplied.perform(),
        (e) => this.onError.perform(e)
      )
      .catch((e) => {
        if (!didCancel(e)) {
          throw e;
        }
      });
  }

  @action
  async dismiss() {
    this.storeCardHeight();
    const recommendations = await this.args.summary.recommendations;

    this.args.summary.excludedRecommendations.pushObjects(recommendations);

    this.args.summary
      .save()
      .then(
        () => this.onDismissed.perform(),
        (e) => this.onError.perform(e)
      )
      .catch((e) => {
        if (!didCancel(e)) {
          throw e;
        }
      });
  }

  @(task(function* () {
    this.interstitialComponent = 'accepted';
    yield timeout(Ember.testing ? 0 : 2000);

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
      yield timeout(Ember.testing ? 0 : 2000);
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

  resetInterstitial() {
    if (!this.args.skipReset) {
      this.interstitialComponent = undefined;
      this.error = undefined;
    }
  }

  @action
  cardInserted(element) {
    this.element = element;
  }

  storeCardHeight() {
    this.cardHeight = this.element.clientHeight;
  }
}

function verbForDelta(delta) {
  if (delta > 0) {
    return 'add';
  } else {
    return 'save';
  }
}
