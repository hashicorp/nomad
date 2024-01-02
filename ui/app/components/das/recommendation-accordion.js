/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { task, timeout } from 'ember-concurrency';
import { htmlSafe } from '@ember/template';
import Ember from 'ember';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';

export default class DasRecommendationAccordionComponent extends Component {
  @tracked waitingToProceed = false;
  @tracked closing = false;
  @tracked animationContainerStyle = htmlSafe('');

  @(task(function* () {
    this.closing = true;
    this.animationContainerStyle = htmlSafe(
      `height: ${this.accordionElement.clientHeight}px`
    );

    yield timeout(10);

    this.animationContainerStyle = htmlSafe('height: 0px');

    // The 450ms for the animation to complete, set in CSS as $timing-slow
    yield timeout(Ember.testing ? 0 : 450);

    this.waitingToProceed = false;
  }).drop())
  proceed;

  @action
  inserted(element) {
    this.accordionElement = element;
    this.waitingToProceed = true;
  }

  get show() {
    return !this.args.summary.isProcessed || this.waitingToProceed;
  }

  get diffs() {
    const summary = this.args.summary;
    const taskGroup = summary.taskGroup;

    return new ResourcesDiffs(
      taskGroup,
      taskGroup.count,
      this.args.summary.recommendations,
      this.args.summary.excludedRecommendations
    );
  }
}
