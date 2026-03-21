/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import and from 'ember-truth-helpers/helpers/and';
import not from 'ember-truth-helpers/helpers/not';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import JobEditor from 'nomad-ui/components/job-editor';
import { HdsAlert } from '@hashicorp/design-system-components/components';

<template>
  <Breadcrumb @crumb={{hash label="Run" args=(array "jobs.run")}} />
  {{pageTitle "Run a job"}}
  <section class="section">
    {{#if
      (and @controller.sourceString (not @controller.disregardNameWarning))
    }}
      <HdsAlert
        @type="inline"
        @color="warning"
        data-test-job-name-warning
        as |A|
      >
        <A.Title>Don't forget to change the job name!</A.Title>
        <A.Description>Since you're cloning a job version's source as a new job,
          you'll want to change the job name. Otherwise, this will appear as a
          new version of the original job, rather than a new job.</A.Description>
      </HdsAlert>
    {{/if}}
    <JobEditor
      @job={{@model}}
      @context="new"
      @onSubmit={{@controller.onSubmit}}
      @handleSaveAsTemplate={{@controller.handleSaveAsTemplate}}
      data-test-job-editor
    />
  </section>
</template>
