/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { hash } from '@ember/helper';
import { pageTitle } from 'ember-page-title';
import JobEditor from 'nomad-ui/components/job-editor';
import JobSubnav from 'nomad-ui/components/job-subnav';

<template>
  {{pageTitle "Job " @controller.job.name " definition"}}
  <JobSubnav @job={{@controller.job}} />
  <section class="section">
    <JobEditor
      @cancelable={{true}}
      @context={{@controller.context}}
      @definition={{@controller.definition}}
      @format={{@controller.format}}
      @job={{@controller.job}}
      @specification={{@controller.specification}}
      @variables={{hash
        flags=@controller.variableFlags
        literal=@controller.variableLiteral
      }}
      @view={{@controller.view}}
      @onSubmit={{@controller.onSubmit}}
      @onSelect={{@controller.selectView}}
      @onToggleEdit={{@controller.toggleEdit}}
    />
  </section>
</template>
