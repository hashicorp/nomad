/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import { eq } from 'ember-truth-helpers';
import {
  HdsDropdown,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';
import didUpdateHelper from 'ember-render-helpers/helpers/did-update-helper';
import JobSubnav from 'nomad-ui/components/job-subnav';
import JobVersionsStream from 'nomad-ui/components/job-versions-stream';

<template>
  {{pageTitle "Job " @model.name " versions"}}
  {{didUpdateHelper @controller.versionsDidUpdate @controller.job.versions}}
  <JobSubnav @job={{@model}} />
  <section class="section">

    <HdsPageHeader class="versions-page-header" as |PH|>
      <PH.Actions>
        <HdsDropdown data-test-diff-facet as |dd|>
          <dd.ToggleButton
            @text={{if
              @controller.diffVersion
              (concat "Diff against version " @controller.diffVersion)
              "Diff against previous version"
            }}
            @color="secondary"
          />
          <dd.Radio
            name="diff"
            checked={{eq @controller.diffVersion ""}}
            {{on "change" (fn @controller.setDiffVersion "")}}
          >
            previous version
          </dd.Radio>
          {{#each @controller.optionsDiff key="label" as |option|}}
            <dd.Radio
              name="diff"
              {{on "change" (fn @controller.setDiffVersion option.value)}}
              @value={{option.label}}
              checked={{eq @controller.diffVersion option.value}}
              data-test-dropdown-option={{option.label}}
            >
              {{option.label}}
            </dd.Radio>
          {{else}}
            <dd.Generic data-test-dropdown-empty>
              No versions
            </dd.Generic>
          {{/each}}
        </HdsDropdown>
      </PH.Actions>
    </HdsPageHeader>

    {{#if @controller.error}}
      <div
        data-test-inline-error
        class="notification {{@controller.errorLevelClass}}"
      >
        <div class="columns">
          <div class="column">
            <h3
              data-test-inline-error-title
              class="title is-4"
            >{{@controller.error.title}}</h3>
            <p data-test-inline-error-body>{{@controller.error.description}}</p>
          </div>
          <div class="column is-centered is-minimum">
            <button
              data-test-inline-error-close
              class="button {{@controller.errorLevelClass}}"
              {{on "click" @controller.onDismiss}}
              type="button"
            >Okay</button>
          </div>
        </div>
      </div>
    {{/if}}

    <JobVersionsStream
      @versions={{@model.versions}}
      @diffs={{@controller.diffs}}
      @verbose={{true}}
      @handleError={{@controller.handleError}}
      @diffsExpanded={{@controller.diffsExpanded}}
    />
  </section>
</template>
