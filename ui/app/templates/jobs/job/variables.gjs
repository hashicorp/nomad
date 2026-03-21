/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import gt from 'ember-truth-helpers/helpers/gt';
import EditableVariableLink from 'nomad-ui/components/editable-variable-link';
import JobSubnav from 'nomad-ui/components/job-subnav';
import VariablePaths from 'nomad-ui/components/variable-paths';
import {
  HdsAlert,
  HdsButton,
  HdsLinkInline,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Job " @model.job.name " variables"}}
  <JobSubnav @job={{@model.job}} />

  <section class="section">
    <header class="job-variables-intro">
      <HdsAlert @type="inline" @color="highlight" as |A|>
        <A.Title>Automatic Access to Variables</A.Title>
        <A.Description>
          <p>Tasks in this job can have
            <HdsLinkInline
              @href="https://developer.hashicorp.com/nomad/docs/concepts/variables#task-access-to-variables"
              target="_blank"
              rel="noopener noreferrer"
            >automatic access to Nomad Variables</HdsLinkInline>.</p>
          <ul>
            <li data-test-variables-intro-all-jobs>Use
              <code>
                <EditableVariableLink
                  @path="nomad/jobs"
                  @existingPaths={{@controller.jobRelevantVariables.files}}
                  @namespace={{@model.job.namespace.name}}
                />
              </code>
              for access in all tasks in all jobs</li>
            <li data-test-variables-intro-job>
              Use
              <code>
                <EditableVariableLink
                  @path={{concat "nomad/jobs/" @model.job.name}}
                  @existingPaths={{@controller.jobRelevantVariables.files}}
                  @namespace={{@model.job.namespace.name}}
                />
              </code>
              for access from all tasks in this job
            </li>
            <li data-test-variables-intro-groups>
              Use
              {{#if (gt @controller.firstFewTaskGroupNames.length 1)}}
                {{#each @controller.firstFewTaskGroupNames as |name|}}
                  <code><EditableVariableLink
                      @path={{concat "nomad/jobs/" @model.job.name "/" name}}
                      @existingPaths={{@controller.jobRelevantVariables.files}}
                      @namespace={{@model.job.namespace.name}}
                    /></code>,
                {{/each}}
                etc. for access from all tasks in a specific task group
              {{else}}
                <code>
                  <EditableVariableLink
                    @path={{concat
                      "nomad/jobs/"
                      @model.job.name
                      "/"
                      @controller.firstTaskGroupName
                    }}
                    @existingPaths={{@controller.jobRelevantVariables.files}}
                    @namespace={{@model.job.namespace.name}}
                  />
                </code>
                for access from all tasks in a specific task group
              {{/if}}
            </li>
            <li data-test-variables-intro-tasks>
              Use
              {{#if (gt @controller.firstFewTaskNames.length 1)}}
                {{#each @controller.firstFewTaskNames as |name|}}
                  <code><EditableVariableLink
                      @path={{concat "nomad/jobs/" @model.job.name "/" name}}
                      @existingPaths={{@controller.jobRelevantVariables.files}}
                      @namespace={{@model.job.namespace.name}}
                    /></code>,
                {{/each}}
                etc. for access from a specific task
              {{else}}
                <code>
                  <EditableVariableLink
                    @path={{concat
                      "nomad/jobs/"
                      @model.job.name
                      "/"
                      @controller.firstTaskName
                    }}
                    @existingPaths={{@controller.jobRelevantVariables.files}}
                    @namespace={{@model.job.namespace.name}}
                  />
                </code>
                for access from a specific task
              {{/if}}
            </li>
          </ul>
        </A.Description>
        <A.LinkStandalone
          @color="secondary"
          @icon="arrow-right"
          @iconPosition="trailing"
          @text="Learn more about Nomad Variables"
          @href="https://developer.hashicorp.com/nomad/docs/job-declare/nomad-variables"
        />
      </HdsAlert>
    </header>

    {{#if @controller.jobRelevantVariables.files.length}}
      <VariablePaths @branch={{@controller.jobRelevantVariables}} />
    {{else}}
      <section class="job-variables-message">
        <p data-test-no-auto-vars-message>
          Job
          <strong>{{@model.job.name}}</strong>
          does not have automatic access to any variables, but may have access
          by virtue of policies associated with this job's tasks' workload
          identities. See
          <a
            href="https://developer.hashicorp.com/nomad/docs/concepts/workload-identity#workload-associated-acl-policies"
            target="_blank"
            rel="noopener noreferrer"
          >Workload-Associated ACL Policies</a>
          for more information.
        </p>
        {{#if (can "write variable")}}
          <HdsButton
            data-test-create-variable-button
            @text="Create a Variable"
            @size="large"
            @route="variables.new"
            @query={{hash path=(concat "nomad/jobs/" @model.job.name)}}
          />
        {{/if}}
      </section>
    {{/if}}
  </section>
</template>
