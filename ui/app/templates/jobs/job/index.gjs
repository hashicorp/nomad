/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import eq from 'ember-truth-helpers/helpers/eq';
import { pageTitle } from 'ember-page-title';
import JobPageBatch from 'nomad-ui/components/job-page/batch';
import JobPageParameterized from 'nomad-ui/components/job-page/parameterized';
import JobPageParameterizedChild from 'nomad-ui/components/job-page/parameterized-child';
import JobPagePeriodic from 'nomad-ui/components/job-page/periodic';
import JobPagePeriodicChild from 'nomad-ui/components/job-page/periodic-child';
import JobPageService from 'nomad-ui/components/job-page/service';
import JobPageSystem from 'nomad-ui/components/job-page/system';
import JobPageSysbatch from 'nomad-ui/components/job-page/sysbatch';

<template>
  {{pageTitle "Job " @model.name}}
  {{#if (eq @model.templateType "batch")}}
    <JobPageBatch
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{else if (eq @model.templateType "system")}}
    <JobPageSystem
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{else if (eq @model.templateType "sysbatch")}}
    <JobPageSysbatch
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{else if (eq @model.templateType "periodic")}}
    <JobPagePeriodic
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{else if (eq @model.templateType "periodic-child")}}
    <JobPagePeriodicChild
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{else if (eq @model.templateType "parameterized")}}
    <JobPageParameterized
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{else if (eq @model.templateType "parameterized-child")}}
    <JobPageParameterizedChild
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{else}}
    <JobPageService
      @job={{@model}}
      @sortProperty={{@controller.sortProperty}}
      @sortDescending={{@controller.sortDescending}}
      @currentPage={{@controller.currentPage}}
      @activeTask={{@controller.activeTask}}
      @setActiveTaskQueryParam={{@controller.setActiveTaskQueryParam}}
      @statusMode={{@controller.statusMode}}
      @setStatusMode={{@controller.setStatusMode}}
      @childJobs={{@controller.childJobs}}
    />
  {{/if}}
</template>
