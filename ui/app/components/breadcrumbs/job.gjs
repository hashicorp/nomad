/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LinkTo } from '@ember/routing';

import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import KeyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import Trigger from 'nomad-ui/components/trigger';
import BreadcrumbsTemplate from 'nomad-ui/components/breadcrumbs/default';

export default class BreadcrumbsJob extends BreadcrumbsTemplate {
  shortcutPattern = ['u'];

  get job() {
    return this.args.crumb.job;
  }

  get hasParent() {
    const job = this.job;

    if (!job || typeof job.belongsTo !== 'function') {
      return false;
    }

    return !!job.belongsTo('parent').id();
  }

  traverseUpALevel = () => {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  };

  onError = (err) => {
    // Parent breadcrumb lookup can fail for ephemeral/missing parent records.
    // Keep the current-job breadcrumb visible instead of crashing the app.
    return err;
  };

  fetchParent = () => {
    if (this.hasParent) {
      return this.job.get('parent');
    }
  };

  <template>
    <Trigger @onError={{this.onError}} @do={{this.fetchParent}} as |trigger|>
      <span hidden {{didInsert trigger.fns.do}}></span>

      {{#if trigger.data.isBusy}}
        <li>
          <a href="#" aria-label="loading" data-test-breadcrumb="loading">
            ...
          </a>
        </li>
      {{/if}}

      {{#if trigger.data.isSuccess}}
        {{#if trigger.data.result}}
          {{#if this.hasParent}}
            <li>
              <LinkTo
                @route="jobs.job.index"
                @model={{trigger.data.result}}
                data-test-breadcrumb="jobs.job.index"
              >
                <dl>
                  <dt>
                    Parent Job
                  </dt>
                  <dd>
                    {{trigger.data.result.trimmedName}}
                  </dd>
                </dl>
              </LinkTo>
            </li>
          {{/if}}
        {{/if}}

        {{#if this.isOneCrumbUp}}
          <li
            {{KeyboardShortcutModifier
              label="Go up a level"
              pattern=this.shortcutPattern
              menuLevel=true
              action=this.traverseUpALevel
              exclusive=true
            }}
          >
            <LinkTo
              @route="jobs.job.index"
              @model={{this.job}}
              data-test-breadcrumb="jobs.job.index"
              data-test-job-breadcrumb
            >
              <dl>
                <dt>
                  {{if this.job.hasChildren "Parent Job" "Job"}}
                </dt>
                <dd>
                  {{this.job.trimmedName}}
                </dd>
              </dl>
            </LinkTo>
          </li>
        {{else}}
          <li>
            <LinkTo
              @route="jobs.job.index"
              @model={{this.job}}
              data-test-breadcrumb="jobs.job.index"
              data-test-job-breadcrumb
            >
              <dl>
                <dt>
                  {{if this.job.hasChildren "Parent Job" "Job"}}
                </dt>
                <dd>
                  {{this.job.trimmedName}}
                </dd>
              </dl>
            </LinkTo>
          </li>
        {{/if}}
      {{/if}}
    </Trigger>
  </template>
}
