/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { concat, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { matchesState } from 'ember-statecharts';
import d3 from 'd3';
import onClickOutside from 'ember-click-outside/modifiers/on-click-outside';
import EvaluationSidebarRelatedEvaluations from 'nomad-ui/components/evaluation-sidebar/related-evaluations';
import JsonViewer from 'nomad-ui/components/json-viewer';
import LoadingSpinner from 'nomad-ui/components/loading-spinner';
import PlacementFailure from 'nomad-ui/components/placement-failure';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import keyboardCommands from 'nomad-ui/helpers/keyboard-commands';

export default class EvaluationSidebarDetail extends Component {
  get statechart() {
    return this.args.statechart;
  }

  @matchesState({ sidebar: 'open' })
  isSideBarOpen;

  @matchesState({ sidebar: { open: 'success' } })
  isSuccess;

  @matchesState({ sidebar: { open: 'busy' } })
  isLoading;

  @matchesState({ sidebar: { open: 'error' } })
  isError;

  @tracked width = null;
  @tracked height = null;

  handleResize = ({ target: { scrollWidth: width, scrollHeight: height } }) => {
    if (width === this.width || height === this.height) return;

    this.height = height;
    this.width = width;
  };

  get currentEvalDetail() {
    return this.args.statechart.state.context.evaluation;
  }

  get hierarchy() {
    try {
      const data = this.currentEvalDetail?.relatedEvals;

      if (data) {
        return d3
          .stratify()
          .id((detail) => detail.id)
          .parentId((detail) => detail.previousEval)([
          ...data.toArray(),
          this.currentEvalDetail,
        ]);
      }
    } catch (error) {
      console.error(`\n\nRelated Evaluation Error:   ${error.message}`);
    }

    return null;
  }

  get descendentsMap() {
    return this.hierarchy
      ?.descendants()
      .map((detail) => detail.children)
      .filter(val => val !== undefined && val !== null);
  }

  get parentEvaluation() {
    return this.hierarchy?.data;
  }

  get portalTargetElement() {
    if (typeof document === 'undefined') {
      return null;
    }

    return document.getElementById('eval-detail-portal');
  }

  closeSidebar = () => {
    return this.args.statechart.send('MODAL_CLOSE');
  };

  get keyCommands() {
    return [
      {
        label: 'Close Evaluations Sidebar',
        pattern: ['Escape'],
        action: () => this.closeSidebar(),
      },
    ];
  }

  <template>
    {{#let this.currentEvalDetail as |evaluation|}}
      {{#if this.isSideBarOpen}}
        {{keyboardCommands this.keyCommands}}
      {{/if}}

      {{#if this.portalTargetElement}}
        {{#in-element this.portalTargetElement}}
          <div
            data-test-eval-detail
            data-test-eval-detail-is-open={{this.isSideBarOpen}}
            class="sidebar {{if this.isSideBarOpen 'open'}} evaluations-sidebar"
            {{onClickOutside
              this.closeSidebar
              capture=true
              exceptSelector="tr[data-eval-row]"
            }}
          >
            {{#if this.isLoading}}
              <div data-test-eval-loading>
                <section class="section has-text-centered">
                  <LoadingSpinner />
                </section>
              </div>
            {{/if}}

            {{#if this.isError}}
              <div data-test-eval-detail-header class="error-header">
                <button
                  data-test-eval-sidebar-x
                  class="button is-borderless"
                  type="button"
                  {{on "click" this.closeSidebar}}
                >
                  <HdsIcon @name="x" />
                </button>
              </div>

              <div class="error-container">
                <div data-test-eval-error class="error-message">
                  <h1 data-test-error-title class="title is-spaced">
                    Not Found
                  </h1>
                  <p data-test-error-message class="subtitle">
                    The requested evaluation could not be found. You may not be
                    authorized to view this evaluation, it may have been garbage
                    collected, or the ID is invalid.
                  </p>
                </div>
              </div>
            {{/if}}

            {{#if this.isSuccess}}
              <div data-test-eval-detail-header class="detail-header">
                <h1 data-test-title class="title">
                  {{evaluation.shortId}}
                  <span class="bumper-left tag is-primary">
                    {{evaluation.status}}
                  </span>
                </h1>

                <button
                  data-test-eval-sidebar-x
                  class="button is-borderless"
                  type="button"
                  {{on "click" this.closeSidebar}}
                >
                  <HdsIcon @name="x" />
                </button>
              </div>

              <div class="boxed-section is-small">
                <div
                  class="boxed-section-body inline-definitions"
                  style="display: flex;"
                >
                  <span class="label" style="width: 6.125rem;">
                    Evaluation Details
                  </span>

                  <div style="display: flex; flex-direction: column">
                    <span class="pair">
                      <span class="term">
                        Job
                      </span>
                      <LinkTo
                        data-test-evaluation-job
                        @model={{concat
                          evaluation.plainJobId
                          "@"
                          evaluation.namespace
                        }}
                        @route="jobs.job"
                      >
                        {{evaluation.plainJobId}}
                      </LinkTo>
                    </span>

                    <span class="pair">
                      <span class="term">
                        Triggered By
                      </span>
                      {{evaluation.triggeredBy}}
                    </span>
                  </div>

                  <div style="display: flex; flex-direction: column">
                    <span class="pair">
                      <span class="term">
                        Priority
                      </span>
                      {{evaluation.priority}}
                    </span>
                  </div>

                  <div style="display: flex; flex-direction: column">
                    <span class="pair">
                      <span class="term">
                        Created
                      </span>
                      {{formatMonthTs evaluation.createTime}}
                    </span>
                    <span class="pair">
                      <span class="term">
                        Placement Failure
                      </span>
                      {{evaluation.hasPlacementFailures}}
                    </span>
                  </div>
                </div>
              </div>

              {{#if evaluation.failedTGAllocs}}
                <div class="boxed-section is-danger">
                  <div class="boxed-section-head">
                    Placement Failures
                  </div>
                  <div class="boxed-section-body">
                    {{#each evaluation.failedTGAllocs as |placementFailure|}}
                      <PlacementFailure @failedTGAlloc={{placementFailure}} />
                    {{/each}}
                  </div>
                </div>
              {{/if}}

              {{#if this.descendentsMap}}
                <EvaluationSidebarRelatedEvaluations
                  @fns={{hash
                    handleResize=this.handleResize
                    handleEvaluationClick=@fns.handleEvaluationClick
                  }}
                  @data={{hash
                    width=this.width
                    height=this.height
                    parentEvaluation=this.parentEvaluation
                    descendentsMap=this.descendentsMap
                    activeEvaluationID=this.currentEvalDetail.id
                  }}
                />
              {{else}}
                <div class="boxed-section">
                  <div class="boxed-section-head">
                    Related Evaluations
                  </div>
                  <div class="boxed-section-body">
                    <div data-test-eval-error class="error-message title">
                      <p data-test-error-message class="subtitle">
                        The related evaluations cannot be visualized.
                      </p>
                    </div>
                  </div>
                </div>
              {{/if}}

              <div class="boxed-section evaluation-response">
                <div class="boxed-section-head">
                  Evaluation Response
                </div>
                <div class="boxed-section-body is-full-bleed">
                  <JsonViewer @json={{evaluation}} />
                </div>
              </div>
            {{/if}}
          </div>
        {{/in-element}}
      {{else}}
        <div
          data-test-eval-detail
          data-test-eval-detail-is-open={{this.isSideBarOpen}}
          class="sidebar {{if this.isSideBarOpen 'open'}} evaluations-sidebar"
          {{onClickOutside
            this.closeSidebar
            capture=true
            exceptSelector="tr[data-eval-row]"
          }}
        >
          {{#if this.isLoading}}
            <div data-test-eval-loading>
              <section class="section has-text-centered">
                <LoadingSpinner />
              </section>
            </div>
          {{/if}}

          {{#if this.isError}}
            <div data-test-eval-detail-header class="error-header">
              <button
                data-test-eval-sidebar-x
                class="button is-borderless"
                type="button"
                {{on "click" this.closeSidebar}}
              >
                <HdsIcon @name="x" />
              </button>
            </div>

            <div class="error-container">
              <div data-test-eval-error class="error-message">
                <h1 data-test-error-title class="title is-spaced">
                  Not Found
                </h1>
                <p data-test-error-message class="subtitle">
                  The requested evaluation could not be found. You may not be
                  authorized to view this evaluation, it may have been garbage
                  collected, or the ID is invalid.
                </p>
              </div>
            </div>
          {{/if}}

          {{#if this.isSuccess}}
            <div data-test-eval-detail-header class="detail-header">
              <h1 data-test-title class="title">
                {{evaluation.shortId}}
                <span class="bumper-left tag is-primary">
                  {{evaluation.status}}
                </span>
              </h1>

              <button
                data-test-eval-sidebar-x
                class="button is-borderless"
                type="button"
                {{on "click" this.closeSidebar}}
              >
                <HdsIcon @name="x" />
              </button>
            </div>

            <div class="boxed-section is-small">
              <div
                class="boxed-section-body inline-definitions"
                style="display: flex;"
              >
                <span class="label" style="width: 6.125rem;">
                  Evaluation Details
                </span>

                <div style="display: flex; flex-direction: column">
                  <span class="pair">
                    <span class="term">
                      Job
                    </span>
                    <LinkTo
                      data-test-evaluation-job
                      @model={{concat
                        evaluation.plainJobId
                        "@"
                        evaluation.namespace
                      }}
                      @route="jobs.job"
                    >
                      {{evaluation.plainJobId}}
                    </LinkTo>
                  </span>

                  <span class="pair">
                    <span class="term">
                      Triggered By
                    </span>
                    {{evaluation.triggeredBy}}
                  </span>
                </div>

                <div style="display: flex; flex-direction: column">
                  <span class="pair">
                    <span class="term">
                      Priority
                    </span>
                    {{evaluation.priority}}
                  </span>
                </div>

                <div style="display: flex; flex-direction: column">
                  <span class="pair">
                    <span class="term">
                      Created
                    </span>
                    {{formatMonthTs evaluation.createTime}}
                  </span>
                  <span class="pair">
                    <span class="term">
                      Placement Failure
                    </span>
                    {{evaluation.hasPlacementFailures}}
                  </span>
                </div>
              </div>
            </div>

            {{#if evaluation.failedTGAllocs}}
              <div class="boxed-section is-danger">
                <div class="boxed-section-head">
                  Placement Failures
                </div>
                <div class="boxed-section-body">
                  {{#each evaluation.failedTGAllocs as |placementFailure|}}
                    <PlacementFailure @failedTGAlloc={{placementFailure}} />
                  {{/each}}
                </div>
              </div>
            {{/if}}

            {{#if this.descendentsMap}}
              <EvaluationSidebarRelatedEvaluations
                @fns={{hash
                  handleResize=this.handleResize
                  handleEvaluationClick=@fns.handleEvaluationClick
                }}
                @data={{hash
                  width=this.width
                  height=this.height
                  parentEvaluation=this.parentEvaluation
                  descendentsMap=this.descendentsMap
                  activeEvaluationID=this.currentEvalDetail.id
                }}
              />
            {{else}}
              <div class="boxed-section">
                <div class="boxed-section-head">
                  Related Evaluations
                </div>
                <div class="boxed-section-body">
                  <div data-test-eval-error class="error-message title">
                    <p data-test-error-message class="subtitle">
                      The related evaluations cannot be visualized.
                    </p>
                  </div>
                </div>
              </div>
            {{/if}}

            <div class="boxed-section evaluation-response">
              <div class="boxed-section-head">
                Evaluation Response
              </div>
              <div class="boxed-section-body is-full-bleed">
                <JsonViewer @json={{evaluation}} />
              </div>
            </div>
          {{/if}}
        </div>
      {{/if}}

    {{/let}}
  </template>
}
