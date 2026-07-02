/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { hash } from '@ember/helper';
import { and, eq } from 'ember-truth-helpers';
import { HdsAlert } from '@hashicorp/design-system-components/components';
import conditionallyCapitalize from 'nomad-ui/helpers/conditionally-capitalize';

export default class Alert extends Component {
  @tracked shouldShowAlert = true;

  dismissAlert = () => {
    this.shouldShowAlert = false;
  };

  <template>
    <div class="job-editor-alerts">
      {{#if @data.error}}
        <HdsAlert
          @type="inline"
          @color="critical"
          data-test-error={{@data.error.type}}
          as |A|
        >
          <A.Title data-test-error-title>{{conditionallyCapitalize
              @data.error.type
              true
            }}</A.Title>
          <A.Description
            data-test-error-message
          >{{@data.error.message}}</A.Description>
          {{#if (eq @data.error.message "Job ID does not match")}}
            <A.Button
              @text="Run as a new job instead"
              @color="primary"
              @route="jobs.run"
              @query={{hash
                sourceString=@data.job._newDefinition
                disregardNameWarning=true
              }}
            />
          {{/if}}
        </HdsAlert>
      {{/if}}
      {{#if
        (and
          (eq @data.stage "read") @data.hasVariables (eq @data.view "job-spec")
        )
      }}
        {{#if this.shouldShowAlert}}
          <HdsAlert
            @type="inline"
            @onDismiss={{this.dismissAlert}}
            data-test-variable-notification
            as |A|
          >
            <A.Title>HCL Variables values may be incomplete</A.Title>
            <A.Description>Nomad cannot ensure that all variable values provided
              below match those provided on job submit. Ensure the proper values
              are provided before re-submitting the job.</A.Description>
          </HdsAlert>
        {{/if}}
      {{/if}}
      {{#if (and (eq @data.stage "edit") (eq @data.view "full-definition"))}}
        <HdsAlert @type="inline" @color="warning" data-test-json-warning as |A|>
          <A.Title>Edit JSON</A.Title>
          <A.Description>If you edit the JSON formation in the full definition,
            you will no longer be able to see job spec in HCL.</A.Description>
        </HdsAlert>
      {{/if}}
      {{#if (and (eq @data.stage "review") @data.shouldShowPlanMessage)}}
        <HdsAlert
          @type="inline"
          @onDismiss={{@fns.onDismissPlanMessage}}
          as |A|
        >
          <A.Title data-test-plan-help-title>Job Plan</A.Title>
          <A.Description data-test-plan-help-message>This is the impact running
            this job will have on your cluster</A.Description>
        </HdsAlert>
      {{/if}}
    </div>
  </template>
}
