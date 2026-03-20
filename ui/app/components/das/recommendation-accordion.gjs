/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { task, timeout } from 'ember-concurrency';
import { htmlSafe } from '@ember/template';
import { macroCondition, isTesting } from '@embroider/macros';
import { array } from '@ember/helper';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import ListAccordion from 'nomad-ui/components/list-accordion';
import DasRecommendationCard from 'nomad-ui/components/das/recommendation-card';
import ResourcesDiffs from 'nomad-ui/utils/resources-diffs';

export default class DasRecommendationAccordion extends Component {
  @tracked waitingToProceed = false;
  @tracked closing = false;
  @tracked animationContainerStyle = htmlSafe('');

  @(task(function* () {
    this.closing = true;
    this.animationContainerStyle = htmlSafe(
      `height: ${this.accordionElement.clientHeight}px`,
    );

    yield timeout(10);

    this.animationContainerStyle = htmlSafe('height: 0px');

    // The 450ms for the animation to complete, set in CSS as $timing-slow
    yield timeout(macroCondition(isTesting()) ? 0 : 450);

    this.waitingToProceed = false;
  }).drop())
  proceed;

  inserted = (element) => {
    this.accordionElement = element;
    this.waitingToProceed = true;
  };

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
      this.args.summary.excludedRecommendations,
    );
  }

  <template>
    {{#if this.show}}
      <div
        data-test-recommendation-accordion
        class="recommendation-accordion boxed-section
          {{if this.closing 'closing'}}"
      >
        <ListAccordion
          @source={{array @summary}}
          @key="id"
          {{didInsert this.inserted}}
          as |a|
        >
          <a.head @buttonLabel={{if a.isOpen "Collapse" "Show"}}>
            <section class="left">
              <HdsIcon @name="info-fill" />
              <span>Resource Recommendation</span>
              <span
                data-test-group
                class="group"
              >{{@summary.taskGroup.name}}</span>
            </section>

            <section class="diffs">
              {{#if this.diffs.cpu.delta}}
                <section>
                  <span class="resource">CPU</span>
                  {{this.diffs.cpu.signedDiff}}
                  <span class="percent">{{this.diffs.cpu.percentDiff}}</span>
                </section>
              {{/if}}

              {{#if this.diffs.memory.delta}}
                <section>
                  <span class="resource">Mem</span>
                  {{this.diffs.memory.signedDiff}}
                  <span class="percent">{{this.diffs.memory.percentDiff}}</span>
                </section>
              {{/if}}
            </section>
          </a.head>

          <a.body @fullBleed={{true}}>
            <div
              class="animation-container"
              style={{this.animationContainerStyle}}
            >
              <DasRecommendationCard
                @summary={{@summary}}
                @proceed={{this.proceed}}
                @onCollapse={{a.close}}
                @skipReset={{true}}
              />
            </div>
          </a.body>
        </ListAccordion>
      </div>
    {{/if}}
  </template>
}
